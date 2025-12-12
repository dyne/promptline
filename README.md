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
    "require_confirmation": ["write_file", "execute_shell_command"]
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

AI can call functions to read/write files, run commands. Permissions in config control what needs confirmation.

Built-in: `ls` `read_file` `write_file` `execute_shell_command` `get_current_datetime`

Add your own in `internal/tools/builtin.go` - see `docs/`.

## Docs

- `docs/ARCHITECTURE.md` - how it works
- `docs/TOOLS.md` - adding tools, permissions

## License

MIT - dyne.org
