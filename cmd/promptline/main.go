package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"promptline/internal/chat"
	"promptline/internal/config"
	"promptline/internal/theme"
	promptui "promptline/internal/ui"
)

func main() {
	// Check if we're running in batch mode (with "-" argument)
	if len(os.Args) > 1 && os.Args[1] == "-" {
		runBatchMode()
		return
	}

	// Run in normal TUI mode
	runTUIMode()
}

func runBatchMode() {
	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create chat session
	session := chat.NewSession(cfg)
	defer session.Close()

	// Read input from stdin
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := scanner.Text()

		// Get response from AI
		response, err := session.GetResponse(input)
		if err != nil {
			log.Fatalf("Error getting response: %v", err)
		}

		// Output response
		fmt.Println(response)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
}

func runTUIMode() {
	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load theme
	tuiTheme, err := theme.LoadTheme("theme.json")
	if err != nil {
		log.Fatalf("Failed to load theme: %v", err)
	}

	// Create chat session
	session := chat.NewSession(cfg)
	defer session.Close()

	uiApp := promptui.New(session, tuiTheme)
	if err := uiApp.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
