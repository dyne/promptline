# Promptline

Terminal interface for streaming chat with OpenAI-compatible APIs. Written in Go.

## Build

```bash
make build
./promptline
```

Requires Go 1.22+ and `OPENAI_API_KEY` set or config.json

## Config.json

```json
{
  "api_key": "sk-...",
  "api_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "tools": {
    "allow": ["read_file", "ls"],
    "ask": ["create_file", "edit_file"]
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

Built-in includes core file and system tools (u-root based). Full list and descriptions in [docs/TOOLS](docs/TOOLS.md).

Add your own in [internal/tools/builtin.go](internal/tools/builtin.go) or [internal/tools/builtin_uroot.go](internal/tools/builtin_uroot.go) - see [docs](docs/).

## Docs

- [ARCHITECTURE](docs/ARCHITECTURE.md) - how it works
- [TOOLS](docs/TOOLS.md) - adding tools, permissions

## License

Copyright (C) 2025-2026 Dyne.org foundation

Designed and written by Denis "[Jaromil](https://jaromil.dyne.org)"
Roio.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful, but
WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public
License along with this program.  If not, see
<https://www.gnu.org/licenses/>.

<p align="center">
  <a href="https://dyne.org">
    <img src="https://files.dyne.org/software_by_dyne.png" width="170">
  </a>
</p>
