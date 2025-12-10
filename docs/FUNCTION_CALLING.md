# Function calling in Promptline

Promptline exposes its tool registry to the model using the OpenAI tool-calling interface. The system prompt reminds the model to send valid JSON in `function.arguments` and to return tool results in TOON (Token-Oriented Object Notation).

## Flow

1. Promptline advertises available tools from `internal/tools` in the chat request.
2. The model returns `tool_calls` with a function name and JSON arguments.
3. Promptline checks the tool allowlist/confirmation policy, prompts the user for consent when required, and either executes the tool or reports a denial. Tool messages always use TOON-encoded payloads (`{result: string, error: string}`).
4. The model receives the tool output (or denial) and continues the conversation with the new context.

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

Default policy: `get_current_datetime`, `read_file`, and `ls` are allowed; `write_file` and `execute_shell_command` are blocked and require explicit approval via the TUI consent prompt. Override this behavior with the `tools` block in `config.json`.

To add your own tools, register them in `internal/tools/builtin.go` or build a custom registry in `internal/tools`. See `docs/ADDING_TOOLS.md` for a step-by-step guide.
