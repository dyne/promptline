package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"batchat/internal/chat"
	"batchat/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Printf("Error loading config: %v, using defaults", err)
		cfg = config.DefaultConfig()
	}

	// Check for API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}
	
	if apiKey == "" && cfg.APIKey == "" {
		log.Fatal("No API key found. Please set OPENAI_API_KEY, DASHSCOPE_API_KEY environment variable or api_key in config.json")
	}
	
	if apiKey != "" {
		cfg.APIKey = apiKey
	}

	// Create chat session
	session := chat.NewSession(cfg)
	
	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nReceived interrupt signal, shutting down...")
		session.Close()
		os.Exit(0)
	}()
	
	// Close session when main function exits
	defer session.Close()

	fmt.Println("Welcome to Batchat - AI Chat CLI for Coders!")
	fmt.Println("Type 'help' for available commands or 'quit' to exit.")
	fmt.Println()

	// Main chat loop
	for {
		input, err := session.GetInput()
		if err != nil {
			log.Printf("Error reading input: %v", err)
			continue
		}

		// Handle special commands
		switch strings.ToLower(input) {
		case "quit", "exit":
			fmt.Println("Goodbye!")
			return
		case "clear":
			session.ClearHistory()
			fmt.Println("Conversation history cleared.")
			continue
		case "history":
			session.PrintHistory()
			continue
		case "help":
			fmt.Println("Available commands:")
			fmt.Println("  quit/exit - Exit the application")
			fmt.Println("  clear     - Clear conversation history")
			fmt.Println("  history   - Display conversation history")
			fmt.Println("  generate  - Generate Python code for your batch task")
			fmt.Println("  help      - Show this help message")
			continue
		case "generate":
			err := session.GeneratePythonCode()
			if err != nil {
				log.Printf("Error generating code: %v", err)
			}
			continue
		case "":
			// Empty input, just prompt again
			continue
		}

		// Get response from AI
		fmt.Print("Assistant: ")
		err = session.GetStreamingResponse(input)
		if err != nil {
			log.Printf("Error getting response: %v", err)
		}
		fmt.Println()
	}
}