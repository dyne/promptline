# Promptline v2

Promptline is a foreground host for one `codex app-server --stdio` child and
one primary Codex thread. It is for Unix administration: run it only with a
narrow working root and the least privilege necessary.

Promptline is not an OpenAI-compatible chat-completions client. Codex owns
authentication and conversation history. Promptline owns the terminal,
instance lock/state, approvals, the embedded toolbox, and audit journal.

## Build

Go 1.24 is required.

```bash
GOCACHE="$PWD/.gocache" /usr/local/go/bin/go build -o promptline ./cmd/promptline
./promptline --version
./promptline --help
```

Run the unit suite and the isolated end-to-end suite with:

```bash
GOCACHE="$PWD/.gocache" /usr/local/go/bin/go test ./...
GOCACHE="$PWD/.gocache" /usr/local/go/bin/go test -tags=integration ./...
```

The integration suite runs against a local mock Codex app-server and exercises
the real Promptline toolbox MCP server with basic embedded u-root tools. It does
not require network access, Codex credentials, or a live account.

Install and authenticate a compatible Codex CLI before starting Promptline.
Promptline verifies the configured binary before it creates or resumes a
thread.

## Operation

Use tmux externally to run independent named instances. Promptline does not
create, manage, or multiplex tmux sessions.

```bash
tmux new-session -s promptline-ops
./promptline --instance ops --cwd ~/devel/ops

# In a separate pane or session:
./promptline --instance docs --cwd ~/devel/docs
```

Each instance has private `0700` state under
`<state-root>/<instance>`, including `codex-home`, its durable primary-thread
record, lock, and audit journal. Stop with `/quit`, EOF, or `SIGTERM`. `Ctrl-C`
interrupts an active turn; it does not create another thread.

When `--state-root` is omitted, Promptline creates and uses
`~/.promptline/instances` for a regular user and
`/var/lib/promptline/instances` for root. An explicit `--state-root` must be an
absolute path; Promptline creates it when its parent is writable.

By default, a later launch resumes the stored primary thread. Use `--new` to
explicitly replace it, or `--resume THREAD_ID` to request a specific thread:

```bash
./promptline --instance ops --cwd ~/devel/ops
./promptline --instance ops --cwd ~/devel/ops --new
```

If the app-server reports that a stored thread cannot be resumed, Promptline
fails closed and tells the operator to use `--new`; it never silently creates
replacement history. After an app-server crash, restart the same command to
perform the stored resume.

## Toolbox and approvals

`promptline toolbox serve` is an internal stdio MCP server used to expose the
embedded Go/u-root toolbox to Codex. It is not a network daemon.

```bash
./promptline toolbox serve --instance ops --cwd ~/devel/ops
```

Mutating effects and privilege expansion are asked or denied by default.
Prompts are read from the controlling terminal; unavailable or malformed input
declines the request. The journal is append-only JSONL under
`<state-root>/<instance>/audit` and records bounded, redacted operational
metadata. It is not an authorization source.

## Migration from v1

## Breaking change

Promptline v2 removes the OpenAI chat-completions client, its `config.json`
schema, batch/TUI/readline interaction path, and local conversation history.
Upgrading requires a compatible Codex CLI and an explicit named instance; there
is no compatibility mode for the former API-key configuration.

| v1 setting or behavior | v2 replacement |
| --- | --- |
| `api_key`, `api_url`, `OPENAI_API_KEY` | Authenticate and configure the Codex CLI; Promptline has no API client configuration. |
| `model` in `config.json` | Optional `--model` when supported by the configured Codex CLI. |
| Local chat and command history | Codex thread history; Promptline persists only a primary-thread ID. |
| Batch/TUI/readline command path | Foreground line-oriented terminal input. |
| Tool policy in `config.json` | Promptline approval policy and per-instance toolbox configuration. |

An optional external Context-mode Codex plugin may be installed and passed
through as Codex configuration. Promptline does not bundle, index, search, or
reimplement it.

## Troubleshooting and non-goals

Use `--codex /absolute/path/to/codex` when the desired binary is not on
`PATH`. An unsupported version, malformed version output, or missing stable
app-server capability is an actionable startup error, not a best-effort mode.

An error containing `cannot unmarshal object into Go struct field
Thread.thread.status of type string` means the Promptline binary expects an
older shape for the Codex app-server's `thread.status` field. It does not refer
to the working directory or Promptline's saved thread state. Upgrade or rebuild
Promptline with support for the installed Codex CLI; deleting instance state or
using `--new` does not correct this protocol-decoding mismatch.

Promptline has no daemon, control socket, WebSocket transport, automatic
restart, automatic tmux integration, second model runtime, internal database,
or search/index service. See [the v2 architecture](docs/ARCHITECTURE_V2.md)
and [toolbox documentation](docs/TOOLS.md) for boundaries and portability.

## License

Copyright (C) 2025-2026 Dyne.org foundation. Licensed under the GNU Affero
General Public License, version 3 or later.
