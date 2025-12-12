package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
	"promptline/internal/tools"
)

func TestSaveConversationHistory(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history.jsonl")
	
	cfg := &config.Config{
		APIKey:      "test-key",
		Model:       "gpt-4o-mini",
		HistoryFile: historyFile,
	}
	
	session := NewSession(cfg)
	session.AddMessage(openai.ChatMessageRoleUser, "Hello")
	session.AddMessage(openai.ChatMessageRoleAssistant, "Hi there!")
	
	err := session.SaveConversationHistory(historyFile)
	if err != nil {
		t.Fatalf("SaveConversationHistory failed: %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Fatal("History file was not created")
	}
	
	// Verify content
	content, err := os.ReadFile(historyFile)
	if err != nil {
		t.Fatalf("Failed to read history file: %v", err)
	}
	
	if len(content) == 0 {
		t.Fatal("History file is empty")
	}
}

func TestSaveConversationHistoryAppends(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history.jsonl")
	
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	session.AddMessage(openai.ChatMessageRoleUser, "Message 1")
	
	// Save first time
	err := session.SaveConversationHistory(historyFile)
	if err != nil {
		t.Fatalf("First save failed: %v", err)
	}
	
	// Add more messages
	session.AddMessage(openai.ChatMessageRoleAssistant, "Response 1")
	
	// Save again - should append
	err = session.SaveConversationHistory(historyFile)
	if err != nil {
		t.Fatalf("Second save failed: %v", err)
	}
	
	// Verify both messages are in file
	file, err := os.Open(historyFile)
	if err != nil {
		t.Fatalf("Failed to open history file: %v", err)
	}
	defer file.Close()
	
	var messages []openai.ChatCompletionMessage
	decoder := json.NewDecoder(file)
	for {
		var msg openai.ChatCompletionMessage
		if err := decoder.Decode(&msg); err != nil {
			break
		}
		messages = append(messages, msg)
	}
	
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}
}

func TestLoadConversationHistory(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history.jsonl")
	
	// Create a history file
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Hi!"},
	}
	
	file, err := os.Create(historyFile)
	if err != nil {
		t.Fatalf("Failed to create history file: %v", err)
	}
	
	encoder := json.NewEncoder(file)
	for _, msg := range messages {
		if err := encoder.Encode(msg); err != nil {
			t.Fatalf("Failed to encode message: %v", err)
		}
	}
	file.Close()
	
	// Load history
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	err = session.LoadConversationHistory(historyFile, 100)
	if err != nil {
		t.Fatalf("LoadConversationHistory failed: %v", err)
	}
	
	// Verify messages were loaded (excluding system message)
	history := session.GetHistory()
	if len(history) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(history))
	}
	
	if history[0].Content != "Hello" {
		t.Errorf("Expected first message 'Hello', got '%s'", history[0].Content)
	}
	
	if history[1].Content != "Hi!" {
		t.Errorf("Expected second message 'Hi!', got '%s'", history[1].Content)
	}
}

func TestLoadConversationHistoryWithLimit(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history.jsonl")
	
	// Create history with 5 messages
	file, err := os.Create(historyFile)
	if err != nil {
		t.Fatalf("Failed to create history file: %v", err)
	}
	
	encoder := json.NewEncoder(file)
	for i := 1; i <= 5; i++ {
		msg := openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "Message " + string(rune('0'+i)),
		}
		encoder.Encode(msg)
	}
	file.Close()
	
	// Load with limit of 2
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	err = session.LoadConversationHistory(historyFile, 2)
	if err != nil {
		t.Fatalf("LoadConversationHistory failed: %v", err)
	}
	
	history := session.GetHistory()
	if len(history) != 2 {
		t.Fatalf("Expected 2 messages (limited), got %d", len(history))
	}
}

func TestLoadConversationHistoryNonExistent(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	err := session.LoadConversationHistory("/nonexistent/path/history.jsonl", 100)
	
	// Should not error on non-existent file
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}
}

func TestLoadConversationHistoryCorruptedJSON(t *testing.T) {
	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "history.jsonl")
	
	// Create corrupted file
	err := os.WriteFile(historyFile, []byte("{invalid json}\n{more invalid"), 0644)
	if err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}
	
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	err = session.LoadConversationHistory(historyFile, 100)
	
	// Should return error for corrupted JSON
	if err == nil {
		t.Error("Expected error for corrupted JSON, got nil")
	}
}

func TestPrintHistory(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	session.AddMessage(openai.ChatMessageRoleUser, "Test message")
	
	// Just ensure it doesn't panic
	session.PrintHistory()
}

func TestFormatToolCallDisplay(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	
	toolCall := openai.ToolCall{
		ID:   "call_123",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_tool",
			Arguments: `{"arg1": "value1"}`,
		},
	}
	
	result := &tools.ToolResult{
		Function: "test_tool",
		Result:   "Success",
		Error:    nil,
	}
	
	formatted := session.FormatToolCallDisplay(toolCall, result)
	
	if formatted == "" {
		t.Error("Expected non-empty formatted result")
	}
	
	if !contains(formatted, "test_tool") {
		t.Error("Expected formatted result to contain tool name")
	}
}

func TestClose(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSession(cfg)
	err := session.Close()
	
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestSaveConversationHistoryNoNewMessages(t *testing.T) {
tempDir := t.TempDir()
historyFile := filepath.Join(tempDir, "history.jsonl")

cfg := &config.Config{
APIKey: "test-key",
Model:  "gpt-4o-mini",
}

session := NewSession(cfg)
session.AddMessage(openai.ChatMessageRoleUser, "Message 1")

// Save first time
err := session.SaveConversationHistory(historyFile)
if err != nil {
t.Fatalf("First save failed: %v", err)
}

// Save again without adding messages - should be no-op
err = session.SaveConversationHistory(historyFile)
if err != nil {
t.Fatalf("Second save failed: %v", err)
}

// File should still have only 1 message
file, err := os.Open(historyFile)
if err != nil {
t.Fatalf("Failed to open history file: %v", err)
}
defer file.Close()

var messages []openai.ChatCompletionMessage
decoder := json.NewDecoder(file)
for {
var msg openai.ChatCompletionMessage
if err := decoder.Decode(&msg); err != nil {
break
}
messages = append(messages, msg)
}

if len(messages) != 1 {
t.Fatalf("Expected 1 message (no duplicates), got %d", len(messages))
}
}

func TestNewSessionWithCustomHTTPClient(t *testing.T) {
cfg := &config.Config{
APIKey: "test-key",
APIURL: "https://custom.api.com/v1",
Model:  "gpt-4o-mini",
}

session := NewSession(cfg)

if session == nil {
t.Fatal("Expected non-nil session")
}

if session.Config.APIURL != "https://custom.api.com/v1" {
t.Errorf("Expected custom API URL, got %s", session.Config.APIURL)
}
}
