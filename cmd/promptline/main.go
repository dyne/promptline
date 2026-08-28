// Copyright (C) 2025 Dyne.org foundation
// designed, written and maintained by Denis Roio <jaromil@dyne.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"promptline/internal/application"
	"promptline/internal/appserver"
	"promptline/internal/governance"
	"promptline/internal/instance"
	pruntime "promptline/internal/runtime"
	"promptline/plugins/promptline/skills"
)

// Version is set at build time via ldflags. Defaults to "dev".
var Version = "dev"

const uRootModulePath = "github.com/u-root/u-root"

// runApplication is a command-level seam. Its production value is the sole
// composition root for interactive and standalone MCP invocations.
var runApplication = application.Run

func main() {
	os.Exit(exitCode(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func exitCode(args []string, input io.Reader, output, stderr io.Writer) int {
	if err := run(args, input, output, stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "promptline:", err)
		return 1
	}
	return 0
}

func run(args []string, input io.Reader, output, stderr io.Writer) error {
	restoreUmask := instance.SetPrivateUmaskBeforeConcurrency()
	defer restoreUmask()
	cmd, err := pruntime.Parse(args, stderr)
	if err != nil {
		return err
	}
	if cmd.Version {
		return printVersionReport(output, cmd.Instance.CodexExecutable)
	}
	if cmd.VerifyAudit != "" {
		root, err := os.OpenRoot(cmd.VerifyAudit)
		if err != nil {
			return fmt.Errorf("open audit state root: %w", err)
		}
		defer root.Close()
		hash, err := governance.VerifyJournalRoot(root, "audit/events.jsonl", cmd.AuditAnchor)
		if err != nil {
			return err
		}
		if cmd.AuditAnchor == "" {
			_, err = fmt.Fprintf(output, "audit verified: local chain only (final hash %s); this does not defeat a same-user rewrite\n", hash)
		} else {
			_, err = fmt.Fprintf(output, "audit verified: external anchor matches (final hash %s)\n", hash)
		}
		return err
	}
	if cmd.ListSkills || cmd.SkillFiles != "" || cmd.Materialize != "" {
		catalog, err := skills.EmbeddedCatalog()
		if err != nil {
			return err
		}
		if cmd.ListSkills {
			_, err := fmt.Fprintln(output, strings.Join(catalog.ListSkills(), "\n"))
			return err
		}
		if cmd.SkillFiles != "" {
			files, err := catalog.ListFiles(cmd.SkillFiles)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(output, strings.Join(files, "\n"))
			return err
		}
		return catalog.Materialize(cmd.Materialize)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return runApplication(ctx, cmd, input, output, Version)
}

func printVersionReport(output io.Writer, codexExecutable string) error {
	codexVersion := "unavailable"
	if detected, err := appserver.Probe(context.Background(), codexExecutable); err == nil {
		codexVersion = detected
	} else {
		codexVersion = fmt.Sprintf("unavailable (%v)", err)
	}
	_, err := fmt.Fprintf(
		output,
		"promptline: %s\ncodex-cli: %s\nu-root: %s\ngo: %s\n",
		Version,
		codexVersion,
		dependencyVersion(uRootModulePath),
		runtime.Version(),
	)
	return err
}

func dependencyVersion(modulePath string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dependency := range info.Deps {
		if dependency.Path != modulePath {
			continue
		}
		if dependency.Replace != nil && dependency.Replace.Version != "" {
			return dependency.Replace.Version
		}
		if dependency.Version != "" {
			return dependency.Version
		}
		return "unknown"
	}
	return "unknown"
}
