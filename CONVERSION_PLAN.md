# Promptline: tview TUI to Streaming Console Architecture Conversion Plan

**Epic:** `batchat-3x8` - Convert promptline from tview TUI to streaming console architecture

## Executive Summary

This document outlines the comprehensive plan to convert promptline from a full-screen tview TUI application to a modern streaming console interface. The new architecture will provide better usability, maintainability, and portability while preserving all core functionality.

### Current Architecture
- **Framework:** tview (full terminal takeover)
- **Components:** TextView, TextArea, Table, Flex layouts, Pages, Modal dialogs
- **Interaction:** Keyboard navigation, Ctrl+Enter submission, modal overlays
- **State Management:** Reactive UI redraws via QueueUpdateDraw()

### Target Architecture
- **Framework:** pterm (streaming output), readline (input), zerolog (logging)
- **Components:** Direct stdout streaming, readline prompts, tabwriter tables, AreaPrinter for fixed regions
- **Interaction:** Standard CLI (Enter submission), readline history, simple prompts
- **State Management:** Event-driven with structured logging

### Key Benefits
✅ **Full scrollback history** - Users can scroll terminal to see complete session  
✅ **SSH-friendly** - Works seamlessly over remote connections  
✅ **Simpler UX** - Standard CLI patterns (Enter to submit)  
✅ **Better logging** - Structured event history via zerolog  
✅ **Lower overhead** - No full-screen redraws  
✅ **Unix composability** - Can pipe output, redirect logs  

## Analysis: Current tview Components

### Core UI Elements (internal/ui/ui.go)

| tview Component | Purpose | Lines | Target Replacement |
|----------------|---------|-------|-------------------|
| `app *tview.Application` | Main event loop | 70 | readline event loop |
| `chatView *tview.TextView` | Chat message display | 204-208 | pterm.Println() streaming |
| `inputArea *tview.TextArea` | Multi-line input | 230-239 | readline.Readline() |
| `header *tview.TextView` | App title/shortcuts | 199-202 | pterm.DefaultHeader |
| `progressIndicator *tview.TextView` | Processing animation | 210-213 | pterm.DefaultSpinner |
| `separator *tview.TextView` | Visual separator | 215-228 | Simple pterm separator |
| `permissionsTable *tview.Table` | Tool permissions UI | 262-266 | tabwriter + pterm styling |
| `flex *tview.Flex` | Vertical layout | 241-247 | Sequential output |
| `pages *tview.Pages` | Modal management | 255-256 | Direct prompts |

### Event Handlers

| Handler | Purpose | Target Approach |
|---------|---------|----------------|
| `setupInputHandlers()` | Ctrl+Enter, Ctrl+C, Ctrl+Q, history nav | readline with signal handling |
| `runProgressIndicator()` | Animated spinner goroutine | pterm spinner.Start()/Stop() |
| `runElasticHeight()` | Dynamic input area sizing | Remove (not needed) |
| `showModalPrompt()` | Sync confirmation dialogs | Print + readline Y/N |
| `promptForToolPermission()` | Tool execution consent | Print tool info + readline choice |

### Data Flow (internal/chat/session.go)

| Current Pattern | New Pattern |
|----------------|-------------|
| Stream → QueueUpdateDraw → TextView.SetText | Stream → pterm.Print → stdout |
| Tool call → Modal → Response | Tool call → Print + Readline → Response |
| Error → TextView append with color tags | Error → pterm.Error.Println() |
| History → TextView buffer | History → Terminal scrollback + zerolog |

## Conversion Tasks (Priority Order)

### Phase 1: Foundation (Tasks 1-2)
**Goal:** Set up new dependencies and document mapping

#### batchat-3x8.1: Analysis: Map tview components to streaming architecture
- **Priority:** P1
- **Type:** task
- **Description:** Document all tview components and their streaming equivalents
- **Deliverables:**
  - Component mapping table (see above)
  - Event handler conversion plan
  - Data flow diagrams (before/after)

#### batchat-3x8.2: Add new dependencies for streaming architecture
- **Priority:** P1
- **Type:** task
- **Dependencies to add:**
  - `github.com/pterm/pterm` - CLI framework, colors, AreaPrinter
  - `github.com/fatih/color` - Color/text styling
  - `github.com/rs/zerolog` - Structured logging
- **Already available:**
  - `github.com/chzyer/readline` (already in go.mod)
  - `text/tabwriter` (stdlib)
- **Action:** Run `go get` for new deps, update go.mod/go.sum

### Phase 2: Core Infrastructure (Tasks 10-11)
**Goal:** Add logging and theme support

#### batchat-3x8.10: Add structured logging with zerolog
- **Priority:** P1
- **Type:** task
- **Implementation:**
  - Initialize logger in main: `zerolog.New(os.Stderr).With().Timestamp().Logger()`
  - Log events: user input, API requests, tool calls, errors
  - Structured fields: `timestamp`, `user_input`, `model_response`, `tool_name`, `tool_args`, `tool_result`
  - Optional `--log-file` flag for persistent logs
  - Debug flag `-d` sets log level to debug

#### batchat-3x8.11: Convert theme system to pterm/color styles
- **Priority:** P1
- **Type:** task
- **Options:**
  1. **Simple:** Use pterm defaults (Info/Success/Error/Warning colors)
  2. **Full:** Map theme.json colors to pterm/fatih/color equivalents
- **Mapping:**
  - `HeaderTextColor` → `pterm.FgCyan`
  - `ChatUserColor` → `color.FgBlue`
  - `ChatAssistantColor` → `color.FgGreen`
  - `ChatErrorColor` → `color.FgRed`
  - `ProgressIndicatorColor` → `pterm.FgYellow`

### Phase 3: Main Loop Refactor (Task 3)
**Goal:** Replace tview event loop with readline

#### batchat-3x8.3: Refactor main TUI loop to streaming event loop
- **Priority:** P1
- **Type:** task
- **Current (cmd/promptline/main.go:58-79):**
```go
func runTUIMode() {
    cfg, _ := config.LoadConfig("config.json")
    tuiTheme, _ := theme.LoadTheme("theme.json")
    session := chat.NewSession(cfg)
    defer session.Close()
    
    uiApp := promptui.New(session, tuiTheme)
    uiApp.Run(context.Background())  // Blocks here
}
```

- **Target:**
```go
func runTUIMode() {
    cfg, _ := config.LoadConfig("config.json")
    log := zerolog.New(os.Stderr).With().Timestamp().Logger()
    
    rl, _ := readline.New("> ")
    defer rl.Close()
    
    session := chat.NewSession(cfg)
    defer session.Close()
    
    pterm.DefaultHeader.Println("Promptline - AI Chat")
    pterm.Info.Println("Type /help for commands, Ctrl+C to quit")
    
    for {
        line, err := rl.Readline()
        if err != nil {
            break  // EOF or interrupt
        }
        
        if handleCommand(line, session, log) {
            continue
        }
        
        handleConversation(line, session, log)
    }
}
```

### Phase 4: Output Conversion (Tasks 4-5)
**Goal:** Replace TextView with streaming output

#### batchat-3x8.4: Convert chat display from TextView to streaming output
- **Priority:** P1
- **Type:** task
- **Current:** 808 lines in ui.go, heavy use of QueueUpdateDraw()
- **Changes:**
  - User messages: `pterm.Info.Printf("[User] %s\n", text)`
  - Assistant prefix: `pterm.Success.Print("[Assistant] ")`
  - Streaming content: `fmt.Print(chunk)` (raw, no newline)
  - Tool calls: `pterm.Warning.Printf("[Tool] %s(%s)\n", name, args)`
  - Errors: `pterm.Error.Printf("[Error] %s\n", err)`
- **Remove:**
  - All `ui.app.QueueUpdateDraw()` calls
  - TextView state management
  - ScrollToEnd() calls

#### batchat-3x8.5: Convert progress indicator to pterm spinner
- **Priority:** P1
- **Type:** task
- **Current:** Goroutine with ticker updating TextView
- **Target:**
```go
spinner, _ := pterm.DefaultSpinner.Start("Processing...")
// ... do work ...
spinner.Success("Done!")
// or spinner.Fail("Error!")
```

### Phase 5: Input Conversion (Task 6)
**Goal:** Simplify input handling

#### batchat-3x8.6: Replace TextArea input with readline prompts
- **Priority:** P1
- **Type:** task
- **Changes:**
  - Replace `Ctrl+Enter` with `Enter` (standard CLI)
  - Remove elastic height logic (lines 127-155)
  - History navigation built into readline (Ctrl+Up/Down already works)
  - Multi-line: Can add later if needed via readline.NewEx config

### Phase 6: Interactive Elements (Tasks 7-8)
**Goal:** Convert complex UI to simple prompts

#### batchat-3x8.7: Convert permissions panel to interactive menu
- **Priority:** P1
- **Type:** task
- **Approach 1: Table display + prompts**
```go
// /permissions command
func showPermissions(session *chat.Session) {
    w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
    fmt.Fprintln(w, "Tool\tAllowed\tConfirm")
    fmt.Fprintln(w, "----\t-------\t-------")
    for _, name := range session.ToolRegistry.GetToolNames() {
        perm := session.ToolRegistry.GetPermission(name)
        fmt.Fprintf(w, "%s\t%v\t%v\n", name, perm.Allowed, perm.RequireConfirmation)
    }
    w.Flush()
    
    // Optional: interactive toggle
    fmt.Print("Toggle tool (or 'done'): ")
    // ... read + update ...
}
```

- **Approach 2: pterm interactive select**
```go
options := []string{"Allow all", "Block all", "Toggle specific tool", "Done"}
result, _ := pterm.DefaultInteractiveSelect.
    WithOptions(options).
    Show()
```

#### batchat-3x8.8: Replace modals with readline prompts
- **Priority:** P1
- **Type:** task
- **Quit confirmation:**
```go
pterm.Warning.Println("Quit Promptline? (y/N)")
answer, _ := rl.Readline()
if strings.ToLower(answer) == "y" {
    return
}
```

- **Tool permission:**
```go
pterm.Warning.Printf("Tool '%s' wants to execute:\n", toolName)
pterm.Info.Printf("  Arguments: %s\n", argsPreview)
pterm.DefaultBulletList.WithItems([]pterm.BulletListItem{
    {Level: 0, Text: "1. Allow once"},
    {Level: 0, Text: "2. Always allow"},
    {Level: 0, Text: "3. Deny"},
}).Render()

choice, _ := rl.Readline()
switch choice {
case "1": return allowOnce
case "2": return alwaysAllow
default: return deny
}
```

### Phase 7: Commands Refactor (Task 9)
**Goal:** Update slash commands for streaming

#### batchat-3x8.9: Refactor slash commands for streaming output
- **Priority:** P1
- **Type:** task
- **Current signature:**
```go
type Handler func(session *chat.Session, chatView *tview.TextView, 
                  tuiTheme *theme.Theme, app *tview.Application) bool
```

- **New signature:**
```go
type Handler func(session *chat.Session, log zerolog.Logger) error
```

- **Command updates:**
  - `/help` - Print with pterm.DefaultTable or styled list
  - `/clear` - session.ClearHistory() + `pterm.Success.Println("History cleared")`
  - `/history` - Loop messages, print with pterm styling
  - `/debug` - Toggle flag, print system prompt if enabled
  - `/permissions` - Call showPermissions() from task 7
  - `/quit` - Return special error or set quit flag

### Phase 8: Cleanup (Task 13)
**Goal:** Remove deprecated code

#### batchat-3x8.13: Remove internal/ui package
- **Priority:** P1
- **Type:** task
- **Files to remove:**
  - `internal/ui/ui.go` (808 lines)
  - `internal/ui/history.go` (if not moved to session)
  - `internal/ui/ui_test.go`
- **Move if needed:**
  - History logic to internal/chat/session.go (or rely on readline)
- **Verify:**
  - No imports of `promptline/internal/ui`
  - No tview imports anywhere
  - `go build` succeeds

### Phase 9: Testing & Documentation (Tasks 14-15)
**Goal:** Validate and document

#### batchat-3x8.14: Test streaming console implementation
- **Priority:** P1
- **Type:** task
- **Manual tests:**
  - ✓ Interactive session with multiple turns
  - ✓ All slash commands (/help, /clear, /history, /debug, /permissions, /quit)
  - ✓ Tool permission flow (allow/deny/always)
  - ✓ Ctrl+C cancellation during streaming
  - ✓ Terminal scrollback (scroll up to see history)
  - ✓ Readline history (Ctrl+Up/Down between sessions)
- **Cross-environment:**
  - ✓ Local terminal (various: iTerm2, Alacritty, GNOME Terminal)
  - ✓ Over SSH
  - ✓ Tmux/Screen compatibility
- **Regression:**
  - ✓ Batch mode (`echo "test" | ./promptline -`)
  - ✓ Tool execution still works
  - ✓ Streaming responses without errors
  - ✓ Signal handling (SIGINT, SIGTERM)

#### batchat-3x8.15: Update documentation for streaming architecture
- **Priority:** P1
- **Type:** task
- **Files to update:**
  - `README.md`
    - Change "TUI" → "Interactive Console"
    - Update controls: "Enter" instead of "Ctrl+Enter"
    - Add "Scroll up in terminal for history"
    - Update feature list (mention scrollback, SSH-friendly)
  - `QUICKSTART.md`
    - Update description from TUI to console
    - Update keyboard shortcuts table
    - Add scrollback note
  - New: `MIGRATION.md`
    - Document behavioral changes
    - Old: Ctrl+Enter to send → New: Enter
    - Old: Fixed window UI → New: Scrolling terminal
    - Advantages: scrollback, logging, SSH
  - Optional: Update screenshot/demo GIF

### Optional Enhancement (Task 12)
**Goal:** Add persistent status display

#### batchat-3x8.12: Optional: Add status area with AreaPrinter
- **Priority:** P2 (nice-to-have)
- **Type:** task
- **Use case:** Show live info at top/bottom of terminal
- **Example:**
```go
statusArea, _ := pterm.DefaultArea.Start()

go func() {
    ticker := time.NewTicker(time.Second)
    for range ticker.C {
        status := fmt.Sprintf("Model: %s | Status: %s | Time: %s",
            cfg.Model, 
            processingStatus,
            time.Now().Format("15:04:05"))
        statusArea.Update(status)
    }
}()
```
- **Decision:** Add post-MVP if users request it. Spinner may suffice.

## Implementation Order (Recommended)

### Week 1: Foundation
1. **Day 1-2:** Task 1 (Analysis) - Document everything
2. **Day 3:** Task 2 (Dependencies) - Add pterm, color, zerolog
3. **Day 4-5:** Task 10 (Logging) - Integrate zerolog throughout

### Week 2: Core Conversion
4. **Day 1-2:** Task 11 (Theme) - Convert or simplify theme system
5. **Day 3:** Task 3 (Main Loop) - Refactor runTUIMode to readline loop
6. **Day 4-5:** Task 4 (Chat Display) - Convert streaming output

### Week 3: Input & Interaction
7. **Day 1:** Task 5 (Progress) - Add spinner
8. **Day 2:** Task 6 (Input) - Readline prompts
9. **Day 3:** Task 9 (Commands) - Refactor command handlers
10. **Day 4-5:** Tasks 7-8 (Permissions & Modals) - Interactive prompts

### Week 4: Finalization
11. **Day 1:** Task 13 (Cleanup) - Remove internal/ui
12. **Day 2-3:** Task 14 (Testing) - Comprehensive validation
13. **Day 4-5:** Task 15 (Documentation) - Update all docs

**Total estimate:** 3-4 weeks for complete conversion

## Risk Analysis

### High Risk Areas
1. **Tool permission flow** - Complex modal interaction → Must preserve UX
   - Mitigation: Add clear numbered menu prompts
2. **Streaming interruption** - Ctrl+C handling during stream
   - Mitigation: Test signal handling thoroughly
3. **Multi-line input** - Users may need code blocks
   - Mitigation: Add later if requested (readline.NewEx supports it)

### Low Risk Areas
1. **Batch mode** - Already separate code path, won't change
2. **Chat session logic** - No changes to internal/chat/session.go
3. **Tool execution** - Tools themselves unchanged
4. **Configuration** - config.json handling unchanged

## Success Criteria

### Must Have (MVP)
- ✅ All existing features work (chat, tools, commands)
- ✅ Streaming responses display correctly
- ✅ Tool permissions can be managed
- ✅ History accessible via terminal scrollback
- ✅ Batch mode unchanged (regression-free)

### Should Have
- ✅ Structured logging with zerolog
- ✅ Clean quit/cancel flow
- ✅ Better SSH experience than tview
- ✅ Documentation updated

### Nice to Have (Future)
- ⭕ Status area with AreaPrinter (Task 12)
- ⭕ Multi-line input mode
- ⭕ Command auto-completion
- ⭕ Color scheme customization

## Component Mapping Reference

### Input/Output Pattern Changes

**Before (tview):**
```go
// Get input
text := ui.inputArea.GetText()

// Show output
ui.app.QueueUpdateDraw(func() {
    currentText := ui.chatView.GetText(false)
    newText := currentText + fmt.Sprintf("[%s]Message[-]", color)
    ui.chatView.SetText(newText)
    ui.chatView.ScrollToEnd()
})
```

**After (streaming):**
```go
// Get input
line, _ := rl.Readline()

// Show output
pterm.Info.Println("Message")
// Automatically scrolls, no manual management
```

### Progress Indication

**Before (tview):**
```go
go func() {
    ticker := time.NewTicker(200 * time.Millisecond)
    for range ticker.C {
        if processing {
            ui.app.QueueUpdateDraw(func() {
                ui.progressIndicator.SetText(spinnerChar)
            })
        }
    }
}()
```

**After (pterm):**
```go
spinner, _ := pterm.DefaultSpinner.Start("Processing...")
// ... work happens ...
spinner.Success("Done!")
```

### Modal Dialogs

**Before (tview):**
```go
modal := tview.NewModal().
    SetText("Confirm?").
    AddButtons([]string{"Yes", "No"}).
    SetDoneFunc(func(buttonIndex int, _ string) {
        // Handle choice
    })
ui.pages.AddPage("modal", modal, true, true)
```

**After (readline):**
```go
pterm.Warning.Println("Confirm? (y/N)")
answer, _ := rl.Readline()
if strings.ToLower(answer) == "y" {
    // Handle yes
}
```

## Dependencies Added

```bash
go get github.com/pterm/pterm
go get github.com/fatih/color  
go get github.com/rs/zerolog
# Already have:
# - github.com/chzyer/readline
# - text/tabwriter (stdlib)
```

## Dependencies Removed

```bash
# After conversion complete:
go mod tidy
# Will remove:
# - github.com/rivo/tview
# - github.com/gdamore/tcell/v2
# - Related tview dependencies
```

## File Size Impact

| File | Before | After | Change |
|------|--------|-------|--------|
| internal/ui/ui.go | 808 lines | **DELETED** | -808 |
| internal/ui/history.go | ~50 lines | **DELETED** | -50 |
| cmd/promptline/main.go | 79 lines | ~150 lines | +71 |
| internal/commands/commands.go | 182 lines | ~120 lines | -62 |
| **Total** | **~1119 lines** | **~270 lines** | **-849 lines (-76%)** |

**Net result:** Simpler, more maintainable codebase with fewer lines.

## Next Steps

1. **Review this plan** - Confirm approach with team/stakeholders
2. **Start Task 1** - Complete analysis and get sign-off
3. **Set up branch** - Create `feature/streaming-console` branch
4. **Begin Phase 1** - Add dependencies and logging infrastructure
5. **Iterate** - Complete tasks in order, testing after each phase

## Questions & Answers

**Q: Will this break existing workflows?**  
A: Batch mode is unchanged. Interactive users will need to adapt to Enter (not Ctrl+Enter), but this is more standard.

**Q: Can we still see history?**  
A: Yes! Terminal scrollback provides unlimited history. Plus zerolog logs everything to file.

**Q: What about mouse scrolling?**  
A: Terminal native scrolling works (scroll wheel, trackpad). Better than tview which required special handling.

**Q: Will this work on Windows?**  
A: readline and pterm both support Windows. Testing recommended but should work.

**Q: Can we add features back later?**  
A: Yes! Multi-line input, status areas, etc. can be added incrementally. This is MVP.

---

**Tracked in:** `bd show batchat-3x8`  
**Tasks:** 15 total (14 P1, 1 P2)  
**Status:** Planning phase - ready to begin implementation  
**Owner:** TBD  
**Target:** 3-4 weeks to completion
