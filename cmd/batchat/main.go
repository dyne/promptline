package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"batchat/internal/chat"
	"batchat/internal/config"
	"batchat/internal/theme"
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

	// Create TUI application
	app := tview.NewApplication()

	// State variables
	var isProcessing bool
	var processingMutex sync.Mutex
	var cancelFunc context.CancelFunc
	var ctx context.Context

	// Create UI components
	header := tview.NewTextView().
		SetText("Batchat - AI Chat for Batch Processing Jobs\nPress Ctrl+C or Ctrl+Q to quit\n").
		SetTextColor(tcell.GetColor(tuiTheme.HeaderTextColor)).
		SetDynamicColors(true)

	chatView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	chatView.SetBorder(true).SetTitle("Chat").SetBorderColor(tcell.GetColor(tuiTheme.BorderColor))

	// Progress indicator
	progressIndicator := tview.NewTextView().
		SetText("").
		SetTextColor(tcell.GetColor(tuiTheme.ProgressIndicatorColor)).
		SetDynamicColors(true)

	inputField := tview.NewInputField()
	inputField.SetLabel("User: ")
	inputField.SetFieldWidth(0) // Full width
	inputField.SetLabelColor(tcell.GetColor(tuiTheme.InputLabelColor))
	inputField.SetBackgroundColor(tcell.GetColor(tuiTheme.InputBackgroundColor))

	// Create layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(header, 2, 1, false).
		AddItem(chatView, 0, 1, false).
		AddItem(progressIndicator, 1, 1, false).
		AddItem(inputField, 3, 1, true)

	// Focus management
	app.SetFocus(inputField)

	// Progress animation
	go func() {
		chars := []string{"|", "/", "-", "\\"}
		i := 0
		for {
			processingMutex.Lock()
			processing := isProcessing
			processingMutex.Unlock()

			if processing {
				progressIndicator.SetText(fmt.Sprintf("[%s]Processing... %s[-]", tuiTheme.ProgressIndicatorColor, chars[i%len(chars)]))
				app.Draw()
				i++
			} else {
				progressIndicator.SetText("")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// Handle input
	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := inputField.GetText()
			if text == "" {
				return
			}

			// Clear input field
			inputField.SetText("")

			// Display user message
			currentText := chatView.GetText(false)
			if currentText != "" {
				currentText += "\n"
			}
			newText := currentText + fmt.Sprintf("[%s]User:[-] %s", tuiTheme.ChatUserColor, text)
			chatView.SetText(newText)

			// Check for special commands
			switch strings.ToLower(text) {
			case "quit", "exit":
				app.Stop()
				return
			case "clear":
				chatView.Clear()
				session.ClearHistory()
				chatView.SetText(fmt.Sprintf("[%s]Chat history cleared[-]", tuiTheme.ChatSuccessColor))
				return
			case "history":
				// Show a message that this feature needs implementation
				currentText = chatView.GetText(false)
				newText = currentText + "\n" + fmt.Sprintf("[%s]History command not yet implemented in TUI[-]", tuiTheme.ChatErrorColor)
				chatView.SetText(newText)
				return
			case "generate":
				// For now, we'll just show a message that code generation is handled differently in TUI
				currentText = chatView.GetText(false)
				newText = currentText + "\n" + fmt.Sprintf("[%s]Code generation is triggered with Ctrl+G[-]", tuiTheme.ChatErrorColor)
				chatView.SetText(newText)
				return
			}

			// Set processing state and create context
			processingMutex.Lock()
			isProcessing = true
			ctx, cancelFunc = context.WithCancel(context.Background())
			processingMutex.Unlock()

			// Get response from AI (using streaming for better UX)
			go func() {
				defer func() {
					processingMutex.Lock()
					isProcessing = false
					processingMutex.Unlock()
				}()

				// Create channels for streaming response
				responseChan := make(chan string)
				errorChan := make(chan error)

				// Start streaming response
				go session.StreamResponseWithContext(ctx, text, responseChan, errorChan)

				// Process the streaming response
				for {
					select {
					case content, ok := <-responseChan:
						if !ok {
							// Streaming finished
							app.QueueUpdateDraw(func() {
								chatView.ScrollToEnd()
							})
							return
						}
						// Update chat view
						app.QueueUpdateDraw(func() {
							currentText := chatView.GetText(false)
							newText := currentText + content
							chatView.SetText(newText)
							chatView.ScrollToEnd()
						})
					case err, ok := <-errorChan:
						if !ok {
							// Streaming finished normally
							app.QueueUpdateDraw(func() {
								chatView.ScrollToEnd()
							})
							return
						}
						// Handle error
						if err == context.Canceled {
							app.QueueUpdateDraw(func() {
								currentText := chatView.GetText(false)
								newText := currentText + fmt.Sprintf("\n[%s]Request cancelled[-]", tuiTheme.ChatErrorColor)
								chatView.SetText(newText)
								chatView.ScrollToEnd()
							})
						} else {
							app.QueueUpdateDraw(func() {
								currentText := chatView.GetText(false)
								newText := currentText + fmt.Sprintf("\n[%s]Error: %v[-]", tuiTheme.ChatErrorColor, err)
								chatView.SetText(newText)
								chatView.ScrollToEnd()
							})
						}
						return
					}
				}
			}()
		}
	})

	// Handle Ctrl+Q for quitting
	inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlQ:
			// Check if we're processing and cancel if needed
			processingMutex.Lock()
			if isProcessing && cancelFunc != nil {
				cancelFunc()
			}
			processingMutex.Unlock()

			// Quit the application
			app.Stop()
			return nil
		}
		return event
	})

	// Set the root and run the application
	app.SetRoot(flex, true)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}