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
	"github.com/sashabaranov/go-openai"

	"promptline/internal/chat"
	"promptline/internal/commands"
	"promptline/internal/config"
	"promptline/internal/theme"
)

// loadHistoryFromFile loads command history from readline history file
func loadHistoryFromFile(filepath string) []string {
	history := make([]string, 0)

	file, err := os.Open(filepath)
	if err != nil {
		// File doesn't exist yet, return empty history
		return history
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			history = append(history, line)
		}
	}

	return history
}

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

	// State variables
	var isProcessing bool
	var processingMutex sync.Mutex
	var cancelFunc context.CancelFunc
	var ctx context.Context
	var debugMode bool = false

	// Create command registry with debug mode reference
	cmdRegistry := commands.NewRegistry(&debugMode)

	// Create TUI application
	app := tview.NewApplication()

	// Input history navigation - load from readline history file
	inputHistory := loadHistoryFromFile(".promptline_history")
	historyIndex := -1

	// Create UI components
	header := tview.NewTextView().
		SetText("Promptline - TUI AI chat from dyne.org\nCtrl+C: Cancel | Ctrl+Q: Quit\n").
		SetTextColor(tcell.GetColor(tuiTheme.HeaderTextColor)).
		SetDynamicColors(true)

	chatView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	chatView.SetBorder(false)

	// Progress indicator
	progressIndicator := tview.NewTextView().
		SetText("").
		SetTextColor(tcell.GetColor(tuiTheme.ProgressIndicatorColor)).
		SetDynamicColors(true)

	// Horizontal separator with hint
	separator := tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.GetColor(tuiTheme.BorderColor))
	separator.SetBackgroundColor(tcell.ColorBlack)

	// Update separator width on draw and add Ctrl+Enter hint
	separator.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		hint := " Ctrl+Enter to send "
		hintLen := len(hint)
		lineLen := width - hintLen
		if lineLen < 0 {
			lineLen = 0
		}
		separator.SetText(strings.Repeat("─", lineLen) + hint)
		return x, y, width, height
	})

	// Multiline input area with elastic height
	inputArea := tview.NewTextArea().
		SetPlaceholder("Type your message... (Ctrl+Enter to send)")
	inputArea.SetBackgroundColor(tcell.GetColor(tuiTheme.InputBackgroundColor))
	inputArea.SetTextStyle(tcell.StyleDefault.
		Foreground(tcell.GetColor(tuiTheme.InputTextColor)).
		Background(tcell.GetColor(tuiTheme.InputBackgroundColor)))
	inputArea.SetPlaceholderStyle(tcell.StyleDefault.
		Foreground(tcell.GetColor(tuiTheme.BorderColor)).
		Background(tcell.GetColor(tuiTheme.InputBackgroundColor)))
	inputArea.SetBorder(false)

	// Input area elastic height management
	const minInputHeight = 5
	const maxInputHeight = 15
	currentInputHeight := minInputHeight

	// Create layout
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(header, 2, 1, false).
		AddItem(chatView, 0, 1, false).
		AddItem(progressIndicator, 1, 1, false).
		AddItem(separator, 1, 1, false).
		AddItem(inputArea, currentInputHeight, 1, true)

	// Elastic height monitoring (runs in background)
	go func() {
		lastLineCount := 0
		for {
			time.Sleep(100 * time.Millisecond)

			text := inputArea.GetText()
			lineCount := strings.Count(text, "\n") + 1

			// Only update if line count changed
			if lineCount != lastLineCount {
				lastLineCount = lineCount

				var newHeight int
				if lineCount <= minInputHeight {
					newHeight = minInputHeight
				} else if lineCount <= minInputHeight*2 {
					newHeight = minInputHeight * 2
				} else {
					newHeight = maxInputHeight
				}

				if newHeight != currentInputHeight {
					currentInputHeight = newHeight
					app.QueueUpdateDraw(func() {
						flex.ResizeItem(inputArea, currentInputHeight, 1)
					})
				}
			}
		}
	}()

	// Focus management
	app.SetFocus(inputArea)

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

	// Handle input submission function
	handleSubmit := func() {
		text := inputArea.GetText()
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}

		// Clear input area
		inputArea.SetText("", true)

		// Add to input history for arrow key navigation
		inputHistory = append(inputHistory, text)
		historyIndex = -1 // Reset history navigation

		// Save to readline history file
		if session.RL != nil {
			session.RL.SaveHistory(text)
		}

		// Check if it's a command
		if cmdRegistry.Execute(text, session, chatView, tuiTheme, app) {
			return
		}

		// Display user message
		currentText := chatView.GetText(false)
		if currentText != "" {
			currentText += "\n"
		}
		newText := currentText + fmt.Sprintf("[%s]User:[-] %s", tuiTheme.ChatUserColor, text)
		chatView.SetText(newText)

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

			// Loop to handle tool calls - AI may need multiple turns
			includeUserMessage := true
			prompt := text

			for {
				events := make(chan chat.StreamEvent)
				go session.StreamResponseWithContext(ctx, prompt, includeUserMessage, events)

				fullResponse := ""
				assistantPrefixShown := false
				var toolCalls []openai.ToolCall

				for event := range events {
					switch event.Type {
					case chat.StreamEventContent:
						content := event.Content
						fullResponse += content
						if !assistantPrefixShown {
							app.QueueUpdateDraw(func() {
								currentText := chatView.GetText(false)
								newText := currentText + fmt.Sprintf("\n[%s]Assistant:[-] ", tuiTheme.ChatAssistantColor)
								chatView.SetText(newText)
							})
							assistantPrefixShown = true
						}
						app.QueueUpdateDraw(func() {
							currentText := chatView.GetText(false)
							newText := currentText + content
							chatView.SetText(newText)
							chatView.ScrollToEnd()
						})
					case chat.StreamEventToolCall:
						if event.ToolCall != nil {
							toolCalls = append(toolCalls, *event.ToolCall)
						}
					case chat.StreamEventError:
						err := event.Err
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

				// If no tool calls were made, we're done
				if len(toolCalls) == 0 {
					if debugMode && fullResponse != "" {
						app.QueueUpdateDraw(func() {
							currentText := chatView.GetText(false)
							currentText += fmt.Sprintf("\n[%s]DEBUG - Full Response:[-]\n%s\n", tuiTheme.ChatErrorColor, fullResponse)
							chatView.SetText(currentText)
							chatView.ScrollToEnd()
						})
					}
					return
				}

				// Execute tool calls and display results
				for _, tc := range toolCalls {
					result := session.ToolRegistry.ExecuteOpenAIToolCall(tc)
					session.AddToolResultMessage(tc, result)

					toolCallInfo := session.FormatToolCallDisplay(tc, result)
					app.QueueUpdateDraw(func() {
						currentText := chatView.GetText(false)
						currentText += fmt.Sprintf("\n[%s]%s[-]", tuiTheme.ProgressIndicatorColor, toolCallInfo)
						if debugMode {
							currentText += fmt.Sprintf("\n[%s]DEBUG - Tool Args:[-]\n%s\n", tuiTheme.ChatErrorColor, tc.Function.Arguments)
						}
						chatView.SetText(currentText)
						chatView.ScrollToEnd()
					})
				}

				// Request a follow-up without adding another user message
				includeUserMessage = false
				prompt = ""
			}
		}()
	}

	// Handle keyboard shortcuts and history navigation
	inputArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Ctrl+Enter to submit (check multiple ways this might be sent)
		if (event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModCtrl != 0) ||
			(event.Key() == tcell.KeyCtrlJ) ||
			(event.Rune() == '\n' && event.Modifiers()&tcell.ModCtrl != 0) {
			handleSubmit()
			return nil
		}

		switch event.Key() {
		case tcell.KeyCtrlC:
			// Ctrl+C: Cancel current operation (break signal)
			processingMutex.Lock()
			if isProcessing && cancelFunc != nil {
				cancelFunc()
				app.QueueUpdateDraw(func() {
					currentText := chatView.GetText(false)
					newText := currentText + fmt.Sprintf("\n[%s]⚠ Operation cancelled by user[-]", tuiTheme.ChatErrorColor)
					chatView.SetText(newText)
					chatView.ScrollToEnd()
				})
			}
			processingMutex.Unlock()
			return nil

		case tcell.KeyCtrlQ:
			// Ctrl+Q: Quit the application
			processingMutex.Lock()
			if isProcessing && cancelFunc != nil {
				cancelFunc()
			}
			processingMutex.Unlock()
			app.Stop()
			return nil

		case tcell.KeyUp:
			// Navigate backward through history (only when Ctrl is pressed)
			if event.Modifiers()&tcell.ModCtrl != 0 && len(inputHistory) > 0 {
				if historyIndex == -1 {
					historyIndex = len(inputHistory) - 1
				} else if historyIndex > 0 {
					historyIndex--
				}
				if historyIndex >= 0 && historyIndex < len(inputHistory) {
					inputArea.SetText(inputHistory[historyIndex], true)
				}
				return nil
			}

		case tcell.KeyDown:
			// Navigate forward through history (only when Ctrl is pressed)
			if event.Modifiers()&tcell.ModCtrl != 0 && len(inputHistory) > 0 && historyIndex != -1 {
				if historyIndex < len(inputHistory)-1 {
					historyIndex++
					inputArea.SetText(inputHistory[historyIndex], true)
				} else {
					// At the end, clear the input
					historyIndex = -1
					inputArea.SetText("", true)
				}
				return nil
			}
		}
		return event
	})

	// Set the root and run the application
	app.SetRoot(flex, true)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
