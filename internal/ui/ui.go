package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sashabaranov/go-openai"

	"promptline/internal/chat"
	"promptline/internal/commands"
	"promptline/internal/theme"
	"promptline/internal/tools"
)

type toolDecision int

const (
	toolDecisionDeny toolDecision = iota
	toolDecisionAllowOnce
	toolDecisionAlwaysAllow
	toolDecisionCancelQuit
	toolDecisionConfirmQuit
)

// UI owns the Promptline TUI components and lifecycle.
type UI struct {
	app         *tview.Application
	session     *chat.Session
	theme       *theme.Theme
	cmdRegistry *commands.Registry

	header               *tview.TextView
	chatView             *tview.TextView
	progressIndicator    *tview.TextView
	separator            *tview.TextView
	inputArea            *tview.TextArea
	flex                 *tview.Flex
	permissionsContainer *tview.Flex
	permissionsPanel     *tview.Flex
	permissionsTable     *tview.Table
	mainColumns          *tview.Flex
	pages                *tview.Pages

	permissionsVisible bool

	history *History

	isProcessing    bool
	processingMutex sync.Mutex
	cancelFunc      context.CancelFunc

	debugMode bool

	minInputHeight     int
	maxInputHeight     int
	currentInputHeight int

	bgWG sync.WaitGroup
}

// New constructs a UI with all widgets and handlers wired.
func New(session *chat.Session, tuiTheme *theme.Theme) *UI {
	ui := &UI{
		app:                tview.NewApplication(),
		session:            session,
		theme:              tuiTheme,
		minInputHeight:     5,
		maxInputHeight:     15,
		currentInputHeight: 5,
	}

	ui.history = NewHistory(LoadHistoryFromFile(".promptline_history"))
	ui.cmdRegistry = commands.NewRegistry(&ui.debugMode)

	ui.buildLayout()
	ui.setupPermissionsHandlers()
	ui.setupInputHandlers()
	ui.setupGlobalInputCapture()

	return ui
}

// Run starts the TUI and blocks until the application stops or ctx is cancelled.
func (ui *UI) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	ui.startBackgroundWorkers(runCtx)

	ui.app.SetRoot(ui.pages, true)
	ui.app.SetFocus(ui.inputArea)

	go func() {
		<-runCtx.Done()
		ui.app.Stop()
	}()

	if err := ui.app.Run(); err != nil {
		cancel()
		ui.bgWG.Wait()
		return err
	}

	cancel()
	ui.bgWG.Wait()
	return nil
}

func (ui *UI) startBackgroundWorkers(ctx context.Context) {
	ui.bgWG.Add(2)
	go func() {
		defer ui.bgWG.Done()
		ui.runElasticHeight(ctx)
	}()
	go func() {
		defer ui.bgWG.Done()
		ui.runProgressIndicator(ctx)
	}()
}

func (ui *UI) runElasticHeight(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	lastLineCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			text := ui.inputArea.GetText()
			lineCount := strings.Count(text, "\n") + 1

			if lineCount == lastLineCount {
				continue
			}
			lastLineCount = lineCount

			newHeight := computeElasticHeight(lineCount, ui.minInputHeight, ui.maxInputHeight)
			if newHeight == ui.currentInputHeight {
				continue
			}
			ui.currentInputHeight = newHeight
			ui.app.QueueUpdateDraw(func() {
				ui.flex.ResizeItem(ui.inputArea, ui.currentInputHeight, 1)
			})
		}
	}
}

func (ui *UI) runProgressIndicator(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	chars := []string{"|", "/", "-", "\\"}
	i := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ui.processingMutex.Lock()
			processing := ui.isProcessing
			ui.processingMutex.Unlock()

			if processing {
				indicator := fmt.Sprintf("[%s]Processing... %s[-]", ui.theme.ProgressIndicatorColor, chars[i%len(chars)])
				i++
				ui.app.QueueUpdateDraw(func() {
					ui.progressIndicator.SetText(indicator)
				})
			} else {
				ui.app.QueueUpdateDraw(func() {
					ui.progressIndicator.SetText("")
				})
			}
		}
	}
}

func computeElasticHeight(lineCount, minHeight, maxHeight int) int {
	switch {
	case lineCount <= minHeight:
		return minHeight
	case lineCount <= minHeight*2:
		return minHeight * 2
	default:
		return maxHeight
	}
}

func (ui *UI) buildLayout() {
	ui.header = tview.NewTextView().
		SetText("Promptline - TUI AI chat from dyne.org\nCtrl+C: Cancel | Ctrl+Q: Quit\n").
		SetTextColor(tcell.GetColor(ui.theme.HeaderTextColor)).
		SetDynamicColors(true)

	ui.chatView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	ui.chatView.SetBorder(false)

	ui.progressIndicator = tview.NewTextView().
		SetText("").
		SetTextColor(tcell.GetColor(ui.theme.ProgressIndicatorColor)).
		SetDynamicColors(true)

	ui.separator = tview.NewTextView().
		SetDynamicColors(true).
		SetTextColor(tcell.GetColor(ui.theme.BorderColor))
	ui.separator.SetBackgroundColor(tcell.ColorBlack)
	ui.separator.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		hint := " Ctrl+Enter to send "
		hintLen := len(hint)
		lineLen := width - hintLen
		if lineLen < 0 {
			lineLen = 0
		}
		ui.separator.SetText(strings.Repeat("─", lineLen) + hint)
		return x, y, width, height
	})

	ui.inputArea = tview.NewTextArea().
		SetPlaceholder("Type your message... (Ctrl+Enter to send)")
	ui.inputArea.SetBackgroundColor(tcell.GetColor(ui.theme.InputBackgroundColor))
	ui.inputArea.SetTextStyle(tcell.StyleDefault.
		Foreground(tcell.GetColor(ui.theme.InputTextColor)).
		Background(tcell.GetColor(ui.theme.InputBackgroundColor)))
	ui.inputArea.SetPlaceholderStyle(tcell.StyleDefault.
		Foreground(tcell.GetColor(ui.theme.BorderColor)).
		Background(tcell.GetColor(ui.theme.InputBackgroundColor)))
	ui.inputArea.SetBorder(false)

	ui.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.header, 2, 1, false).
		AddItem(ui.chatView, 0, 1, false).
		AddItem(ui.progressIndicator, 1, 1, false).
		AddItem(ui.separator, 1, 1, false).
		AddItem(ui.inputArea, ui.currentInputHeight, 1, true)

	ui.permissionsContainer = tview.NewFlex().SetDirection(tview.FlexRow)

	ui.mainColumns = tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(ui.flex, 0, 1, true)

	ui.pages = tview.NewPages().
		AddPage("main", ui.mainColumns, true, true)

	ui.buildPermissionsPanel()
}

func (ui *UI) buildPermissionsPanel() {
	ui.permissionsTable = tview.NewTable().
		SetSelectable(true, true).
		SetBorders(true).
		SetFixed(1, 0)
	ui.permissionsTable.SetTitle(" Tool Permissions ").SetBorder(true)

	permissionsHeader := tview.NewTextView().
		SetText("[::b]Tool Permissions\nToggle allow/confirm with Enter or Space; ESC to close.").
		SetDynamicColors(true).
		SetBorder(true).
		SetTitle(" Control ")

	permissionsFooter := tview.NewTextView().
		SetText("[::b]Columns[::-]: Tool | Allow | Require Confirmation").
		SetDynamicColors(true).
		SetBorder(true)

	ui.permissionsPanel = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(permissionsHeader, 3, 1, false).
		AddItem(ui.permissionsTable, 0, 1, true).
		AddItem(permissionsFooter, 2, 1, false)
}

func (ui *UI) setupPermissionsHandlers() {
	ui.permissionsTable.SetSelectedFunc(func(row, column int) {
		ui.togglePermission(row, column)
	})

	ui.permissionsTable.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			ui.hidePermissions()
		}
	})

	ui.permissionsTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && (event.Rune() == 'q' || event.Rune() == 'Q') {
			ui.hidePermissions()
			return nil
		}
		if event.Key() == tcell.KeyCtrlQ {
			ui.promptForQuit()
			return nil
		}
		if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
			row, column := ui.permissionsTable.GetSelection()
			ui.togglePermission(row, column)
			return nil
		}
		return event
	})

	ui.cmdRegistry.SetPermissionsHandler(func(session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
		if ui.permissionsVisible {
			ui.hidePermissions()
		} else {
			ui.showPermissions()
		}
		return true
	})
}

func (ui *UI) setupInputHandlers() {
	ui.inputArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Ctrl+Enter to submit
		if (event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModCtrl != 0) ||
			(event.Key() == tcell.KeyCtrlJ) ||
			(event.Rune() == '\n' && event.Modifiers()&tcell.ModCtrl != 0) {
			ui.handleSubmit()
			return nil
		}

		switch event.Key() {
		case tcell.KeyCtrlC:
			ui.processingMutex.Lock()
			if ui.isProcessing && ui.cancelFunc != nil {
				ui.cancelFunc()
				ui.app.QueueUpdateDraw(func() {
					currentText := ui.chatView.GetText(false)
					newText := currentText + fmt.Sprintf("\n[%s]⚠ Operation cancelled by user[-]", ui.theme.ChatErrorColor)
					ui.chatView.SetText(newText)
					ui.chatView.ScrollToEnd()
				})
			}
			ui.processingMutex.Unlock()
			return nil

		case tcell.KeyCtrlQ:
			ui.processingMutex.Lock()
			if ui.isProcessing && ui.cancelFunc != nil {
				ui.cancelFunc()
			}
			ui.processingMutex.Unlock()
			ui.promptForQuit()
			return nil

		case tcell.KeyUp:
			if event.Modifiers()&tcell.ModCtrl != 0 {
				if prev, ok := ui.history.Prev(); ok {
					ui.inputArea.SetText(prev, true)
				}
				return nil
			}

		case tcell.KeyDown:
			if event.Modifiers()&tcell.ModCtrl != 0 {
				if next, ok := ui.history.Next(); ok {
					ui.inputArea.SetText(next, true)
				}
				return nil
			}
		}
		return event
	})
}

func (ui *UI) setupGlobalInputCapture() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.permissionsVisible && event.Key() == tcell.KeyEscape {
			ui.hidePermissions()
			return nil
		}
		if event.Key() == tcell.KeyCtrlQ {
			ui.promptForQuit()
			return nil
		}
		return event
	})
}

func (ui *UI) togglePermission(row, column int) {
	if row == 0 {
		return
	}
	names := ui.session.ToolRegistry.GetToolNames()
	sort.Strings(names)
	if row-1 >= len(names) {
		return
	}
	name := names[row-1]
	perm := ui.session.ToolRegistry.GetPermission(name)
	switch column {
	case 1:
		ui.session.ToolRegistry.SetAllowed(name, !perm.Allowed)
	case 2:
		ui.session.ToolRegistry.SetRequireConfirmation(name, !perm.RequireConfirmation)
	default:
		return
	}
	ui.refreshPermissions()
}

func (ui *UI) refreshPermissions() {
	names := ui.session.ToolRegistry.GetToolNames()
	sort.Strings(names)

	ui.permissionsTable.Clear()
	ui.permissionsTable.SetCell(0, 0, tview.NewTableCell(" Tool ").SetAlign(tview.AlignLeft).SetSelectable(false).SetTextColor(tcell.GetColor(ui.theme.ChatUserColor)))
	ui.permissionsTable.SetCell(0, 1, tview.NewTableCell(" Allow ").SetAlign(tview.AlignCenter).SetSelectable(false).SetTextColor(tcell.GetColor(ui.theme.ChatAssistantColor)))
	ui.permissionsTable.SetCell(0, 2, tview.NewTableCell(" Require Confirmation ").SetAlign(tview.AlignCenter).SetSelectable(false).SetTextColor(tcell.GetColor(ui.theme.ChatAssistantColor)))

	for i, name := range names {
		perm := ui.session.ToolRegistry.GetPermission(name)
		row := i + 1
		ui.permissionsTable.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf(" %s ", name)).SetAlign(tview.AlignLeft))
		ui.permissionsTable.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf(" %s ", ui.switchText(perm.Allowed))).SetAlign(tview.AlignCenter))
		ui.permissionsTable.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf(" %s ", ui.switchText(perm.RequireConfirmation))).SetAlign(tview.AlignCenter))
	}
}

func (ui *UI) hidePermissions() {
	if !ui.permissionsVisible {
		return
	}
	ui.permissionsVisible = false
	ui.app.QueueUpdateDraw(func() {
		ui.pages.RemovePage("permissions-overlay")
		ui.app.SetFocus(ui.inputArea)
	})
}

func (ui *UI) showPermissions() {
	ui.permissionsVisible = true
	ui.app.QueueUpdateDraw(func() {
		ui.refreshPermissions()
		row := 1
		if ui.permissionsTable.GetRowCount() == 1 {
			row = 0
		}
		ui.permissionsTable.Select(row, 1)

		overlay := tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(tview.NewBox(), 0, 1, false).
			AddItem(ui.permissionsPanel, 0, 1, true)

		ui.pages.RemovePage("permissions-overlay")
		ui.pages.AddPage("permissions-overlay", overlay, true, true)
		ui.app.SetFocus(ui.permissionsTable)
	})
}

func (ui *UI) switchText(on bool) string {
	if on {
		return fmt.Sprintf("[%s]ON[-]", ui.theme.ChatSuccessColor)
	}
	return fmt.Sprintf("[%s]OFF[-]", ui.theme.ChatErrorColor)
}

func (ui *UI) promptForQuit() {
	go func() {
		choice := ui.showModalPrompt("quit-confirm", "Quit Promptline?", []string{"Cancel", "Quit"}, ui.inputArea)
		if choice == 1 {
			ui.app.Stop()
		}
	}()
}

func (ui *UI) promptForToolPermission(tc openai.ToolCall, perm tools.Permission, argsPreview string) toolDecision {
	toolName := tc.Function.Name
	if toolName == "" {
		toolName = "unknown_tool"
	}

	reason := "allowed but requires confirmation."
	if !perm.Allowed {
		reason = "blocked by policy."
	}

	modalText := fmt.Sprintf("Tool '%s' requested (%s)\nArgs: %s\n\nAllow execution?", toolName, reason, argsPreview)
	choice := ui.showModalPrompt("tool-permission", modalText, []string{"Allow once", "Always allow", "Deny"}, ui.inputArea)
	switch choice {
	case 0:
		return toolDecisionAllowOnce
	case 1:
		return toolDecisionAlwaysAllow
	default:
		return toolDecisionDeny
	}
}

func (ui *UI) showModalPrompt(name, text string, buttons []string, focusAfter tview.Primitive) int {
	doneCh := make(chan int, 1)
	escapeIndex := resolveEscapeButton(buttons)
	sendChoice := func(idx int) {
		select {
		case doneCh <- idx:
		default:
		}
	}
	modal := tview.NewModal().
		SetText(text).
		AddButtons(buttons).
		SetDoneFunc(func(buttonIndex int, _ string) {
			sendChoice(buttonIndex)
		}).
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				sendChoice(escapeIndex)
				return nil
			}
			return event
		})

	ui.app.QueueUpdateDraw(func() {
		ui.pages.RemovePage(name)
		ui.pages.AddPage(name, modal, true, true)
		ui.app.SetFocus(modal)
	})

	selection := <-doneCh

	ui.app.QueueUpdateDraw(func() {
		ui.pages.RemovePage(name)
		if focusAfter != nil {
			ui.app.SetFocus(focusAfter)
		}
	})

	return selection
}

func (ui *UI) showModalPromptAsync(name, text string, buttons []string, focusAfter tview.Primitive, onDone func(int)) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons(buttons)

	escapeIndex := resolveEscapeButton(buttons)
	doneOnce := sync.Once{}
	finish := func(buttonIndex int) {
		doneOnce.Do(func() {
			ui.app.QueueUpdateDraw(func() {
				ui.pages.RemovePage(name)
				if focusAfter != nil {
					ui.app.SetFocus(focusAfter)
				}
				if onDone != nil {
					onDone(buttonIndex)
				}
			})
		})
	}

	modal.SetDoneFunc(func(buttonIndex int, _ string) {
		finish(buttonIndex)
	})

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			finish(escapeIndex)
			return nil
		}
		return event
	})

	ui.app.QueueUpdateDraw(func() {
		ui.pages.RemovePage(name)
		ui.pages.AddPage(name, modal, true, true)
		ui.app.SetFocus(modal)
	})
}

func resolveEscapeButton(buttons []string) int {
	if len(buttons) == 0 {
		return 0
	}
	denyIdx := -1
	for i, b := range buttons {
		lower := strings.ToLower(b)
		if strings.Contains(lower, "cancel") {
			return i
		}
		if strings.Contains(lower, "deny") || strings.Contains(lower, "no") {
			denyIdx = i
		}
	}
	if denyIdx != -1 {
		return denyIdx
	}
	return 0
}

func (ui *UI) handleSubmit() {
	text := strings.TrimSpace(ui.inputArea.GetText())
	if text == "" {
		return
	}

	ui.inputArea.SetText("", true)

	ui.history.Add(text)

	if ui.session.RL != nil {
		ui.session.RL.SaveHistory(text)
	}

	if ui.cmdRegistry.Execute(text, ui.session, ui.chatView, ui.theme, ui.app) {
		return
	}

	ui.appendUserMessage(text)

	ui.processingMutex.Lock()
	ui.isProcessing = true
	reqCtx, cancel := context.WithCancel(context.Background())
	ui.cancelFunc = cancel
	ui.processingMutex.Unlock()

	go ui.handleConversation(reqCtx, text)
}

func (ui *UI) appendUserMessage(text string) {
	currentText := ui.chatView.GetText(false)
	if currentText != "" {
		currentText += "\n"
	}
	newText := currentText + fmt.Sprintf("[%s]User:[-] %s", ui.theme.ChatUserColor, text)
	ui.chatView.SetText(newText)
}

func (ui *UI) handleConversation(ctx context.Context, text string) {
	defer func() {
		ui.processingMutex.Lock()
		ui.isProcessing = false
		ui.processingMutex.Unlock()
	}()

	includeUserMessage := true
	prompt := text

	for {
		events := make(chan chat.StreamEvent)
		go ui.session.StreamResponseWithContext(ctx, prompt, includeUserMessage, events)

		fullResponse := ""
		assistantPrefixShown := false
		var toolCalls []openai.ToolCall

		for event := range events {
			switch event.Type {
			case chat.StreamEventContent:
				content := event.Content
				fullResponse += content
				if !assistantPrefixShown {
					ui.app.QueueUpdateDraw(func() {
						currentText := ui.chatView.GetText(false)
						newText := currentText + fmt.Sprintf("\n[%s]Assistant:[-] ", ui.theme.ChatAssistantColor)
						ui.chatView.SetText(newText)
					})
					assistantPrefixShown = true
				}
				ui.app.QueueUpdateDraw(func() {
					currentText := ui.chatView.GetText(false)
					newText := currentText + content
					ui.chatView.SetText(newText)
					ui.chatView.ScrollToEnd()
				})
			case chat.StreamEventToolCall:
				if event.ToolCall != nil {
					toolCalls = append(toolCalls, *event.ToolCall)
				}
			case chat.StreamEventError:
				err := event.Err
				if err == context.Canceled {
					ui.app.QueueUpdateDraw(func() {
						currentText := ui.chatView.GetText(false)
						newText := currentText + fmt.Sprintf("\n[%s]Request cancelled[-]", ui.theme.ChatErrorColor)
						ui.chatView.SetText(newText)
						ui.chatView.ScrollToEnd()
					})
				} else {
					ui.app.QueueUpdateDraw(func() {
						currentText := ui.chatView.GetText(false)
						newText := currentText + fmt.Sprintf("\n[%s]Error: %v[-]", ui.theme.ChatErrorColor, err)
						ui.chatView.SetText(newText)
						ui.chatView.ScrollToEnd()
					})
				}
				return
			}
		}

		if len(toolCalls) == 0 {
			if ui.debugMode && fullResponse != "" {
				ui.app.QueueUpdateDraw(func() {
					currentText := ui.chatView.GetText(false)
					currentText += fmt.Sprintf("\n[%s]DEBUG - Full Response:[-]\n%s\n", ui.theme.ChatErrorColor, fullResponse)
					ui.chatView.SetText(currentText)
					ui.chatView.ScrollToEnd()
				})
			}
			return
		}

		for _, tc := range toolCalls {
			toolName := tc.Function.Name
			if toolName == "" {
				toolName = "unknown_tool"
			}
			if toolName == "unknown_tool" {
				result := &tools.ToolResult{
					Function: toolName,
					Result:   "Tool call rejected: missing or invalid function name",
					Error:    fmt.Errorf("tool call missing function name"),
				}
				ui.session.AddToolResultMessage(tc, result)
				ui.app.QueueUpdateDraw(func() {
					currentText := ui.chatView.GetText(false)
					currentText += fmt.Sprintf("\n[%s]Tool request rejected:[-] missing function name", ui.theme.ChatErrorColor)
					ui.chatView.SetText(currentText)
					ui.chatView.ScrollToEnd()
				})
				continue
			}
			perm := ui.session.ToolRegistry.GetPermission(toolName)
			argsPreview := summarizeToolArgs(tc.Function.Arguments)

			ui.app.QueueUpdateDraw(func() {
				currentText := ui.chatView.GetText(false)
				status := "allowed by policy"
				if !perm.Allowed {
					status = "blocked until you approve"
				} else if perm.RequireConfirmation {
					status = "requires confirmation"
				}
				currentText += fmt.Sprintf("\n[%s]Tool request:[-] %s(%s) — %s", ui.theme.ProgressIndicatorColor, toolName, argsPreview, status)
				ui.chatView.SetText(currentText)
				ui.chatView.ScrollToEnd()
			})

			execOpts := tools.ExecuteOptions{}
			deny := false

			if !perm.Allowed || perm.RequireConfirmation {
				decision := ui.promptForToolPermission(tc, perm, argsPreview)
				switch decision {
				case toolDecisionAllowOnce:
					execOpts.Force = true
				case toolDecisionAlwaysAllow:
					ui.session.ToolRegistry.AllowTool(toolName, false)
				default:
					deny = true
				}
			}

			var result *tools.ToolResult
			if deny {
				result = &tools.ToolResult{
					Function: toolName,
					Result:   "Execution denied by user",
					Error:    fmt.Errorf("tool execution denied by user"),
				}
			} else {
				result = ui.session.ToolRegistry.ExecuteOpenAIToolCallWithOptions(tc, execOpts)
			}

			ui.session.AddToolResultMessage(tc, result)

			toolCallInfo := ui.session.FormatToolCallDisplay(tc, result)
			ui.app.QueueUpdateDraw(func() {
				currentText := ui.chatView.GetText(false)
				currentText += fmt.Sprintf("\n[%s]%s[-]", ui.theme.ProgressIndicatorColor, toolCallInfo)
				if ui.debugMode {
					currentText += fmt.Sprintf("\n[%s]DEBUG - Tool Args:[-]\n%s\n", ui.theme.ChatErrorColor, tc.Function.Arguments)
				}
				ui.chatView.SetText(currentText)
				ui.chatView.ScrollToEnd()
			})
		}

		includeUserMessage = false
		prompt = ""
	}
}

func summarizeToolArgs(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "(no arguments)"
	}
	const limit = 400
	if len(trimmed) > limit {
		return trimmed[:limit] + "..."
	}
	return trimmed
}
