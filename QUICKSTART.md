# Promptline quickstart

Promptline is a free and open source streaming console chat client built at dyne.org. It is written in Go, easy to customize, and works with any OpenAI-compatible API whether hosted in the cloud or on-premises.

## Issue tracking

This project uses [bd (beads)](https://github.com/steveyegge/beads) for all tasks. Run `bd ready --json` to see work items and update them instead of creating markdown TODOs.

## Prerequisites

- Go 1.22+
- An API key for an OpenAI-compatible endpoint (set `OPENAI_API_KEY`; optional `OPENAI_API_URL`)

## Build and run

```bash
git clone <repository-url>
cd promptline
go build -o promptline ./cmd/promptline
./promptline
```

Helpful make targets: `make build`, `make install`, `make test`, `make clean`.

Run in batch mode (reads stdin once, prints reply, exits):
```bash
echo "hello" | ./promptline -
```

## Configuration

Promptline loads `config.json` next to the binary and lets environment variables override values. The app exits early if no API key is available.

```json
{
  "api_key": "your-api-key",
  "api_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "temperature": 0.7,
  "max_tokens": 1500
}
```

- Required: `api_key` or `OPENAI_API_KEY`
- Optional: `api_url` or `OPENAI_API_URL` for self-hosted/proxy endpoints
- Optional: `model`, `temperature`, `max_tokens`

## Using the console

- Press `Enter` to send messages, `Ctrl+C` or `/quit` to exit
- Navigate history with `Ctrl+↑` / `Ctrl+↓`
- Scroll up in your terminal to see full conversation history
- Slash commands: `/help`, `/clear`, `/history`, `/debug`, `/permissions`, `/quit`
- Tool calls are supported; results stream back in real-time

**Command-line flags:**
```bash
./promptline              # Normal mode
./promptline -d           # Debug mode (verbose logging)
./promptline --log-file session.log  # Save logs to file
```

## Project structure

```
promptline/
├── cmd/promptline         # Main console entry point
├── internal/chat          # Session + streaming + tool-call handling
├── internal/config        # Config loader with env overrides
├── internal/theme         # Color scheme for terminal output
├── internal/tools         # Tool registry and implementations
├── docs                   # Developer docs
└── .github/workflows      # CI
```
