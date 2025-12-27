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
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

var (
	debugMode = flag.Bool("d", false, "Enable debug mode")
	logFile   = flag.String("log-file", "", "Log file path (logs disabled by default)")
	dryRun    = flag.Bool("dry-run", false, "Validate tool calls without executing them")
	version   = flag.Bool("version", false, "Display version information and exit")
)

// Version is set at build time via ldflags. Defaults to "dev".
var Version = "dev"

func main() {
	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Printf("promptline version %s\n", Version)
		os.Exit(0)
	}

	// Initialize logger
	logger, closer, err := initLogger(*debugMode, *logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if closer != nil {
		defer func() {
			_ = closer.Close()
		}()
	}
	logger.Info().Str("version", Version).Msg("Promptline starting")

	// Check if we're running in batch mode (with "-" argument)
	args := flag.Args()
	if len(args) > 0 && args[0] == "-" {
		runBatchMode(logger)
		return
	}

	// Run in normal TUI mode
	runTUIMode(logger)
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
