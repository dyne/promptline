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
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"

	"promptline/internal/appserver"
	"promptline/internal/governance"
	"promptline/internal/instance"
	"promptline/internal/mcp"
	pruntime "promptline/internal/runtime"
	"promptline/internal/tools"
)

// Version is set at build time via ldflags. Defaults to "dev".
var Version = "dev"

const uRootModulePath = "github.com/u-root/u-root"

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
	cmd, err := pruntime.Parse(args, stderr)
	if err != nil {
		return err
	}
	if cmd.Version {
		return printVersionReport(output, cmd.Instance.CodexExecutable)
	}
	if cmd.ToolboxServe {
		toolConfig := tools.DefaultConfig()
		toolConfig.WorkingDirectory = cmd.Instance.WorkingDirectory
		toolConfig.Roots = []string{cmd.Instance.WorkingRoot}
		toolConfig.Policy = mcp.ReadOnlyToolPolicy()
		registry, err := tools.NewRegistryWithConfig(toolConfig)
		if err != nil {
			return err
		}
		defer registry.Close()
		server, err := mcp.NewServer(registry, input, output, 4<<20)
		if err != nil {
			return err
		}
		return server.Serve(context.Background())
	}
	in, err := instance.New(cmd.Instance)
	if err != nil {
		return err
	}
	lock, err := in.AcquireLock()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if in.ToolboxEnabled() {
		executable, err := os.Executable()
		if err != nil {
			_ = lock.Close()
			return fmt.Errorf("resolve Promptline executable: %w", err)
		}
		executable, err = filepath.EvalSymlinks(executable)
		if err != nil {
			_ = lock.Close()
			return fmt.Errorf("resolve Promptline executable symlinks: %w", err)
		}
		if err := mcp.InstallCodexConfig(executable, in); err != nil {
			_ = lock.Close()
			return err
		}
	}
	process, err := appserver.Start(ctx, in)
	if err != nil {
		_ = lock.Close()
		return err
	}
	client := pruntime.AppServer{API: appserver.NewAPI(process.Client), Client: process.Client}
	r, err := pruntime.New(in, client, process, lock)
	if err != nil {
		_ = process.Close(context.Background())
		_ = lock.Close()
		return err
	}
	journal, err := governance.OpenJournal(governance.JournalConfig{Directory: filepath.Join(in.StateDir(), "audit")})
	if err != nil {
		_ = r.Close(context.Background())
		return fmt.Errorf("open audit journal: %w", err)
	}
	defer journal.Close()
	r.SetRequestHandler(func(requestCtx context.Context, request appserver.ServerRequest, approvalInput io.Reader) error {
		prompt := approvalPrompt(in.ApprovalMode(), approvalInput, output)
		decision, decisionErr := governance.HandleServerRequest(requestCtx, governance.Policy{Roots: []string{in.WorkingRoot()}}, prompt, journal, request)
		if decisionErr != nil {
			decision = map[string]string{"decision": string(governance.DecisionDecline)}
		}
		if err := client.ReplyRequest(requestCtx, request.ID, decision); err != nil {
			return err
		}
		return decisionErr
	})
	if err := r.Start(ctx, pruntime.Options{
		Resume: !cmd.New, ResumeID: cmd.ResumeID,
	}, Version); err != nil {
		_ = r.Close(context.Background())
		return err
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		for sig := range signals {
			if sig == os.Interrupt && r.HasActiveTurn() {
				_ = r.Interrupt(context.Background())
				continue
			}
			cancel()
			return
		}
	}()
	return r.Run(ctx, input, pruntime.Terminal{Out: output})
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

func approvalPrompt(mode instance.ApprovalMode, input io.Reader, output io.Writer) governance.Prompt {
	if mode != instance.ApprovalAsk {
		return nil
	}
	return governance.TerminalPrompt{Input: input, Output: output}
}
