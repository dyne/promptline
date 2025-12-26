# Architecture

Streaming console for OpenAI-compatible chat APIs.

## Structure

```
cmd/promptline/main.go      entry, console loop, execution
internal/chat/session.go     conversation, streaming, history
internal/tools/tools.go      tool registry, permissions
internal/tools/builtin.go    tool implementations
internal/config/config.go    config loader
```

## Flow

```
User → Session.StreamResponse() → API
         ↓
      Stream Events (content/tool_call/error)
         ↓
      Tool Execution
         ↓
      Recursive Stream (with results)
         ↓
      Console Output
```

## Streaming

`StreamResponseWithContext()` emits via channel:

- `StreamEventContent` - text chunks
- `StreamEventToolCall` - function call with JSON args
- `StreamEventError` - errors

Tool calls execute immediately, inject results into history, continue streaming.

## Permissions

Per tool:
1. **Allow** - execute without asking
2. **Ask** - prompt before executing
3. **Deny** - reject

Default: ask for all tools unless configured otherwise.

## History

Messages in `[]openai.ChatCompletionMessage`:
- `system` - instructions
- `user` - human input
- `assistant` - AI responses/tool calls
- `tool` - execution results

Persists to `.promptline_conversation_history` (JSONL), loads on startup.
