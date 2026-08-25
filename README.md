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

`--version` reports the Promptline build, installed Codex CLI, vendored u-root,
and Go runtime versions. Use `--codex /path/to/codex --version` to inspect a
non-default Codex executable. If Codex is unavailable, the report says so while
still printing the vendored and runtime component versions.

Run the unit suite and the isolated end-to-end suite with:

```bash
GOCACHE="$PWD/.gocache" /usr/local/go/bin/go test ./...
GOCACHE="$PWD/.gocache" /usr/local/go/bin/go test -tags=integration ./...
```

The integration suite runs against a local mock Codex app-server and exercises
the real Promptline toolbox MCP server with basic embedded u-root tools. It does
not require network access, Codex credentials, or a live account. Tests select
their generated fixture explicitly with `--mock-codex PATH`; this flag is not
for normal operation.

Install and authenticate a compatible Codex CLI before starting Promptline.
Promptline verifies the configured binary before it creates or resumes a
thread.

Each Promptline instance has a private `CODEX_HOME`, so authentication in the
default Codex home is not automatically shared. On an unauthenticated startup,
Promptline exits before creating a thread or showing `>` and prints the exact
instance-specific command to run, for example:

```bash
CODEX_HOME="$HOME/.promptline/instances/default/codex-home" codex login
```

Restart Promptline after that login succeeds.

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

After a prompt is accepted, Promptline prints `[ working ]`, streams agent text
as it arrives, reports tool lifecycle progress, and surfaces app-server turn
errors before returning to the `>` prompt.

When `--state-root` is omitted, Promptline creates and uses
`~/.promptline/instances` for a regular user and
`/var/lib/promptline/instances` for root. An explicit `--state-root` must be an
absolute path; Promptline creates it when its parent is writable.

Each launch starts a new primary thread by default. Use the Codex-style
`resume` command with a thread ID, or omit the ID to resume the last saved
thread. The older `--new` and `--resume THREAD_ID` flags remain aliases:

```bash
./promptline --instance ops --cwd ~/devel/ops
./promptline resume --instance ops --cwd ~/devel/ops
./promptline resume THREAD_ID --instance ops --cwd ~/devel/ops
```

If an explicit resume fails, Promptline never silently creates replacement
history. Quit and EOF shutdown explicitly unsubscribe the selected thread before
the app-server transport closes.

## Toolbox and approvals

`promptline mcp-server` is a standalone stdio MCP server exposing the embedded
Go/u-root toolbox to Codex or another MCP harness. It does not start Codex,
create an instance, acquire a lock, or open a network listener. The
`--mcp-server` flag is equivalent; `toolbox serve` remains a legacy alias.
Before showing the first prompt, Promptline requires Codex to report this server
with its core tools and prints `[ toolbox ready: N tools ]`. New and resumed
threads instruct Codex to prefer these MCP tools over its built-in shell for
supported Unix operations.

The embedded developer instructions are maintained as Markdown in
`internal/runtime/init-prompt.md`; edit that file to extend Promptline's initial
runtime guidance.

```bash
./promptline mcp-server --cwd ~/devel/ops
# Equivalent:
./promptline --mcp-server --cwd ~/devel/ops
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
| `model` in `config.json` | `--model`, defaulting to `gpt-5.6-terra`; the selected model is sent for every turn. |
| Local chat and command history | Codex thread history; Promptline persists only a primary-thread ID. |
| Batch/TUI/readline command path | Foreground line-oriented terminal input. |
| Tool policy in `config.json` | Promptline approval policy and per-instance toolbox configuration. |

An optional external Context-mode Codex plugin may be installed and passed
through as Codex configuration. Promptline does not bundle, index, search, or
reimplement it.

## Troubleshooting and non-goals

Use `--codex /absolute/path/to/codex` when the desired binary is not on
`PATH`. Promptline records the reported Codex CLI version in instance state but
does not reject a well-formed version solely because it differs from the
reference test fixture. Promptline uses `gpt-5.6-terra` unless `--model` selects a
different Codex model; it sends the selection on every turn so resumed threads
also receive the configured model. An unlaunchable executable, malformed
version output, or actual app-server protocol error still stops startup.

An error containing `cannot unmarshal object into Go struct field
Thread.thread.status of type string` means the Promptline binary expects an
older shape for the Codex app-server's `thread.status` field. It does not refer
to the working directory or Promptline's saved thread state. Upgrade or rebuild
Promptline with support for the installed Codex CLI; deleting instance state or
using `--new` does not correct this protocol-decoding mismatch.

If startup reports `codex authentication required`, run the `CODEX_HOME=... codex
login` command included in that error. Promptline checks `account/read` before
starting or resuming a thread, so missing required credentials fail immediately
rather than producing repeated reconnect messages and a later HTTP 401.

Promptline has no daemon, control socket, WebSocket transport, automatic
restart, automatic tmux integration, second model runtime, internal database,
or search/index service. See [the v2 architecture](docs/ARCHITECTURE_V2.md)
and [toolbox documentation](docs/TOOLS.md) for boundaries and portability.

## License

Copyright (C) 2025-2026 Dyne.org foundation. Licensed under the GNU Affero
General Public License, version 3 or later.
