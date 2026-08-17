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
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "promptline:", err)
		return
	}
}

func run(args []string, input io.Reader, output, stderr io.Writer) error {
	cmd, err := pruntime.Parse(args, stderr)
	if err != nil {
		return err
	}
	if cmd.Version {
		_, err := fmt.Fprintf(output, "promptline version %s\n", Version)
		return err
	}
	in, err := instance.New(cmd.Instance)
	if err != nil {
		return err
	}
	if cmd.ToolboxServe {
		registry, err := tools.NewRegistryWithConfig(tools.Config{WorkingDirectory: in.WorkingDirectory(), Roots: []string{in.WorkingRoot()}})
		if err != nil {
			return err
		}
		defer registry.Close()
		server, err := mcp.NewServer(registry, input, output, in.OutputCaps().StdoutBytes)
		if err != nil {
			return err
		}
		return server.Serve(context.Background())
	}
	lock, err := in.AcquireLock()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
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
	r.SetRequestHandler(func(requestCtx context.Context, request appserver.ServerRequest) error {
		decision, decisionErr := governance.HandleServerRequest(requestCtx, governance.Policy{Roots: []string{in.WorkingRoot()}}, nil, journal, request)
		if decisionErr != nil {
			decision = map[string]string{"decision": string(governance.DecisionDecline)}
		}
		if err := client.ReplyRequest(requestCtx, request.ID, decision); err != nil {
			return err
		}
		return decisionErr
	})
	if err := r.Start(ctx, pruntime.Options{New: cmd.New, ResumeID: cmd.ResumeID}, Version); err != nil {
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
