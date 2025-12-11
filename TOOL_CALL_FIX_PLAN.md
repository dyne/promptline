# Tool Call Fix Plan

## Problem Analysis

### Current Issue
Tool calls are detected and logged but NOT executed. The streaming handler in `handleConversation()` just prints tool call info but doesn't:
1. Execute the tool
2. Add tool result to message history
3. Continue the conversation with tool results

### Error Message
```
An assistant message with "tool_calls" must be followed by tool messages responding to each "tool_call_id"
```

This means the API sent a tool call request, but we never sent back the tool result message.

### Root Cause
In the streaming console refactor, we removed the tool execution logic that was in the old TUI code.

## Current Flow (BROKEN)

```
User Input → StreamResponseWithContext()
  ↓
Assistant response with tool_calls emitted
  ↓
handleConversation() receives StreamEventToolCall
  ↓
Just prints "[Tool Call] function_name" ❌
  ↓
Conversation ends (tool result never sent back)
  ↓
Next API call fails: missing tool response messages
```

## Target Flow (WORKING)

```
User Input → StreamResponseWithContext()
  ↓
Assistant response with tool_calls emitted
  ↓
handleConversation() receives StreamEventToolCall
  ↓
Execute tool with ToolRegistry.ExecuteOpenAIToolCall() ✓
  ↓
Add result with Session.AddToolResultMessage() ✓
  ↓
Recursively call conversation handler to continue ✓
  ↓
AI receives tool results and generates final response
```

## Implementation Plan

### Phase 1: Fix handleConversation (Interactive Mode)

**File:** `cmd/promptline/main.go`

**Changes:**
1. When `StreamEventToolCall` is received:
   - Execute tool: `session.ToolRegistry.ExecuteOpenAIToolCall(*event.ToolCall)`
   - Add result: `session.AddToolResultMessage(toolCall, result)`
   - Display tool execution to user
   - Continue streaming with empty prompt to get AI's response with tool results

2. Make `handleConversation` recursive or use a loop to handle multiple tool calls

3. Add proper error handling for tool execution failures

**Pseudocode:**
```go
case chat.StreamEventToolCall:
    if event.ToolCall != nil {
        // Show what tool is being called
        colors.ProgressIndicator.Printf("[Tool Call] %s\n", event.ToolCall.Function.Name)
        
        // Execute the tool
        result := session.ToolRegistry.ExecuteOpenAIToolCall(*event.ToolCall)
        
        // Add result to conversation
        session.AddToolResultMessage(*event.ToolCall, result)
        
        // Display result to user
        if result.Error != nil {
            colors.Error.Printf("Tool error: %v\n", result.Error)
        } else {
            fmt.Printf("Tool result: %s\n", truncate(result.Result))
        }
        
        // Continue conversation with tool results
        // Need to stream again WITHOUT adding user message
        continueConversationWithToolResults(session, colors, logger)
    }
```

### Phase 2: Fix Batch Mode

**File:** `cmd/promptline/main.go` - `runBatchMode()`

**Problem:** Batch mode uses `GetResponse()` not streaming. Need to check if it handles tools.

**Investigation needed:**
- Does `GetResponse()` handle tool calls internally?
- Or does it also need the recursive pattern?

**Action:** Check `internal/chat/session.go:174` (GetResponse implementation)

### Phase 3: Add Helper Methods

**File:** `cmd/promptline/main.go`

**New functions:**
1. `executeTool()` - Execute tool and return result
2. `continueWithToolResults()` - Stream follow-up response after tool execution
3. `formatToolDisplay()` - Pretty print tool calls and results

### Phase 4: Add Tests

**File:** `cmd/promptline/main_test.go` (create)

**Test cases:**
1. Test tool execution in interactive mode
2. Test tool execution in batch mode
3. Test multiple sequential tool calls
4. Test tool execution errors
5. Test tool permission denied
6. Test tool requiring confirmation

**File:** `internal/chat/session_test.go` (extend)

**Additional test cases:**
1. Test AddToolResultMessage
2. Test tool call streaming
3. Test recursive conversation with tools

### Phase 5: Integration Testing

**Manual tests:**
1. "What is the current time?" - should call get_current_datetime
2. "List files in current directory" - should call ls
3. "Read the README file" - should call read_file
4. Multiple tool calls in sequence
5. Tool execution with errors

## Implementation Order

1. ✅ Create this plan document
2. ⬜ Investigate GetResponse() for batch mode
3. ⬜ Implement tool execution in handleConversation
4. ⬜ Test interactive mode manually
5. ⬜ Fix batch mode if needed
6. ⬜ Add helper functions
7. ⬜ Write unit tests
8. ⬜ Write integration tests
9. ⬜ Update documentation

## Files to Modify

- `cmd/promptline/main.go` - Fix handleConversation, add helpers
- `internal/chat/session.go` - Verify tool handling
- `cmd/promptline/main_test.go` - New test file
- `internal/chat/session_test.go` - Add tool tests
- `README.md` - Document tool call behavior

## Success Criteria

- ✓ "What is the current time?" returns actual time
- ✓ "List files" shows directory contents  
- ✓ Tool results visible in conversation history
- ✓ Multiple tool calls work in sequence
- ✓ Batch mode maintains tool call capability
- ✓ All tests pass
- ✓ Error handling works correctly
