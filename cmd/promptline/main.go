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

	"github.com/rs/zerolog"
)

var (
	debugMode = flag.Bool("d", false, "Enable debug mode")
	logFile   = flag.String("log-file", "", "Log file path (logs disabled by default)")
)

func main() {
	flag.Parse()

	// Initialize logger
	logger, err := initLogger(*debugMode, *logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	logger.Info().Msg("Promptline starting")

	// Check if we're running in batch mode (with "-" argument)
	args := flag.Args()
	if len(args) > 0 && args[0] == "-" {
		runBatchMode(logger)
		return
	}

	// Run in normal TUI mode
	runTUIMode(logger)
}

func initLogger(debug bool, logFilePath string) (zerolog.Logger, error) {
	// Set log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// Configure output
	var output io.Writer
	if logFilePath != "" {
		// Log to file only
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return zerolog.Logger{}, fmt.Errorf("failed to open log file: %w", err)
		}
		output = file
	} else {
		// No logging to console by default - use io.Discard
		output = io.Discard
	}

	// Create logger with timestamp
	return zerolog.New(output).With().Timestamp().Logger(), nil
}
