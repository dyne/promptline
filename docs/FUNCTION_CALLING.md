# Function calling in Promptline

Promptline exposes its tool registry to the model using the OpenAI tool-calling interface. The system prompt reminds the model to send valid JSON in `function.arguments` and to return tool results in TOON (Token-Oriented Object Notation).

## Flow

1. Promptline advertises available tools from `internal/tools` in the chat request.
2. The model returns `tool_calls` with a function name and JSON arguments.
3. Promptline executes the tool and appends a tool message containing a TOON-encoded payload (`{result: string, error: string}`).
4. The model receives the tool output and continues the conversation with the new context.

## Example call

Assistant delta:
```json
{
  "tool_calls": [
    {
      "id": "call_1",
      "type": "function",
      "function": {
        "name": "ls",
        "arguments": "{\"path\":\".\"}"
      }
    }
  ]
}
```

Promptline executes the `ls` tool and posts a tool message with `name: "ls"` and TOON content describing the result or error. The assistant then resumes with the next message.

## Built-in tools

- `get_current_datetime` — return the current date/time in RFC3339 format.
- `ls` — list directory contents (supports `path`, `recursive`, `show_hidden`).
- `read_file` — read a file from disk.
- `write_file` — write content to a file.
- `execute_shell_command` — run a shell command and return stdout/stderr.

To add your own tools, register them in `internal/tools/builtin.go` or build a custom registry in `internal/tools`. See `docs/ADDING_TOOLS.md` for a step-by-step guide.
