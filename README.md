# Promptline

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

For focused protocol, race, fuzz, coverage, benchmark, and portability
commands, use the repository Make targets in [docs/TESTING.md](docs/TESTING.md).
`make test-all` is the complete local release gate.

The integration suite runs against a local mock Codex app-server and exercises
the real Promptline toolbox MCP server with basic embedded u-root tools. It does
not require network access, Codex credentials, or a live account. Tests select
their generated fixture explicitly with `--mock-codex PATH`; this flag is not
for normal operation.

## Codex plugin

This repository is also a Codex plugin marketplace. The `sysadmin` plugin
bundles Debian administration, Bash, security ownership, and threat-modeling
skills plus a stdio MCP definition for the Go/u-root toolbox. The MCP definition invokes exactly `promptline mcp-server`; installing
the plugin does not start Promptline's interactive thread host, run a shell
installer, or download an executable.

Run the installer from a checkout to build `promptline`, add this repository as
the `promptline` marketplace, and install and enable `sysadmin`:

```bash
./install.sh
```

The binary is installed under `${PROMPTLINE_BIN_DIR:-$HOME/.local/bin}`. Set
`PROMPTLINE_BIN_DIR` to another absolute directory when needed. It must be on
`PATH` when Codex starts so the bundled MCP configuration can invoke
`promptline mcp-server`.

After the marketplace files are on the repository's default branch, Codex can
track the GitHub source instead of a checkout:

```bash
codex plugin marketplace add dyne/promptline
codex plugin add sysadmin@promptline
```

Start a new Codex thread after installation so Codex discovers the bundled
skill and `promptline-toolbox` MCP server. The server inherits Codex's working
directory and uses it as its only filesystem capability root. Run Codex from
the directory that should be visible to the toolbox; do not launch it from `/`
merely to broaden access.

### Embedded skills and compatibility

The reviewed on-disk sources live under [`plugins/promptline/skills`](plugins/promptline/skills). Each build embeds their public files and one shared `LICENSE.txt`, so these commands work from a copied executable outside a checkout and do not create an instance:

```bash
./promptline list-skills
./promptline list-skill-files debian-sysadmin
./promptline materialize-skill /tmp/promptline-skills
```

The standalone `mcp-server` exposes the same public files as ordinary `skill://` MCP resources through `resources/list` and `resources/read`; the shared license uses `skill-bundle://promptline/LICENSE.txt`. Tests and Debian's validation-only scripts are excluded. Operational scripts in other skills are read-only resources and require explicit materialization before execution. The full source, safe export behavior, plugin discovery, and executable-only fallback are documented in [the embedded-skills guide](plugins/promptline/skills/README.md).

The standalone MCP mode applies Promptline's read-only toolbox policy. Mutating
toolbox definitions remain visible for schema compatibility but fail closed
unless a future explicitly approved integration supplies an authorization
path. Remove the plugin with `codex plugin remove sysadmin@promptline`; remove the
marketplace separately with `codex plugin marketplace remove promptline` when
it is no longer needed.

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

After a prompt is accepted, Promptline streams the resulting turn and returns
to input only after the turn completes or fails. Its behavioral tests observe
typed lifecycle events and persisted effects rather than terminal wording.

When `--state-root` is omitted, Promptline creates and uses
`~/.promptline/instances` for a regular user and
`/var/lib/promptline/instances` for root. An explicit `--state-root` must be an
absolute path; Promptline creates it when its parent is writable.

Each launch starts a new primary thread by default. Use the `resume` command
with a thread ID, or omit the ID to resume the last saved thread:

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
create an instance, acquire a lock, or open a network listener.
Before showing the first prompt, Promptline requires Codex to report this server
with its core tools. New and resumed
threads receive the same tool schemas in a model-facing `toolbox` namespace and
instruct Codex to prefer it over the built-in shell for supported Unix
operations. Calls to that namespace are routed through app-server's
`mcpServer/tool/call` method to the real MCP server. This explicit bridge avoids
Codex configurations that discover MCP tools but defer them without exposing a
tool-search facility.

The MCP server also publishes all embedded skills as ordinary resources and supplies their names and descriptions in standard MCP server instructions; it does not claim a nonstandard MCP skill-discovery extension. A plugin installation supplies the on-disk skills for independently launched Codex environments. Promptline's own app-server child deliberately disables both user/system skill instructions and Codex-bundled skills. If its MCP client cannot access the embedded resources, that executable-only child cannot activate the authoritative skills; exporting them is a compatibility aid, not a bypass of this isolation.

The embedded developer instructions are maintained as Markdown in
`internal/runtime/init-prompt.md`; edit that file to extend Promptline's initial
runtime guidance.

```bash
./promptline mcp-server --cwd ~/devel/ops
```

Mutating effects and privilege expansion are asked or denied by default.
Prompts are read from the controlling terminal; unavailable or malformed input
declines the request. The journal is append-only JSONL under
`<state-root>/<instance>/audit` and records bounded, redacted operational
metadata. It is not an authorization source.

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
starting a new thread does not correct this protocol-decoding mismatch.

If startup reports `codex authentication required`, run the `CODEX_HOME=... codex
login` command included in that error. Promptline checks `account/read` before
starting or resuming a thread, so missing required credentials fail immediately
rather than producing repeated reconnect messages and a later HTTP 401.

Promptline has no daemon, control socket, WebSocket transport, automatic
restart, automatic tmux integration, second model runtime, internal database,
or search/index service. See [the architecture](docs/ARCHITECTURE.md),
[toolbox documentation](docs/TOOLS.md), and [test guidance](docs/TESTING.md)
for boundaries, portability, and maintenance commands.

## Audit verification

`promptline verify-audit STATE_DIRECTORY` verifies only the retained
`audit/events.jsonl` artifact beneath that state directory and
reports that it is **local-chain-only** evidence. It detects corruption, but a
process able to rewrite both the journal and its local files can construct a
new internally consistent chain. Supplying `--audit-anchor HASH`, where HASH
was exported to a separately trusted system, verifies that external anchor and
detects such a rewrite relative to that anchor. The command is read-only.

## License

Copyright (C) 2025-2026 Dyne.org foundation. Licensed under the GNU Affero
General Public License, version 3 or later.
