# Promptline

Terminal interface for streaming chat with OpenAI-compatible APIs. Written in Go.

## Build

```bash
make build
./promptline
```

Requires Go 1.22+ and `OPENAI_API_KEY` set.

## Config

`config.json` or env vars:

```json
{
  "api_key": "sk-...",
  "api_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "tools": {
    "allow": ["read_file", "ls"],
    "ask": ["write_file"]
  },
  "tool_limits": {
    "max_file_size_bytes": 10485760,
    "max_directory_depth": 8,
    "max_directory_entries": 2000
  },
  "tool_rate_limits": {
    "default_per_minute": 60,
    "per_tool": {},
    "cooldown_seconds": {}
  },
  "tool_timeouts": {
    "default_seconds": 0,
    "per_tool_seconds": {}
  }
}
```

## Usage

```bash
./promptline                          # interactive
./promptline -d                       # debug mode
echo "query" | ./promptline -         # batch/pipe
```

Commands: `/help` `/clear` `/history` `/debug` `/permissions` `/quit`

Keys: `Ctrl+↑/↓` history, `Ctrl+C` exit

## Tools

AI can call functions to read/write files and perform safe operations. Promptline does not execute system binaries. Permissions in config control allow/ask/deny behavior.

Built-in includes core file and system tools (u-root based). Full list and descriptions in `docs/TOOLS.md`.

Add your own in `internal/tools/builtin.go` or `internal/tools/builtin_uroot.go` - see `docs/`.

## Docs

- `docs/ARCHITECTURE.md` - how it works
- `docs/TOOLS.md` - adding tools, permissions

## License

GNU AGPLv3+ - dyne.org
