# Promptline - TUI AI chat by dyne.org

Promptline is a free and open source terminal UI for AI chat built at dyne.org. It's written in Go, easy to customize, and works with any OpenAI-compatible API whether hosted in the cloud or on-premises.

## Features

- Terminal-native chat: keyboard-first TUI with multiline input, cancel (`Ctrl+C`), quit (`Ctrl+Q`), and history navigation (`Ctrl+↑/↓`).
- OpenAI-compatible: point at any base URL via `api_url`/`OPENAI_API_URL`; set keys with `api_key` or `OPENAI_API_KEY`.
- Tool calling with TOON output: built-in tools (`ls`, `read_file`, `write_file`, `execute_shell_command`, `get_current_datetime`) and structured tool results embedded in the chat.
- Batch mode: run `promptline -` to read one line from stdin and print the assistant response to stdout.
- Theming & persistence: configurable colors in `theme.json`, readline history stored in `.promptline_history`.

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
  "max_tokens": 1500
}
```

- `api_key` / `OPENAI_API_KEY` (required)
- `api_url` / `OPENAI_API_URL` (optional; set for self-hosted or proxy)
- `model`, `temperature`, `max_tokens` (optional tuning)

The app exits early if no API key is provided.

## Usage

- Type in the input area and press `Ctrl+Enter` to send; use `Ctrl+C` to cancel a running request and `Ctrl+Q` to quit.
- Commands: `/help`, `/clear`, `/history`, `/debug`.
- Tool calls: the assistant can call registered tools; results are returned in TOON format inside the chat transcript.

Batch mode:
```bash
echo "Say hello" | ./promptline -
```

## Project Structure

```
promptline/
├── cmd/
│   └── promptline/main.go     # TUI entry point
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
