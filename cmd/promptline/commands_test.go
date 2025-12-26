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
	"testing"

	"github.com/rs/zerolog"
	"promptline/internal/chat"
	"promptline/internal/config"
)

func TestGetAvailableCommands(t *testing.T) {
	commands := getAvailableCommands()

	if len(commands) == 0 {
		t.Fatal("Expected non-empty command list")
	}

	// Check for essential commands
	essentialCommands := []string{"help", "quit", "clear", "history"}
	for _, essential := range essentialCommands {
		found := false
		for _, cmd := range commands {
			if cmd.Name == essential {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected essential command '%s' to be available", essential)
		}
	}
}

func TestHandleCommandHelp(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	debugMode := false

	// Help command should not quit
	shouldQuit := handleCommand("/help", session, logger, &debugMode)

	if shouldQuit {
		t.Error("Help command should not trigger quit")
	}
}

func TestHandleCommandClear(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	session.AddMessage("user", "Test message")

	logger := zerolog.Nop()
	debugMode := false

	shouldQuit := handleCommand("/clear", session, logger, &debugMode)

	if shouldQuit {
		t.Error("Clear command should not trigger quit")
	}

	// History should be cleared (only system message remains)
	history := session.GetHistory()
	if len(history) != 0 {
		t.Errorf("Expected empty history after clear, got %d messages", len(history))
	}
}

func TestHandleCommandQuit(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	debugMode := false

	// Quit command should return true
	shouldQuit := handleCommand("/quit", session, logger, &debugMode)

	if !shouldQuit {
		t.Error("Quit command should trigger quit")
	}

	// Exit should also trigger quit
	shouldQuit = handleCommand("/exit", session, logger, &debugMode)

	if !shouldQuit {
		t.Error("Exit command should trigger quit")
	}
}

func TestHandleCommandDebug(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	debugMode := false

	// Toggle debug on
	shouldQuit := handleCommand("/debug", session, logger, &debugMode)

	if shouldQuit {
		t.Error("Debug command should not trigger quit")
	}

	if !debugMode {
		t.Error("Debug mode should be enabled")
	}

	// Toggle debug off
	handleCommand("/debug", session, logger, &debugMode)

	if debugMode {
		t.Error("Debug mode should be disabled")
	}
}

func TestHandleCommandUnknown(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	debugMode := false

	// Unknown command should not quit
	shouldQuit := handleCommand("/nonexistent", session, logger, &debugMode)

	if shouldQuit {
		t.Error("Unknown command should not trigger quit")
	}
}

func TestHandleCommandCaseInsensitive(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	debugMode := false

	// Should handle uppercase
	shouldQuit := handleCommand("/QUIT", session, logger, &debugMode)

	if !shouldQuit {
		t.Error("QUIT (uppercase) should trigger quit")
	}
}

func TestShowHistoryEmpty(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)

	// Should not panic on empty history
	showHistory(session)
}

func TestShowHistoryWithMessages(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	session.AddMessage("user", "Hello")
	session.AddMessage("assistant", "Hi")

	// Should not panic with messages
	showHistory(session)
}

func TestShowPermissions(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)

	// Should not panic
	showPermissions(session)
}

func TestGetCommandCompleter(t *testing.T) {
	completer := getCommandCompleter()

	if completer == nil {
		t.Fatal("Expected non-nil completer")
	}

	// Test that it creates completions for available commands
	// The completer should have children for each command
	commands := getAvailableCommands()
	if len(commands) == 0 {
		t.Fatal("No commands available for completer")
	}
}

func TestCommandStructure(t *testing.T) {
	cmd := Command{
		Name:        "test",
		Description: "Test command",
	}

	if cmd.Name != "test" {
		t.Errorf("Expected name 'test', got %s", cmd.Name)
	}

	if cmd.Description != "Test command" {
		t.Errorf("Expected description 'Test command', got %s", cmd.Description)
	}
}

func TestShowHelp(t *testing.T) {
	// Should not panic
	showHelp()
}

func TestHandleCommandTrimming(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	debugMode := false

	// Should handle commands with whitespace - note the slash should not have spaces before it
	shouldQuit := handleCommand("/quit  ", session, logger, &debugMode)

	if !shouldQuit {
		t.Error("Quit with trailing whitespace should trigger quit")
	}
}

func TestHandleHistoryCommand(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	session.AddMessage("user", "Test")

	logger := zerolog.Nop()
	debugMode := false

	shouldQuit := handleCommand("/history", session, logger, &debugMode)

	if shouldQuit {
		t.Error("History command should not trigger quit")
	}
}

func TestHandlePermissionsCommand(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	debugMode := false

	shouldQuit := handleCommand("/permissions", session, logger, &debugMode)

	if shouldQuit {
		t.Error("Permissions command should not trigger quit")
	}
}
