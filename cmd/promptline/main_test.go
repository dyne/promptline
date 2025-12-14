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
	"os"
	"testing"
)

func TestInitLogger(t *testing.T) {
	// Test with debug mode off - just ensure it doesn't crash
	_, err := initLogger(false, "")
	if err != nil {
		t.Fatalf("initLogger failed: %v", err)
	}

	// Test with debug mode on
	_, err = initLogger(true, "")
	if err != nil {
		t.Fatalf("initLogger with debug failed: %v", err)
	}

	// If we got here without panicking, test passed
}

func TestInitLoggerWithFile(t *testing.T) {
	tempDir := t.TempDir()
	logFile := tempDir + "/test.log"

	logger, err := initLogger(false, logFile)
	if err != nil {
		t.Fatalf("initLogger failed: %v", err)
	}

	// Write a log message
	logger.Info().Msg("Test message")

	// Verify file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("Log file was not created")
	}

	// Verify content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Log file is empty")
	}
}

func TestInitLoggerDefaultOutput(t *testing.T) {
	// Without log file, should use io.Discard
	logger, err := initLogger(false, "")
	if err != nil {
		t.Fatalf("initLogger failed: %v", err)
	}

	// Should not panic when logging
	logger.Info().Msg("This should be discarded")
	logger.Debug().Msg("This too")
}

func TestDebugModeFlagDefault(t *testing.T) {
	if debugMode == nil {
		t.Error("debugMode flag should be defined")
	}
}

func TestLogFileFlagDefault(t *testing.T) {
	if logFile == nil {
		t.Error("logFile flag should be defined")
	}
}
