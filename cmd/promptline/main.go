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

	"github.com/rs/zerolog"

	"promptline/internal/appserver"
	"promptline/internal/instance"
	pruntime "promptline/internal/runtime"
)

var (
	// Retained only until the v1 UI is removed by v2-cutover.
	dryRun = new(bool)
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

func initLogger(debug bool, logFilePath string) (zerolog.Logger, io.Closer, error) {
	// Set log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// Configure output
	var output io.Writer
	var closer io.Closer
	if debug {
		if logFilePath == "" {
			cwd, cwdErr := os.Getwd()
			if cwdErr != nil {
				return zerolog.Logger{}, nil, fmt.Errorf("failed to determine default log path: %w", cwdErr)
			}
			logFilePath = filepath.Join(cwd, "promptline_debug.log")
		}

		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fall back to a temp file if the default location is not writable.
			tmp, tmpErr := os.CreateTemp("", "promptline_debug_*.log")
			if tmpErr != nil {
				return zerolog.Logger{}, nil, fmt.Errorf("failed to open log file: %w", err)
			}
			file = tmp
		}
		closer = file
		output = file
	} else {
		// Logging is disabled when debug mode is off
		output = io.Discard
	}

	// Create logger with timestamp
	return zerolog.New(output).With().Timestamp().Logger(), closer, nil
}
