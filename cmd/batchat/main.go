package main

import (
	"fmt"
	"os"

	"batchat/internal/chat"
	"batchat/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check if API key is set
	if cfg.APIKey == "" {
		// Try to get from environment variable
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			fmt.Fprintln(os.Stderr, "No API key found. Please set OPENAI_API_KEY environment variable or add it to config.json")
			os.Exit(1)
		}
		cfg.APIKey = apiKey
	}

	// Create chat session
	session := chat.NewSession(cfg)

	// Show welcome message
	fmt.Println("AI Chat CLI for Coders - Batch Job Assistant")
	fmt.Println("This tool helps you define batch processing tasks and generates Python code using the openbatch library.")
	fmt.Println("Type 'quit' or 'exit' to end the conversation")
	fmt.Println("Type 'clear' to clear conversation history")
	fmt.Println("Type 'history' to see conversation history")
	fmt.Println("Type 'generate' when you're ready to generate Python code for your batch task")
	fmt.Println("---")

	// Main conversation loop
	for {
		// Get user input
		input, err := session.GetInput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		// Handle special commands
		switch input {
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
		case "":
			// Empty input, just show prompt again
			continue
		case "generate":
			// Special handling for code generation
			if err := session.GeneratePythonCode(); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating code: %v\n", err)
			}
			continue
		}

		// Get response from AI
		fmt.Print("Assistant: ")
		response, err := session.GetResponse(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting response: %v\n", err)
			continue
		}
		
		fmt.Println(response)
		fmt.Println()
	}
}