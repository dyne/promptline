package main

import (
	"flag"
	"io"
	"log"
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
	logger := initLogger(*debugMode, *logFile)
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

func initLogger(debug bool, logFilePath string) zerolog.Logger {
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
			log.Fatalf("Failed to open log file: %v", err)
		}
		output = file
	} else {
		// No logging to console by default - use io.Discard
		output = io.Discard
	}

	// Create logger with timestamp
	return zerolog.New(output).With().Timestamp().Logger()
}
