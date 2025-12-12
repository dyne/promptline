# Promptline - Interactive AI Chat Console by dyne.org

Promptline is a free and open source streaming console for AI chat built at dyne.org. It's written in Go, easy to customize, and works with any OpenAI-compatible API whether hosted in the cloud or on-premises.

## Features

- **Streaming console**: Real-time AI responses with full terminal scrollback history
- **Simple controls**: Press `Enter` to send, `Ctrl+C` to quit, `Ctrl+↑/↓` for history navigation
- **SSH-friendly**: Works seamlessly over remote connections with no special terminal requirements
- **OpenAI-compatible**: Point at any base URL via `api_url`/`OPENAI_API_URL`; set keys with `api_key` or `OPENAI_API_KEY`
- **Tool calling**: Built-in tools (`ls`, `read_file`, `write_file`, `execute_shell_command`, `get_current_datetime`) with configurable permissions
- **Batch mode**: Run `promptline -` to read one line from stdin and print the assistant response to stdout
- **Logging**: Structured logs with `--log-file` flag and debug mode with `-d`
- **Theming**: Configurable colors in `theme.json`, readline history stored in `.promptline_history`

## Quickstart

Prerequisites: Go 1.22+, an API key for an OpenAI-compatible endpoint.

```bash
git clone <repository-url>
cd promptline
go build -o promptline ./cmd/promptline
./promptline
```

Set your credentials with environment variables:
```bash
export OPENAI_API_KEY=sk-...
export OPENAI_API_URL=https://api.openai.com/v1   # optional for self-hosted endpoints
```

You can also use `make build`, `make install`, and `make test`.

## Configuration

Promptline reads `config.json` next to the binary. Environment variables take precedence over file values.

```json
{
  "api_key": "your-api-key",
  "api_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "temperature": 0.7,
  "max_tokens": 1500,
  "history_file": ".promptline_conversation_history",
  "history_max_messages": 100,
  "tools": {
    "allow": ["get_current_datetime", "read_file", "ls"],
    "require_confirmation": ["write_file", "execute_shell_command"]
  }
}
```

- `api_key` / `OPENAI_API_KEY` (required)
- `api_url` / `OPENAI_API_URL` (optional; set for self-hosted or proxy)
- `model`, `temperature`, `max_tokens` (optional tuning)
- `history_file` - Path to save conversation history (default: `.promptline_conversation_history`)
- `history_max_messages` - Maximum messages to load from history (default: 100)
- `tools.allow` overrides the tool allowlist (defaults to read-only tools if unset)
- `tools.require_confirmation` forces a confirmation prompt per tool; writes/exec require confirmation by default

The app exits early if no API key is provided.

## Usage

**Interactive Mode:**
```bash
./promptline              # Start interactive console
./promptline -d           # Enable debug logging
./promptline --log-file session.log  # Save logs to file
```

**Controls:**
- Type your message at the `❯` prompt and press `Enter` to send
- Assistant responses are prefixed with `⟫`
- Press `Ctrl+C` or type `/quit` to exit
- Press `Ctrl+R` to search conversation history with fuzzy search
- Use `Ctrl+↑/↓` to navigate command history
- Press `Tab` to auto-complete commands
- Scroll up in your terminal to see full conversation history

**Slash Commands:**
- `/help` - Show available commands
- `/clear` - Clear conversation history
- `/history` - Display conversation history
- `/debug` - Toggle debug mode
- `/permissions` - Show tool permissions
- `/quit` - Exit the application

**Tool calls:** The assistant can call registered tools; results are streamed back in real-time.

Batch mode:
```bash
echo "Say hello" | ./promptline -
```

## Tool permissions

- Default allowlist: `get_current_datetime`, `read_file`, and `ls` run without prompting; `write_file` and `execute_shell_command` are blocked until you approve them
- View current permissions with `/permissions` command
- Adjust the allow/confirm lists in `config.json` to set your preferred default policy
- Denied tools surface back to the model as errors

## Project Structure

```
promptline/
├── cmd/
│   └── promptline/main.go     # Console entry point
├── internal/
│   ├── chat/                  # Session + tool-call handling
│   ├── commands/              # Slash commands for the TUI
│   ├── config/                # Config loader and env override
│   └── tools/                 # Tool registry and implementations
├── docs/                      # Developer docs
├── config.json.example        # Sample config
├── theme.json                 # Sample theme
├── Makefile                   # Build/test helpers
└── .github/workflows/ci.yml   # CI pipeline
```

## License

MIT
