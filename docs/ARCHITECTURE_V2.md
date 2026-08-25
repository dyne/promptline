# Promptline v2 architecture contract

Promptline v2 is a foreground, terminal-oriented host for one Codex app-server
session.  A named instance is the unit of ownership and isolation.

## Process and stream ownership

```
operator / external tmux pane
  └─ promptline --instance NAME (one primary Codex thread)
       └─ codex app-server --stdio (one child process)
```

Each Promptline process owns exactly one named instance, one app-server child,
and one primary thread.  Promptline exclusively owns the child's stdin, stdout,
stderr, lifecycle, and reaping: stdout is JSONL protocol only, stderr is kept
separate from terminal rendering, and the terminal belongs to the Promptline
parent.  The app-server owns Codex thread and turn history; Promptline stores
only instance-local operational state, including the last primary thread ID and
Promptline-owned audit metadata.  A primary thread is started or resumed once
after initialization, then used for foreground turns until process shutdown.

tmux is intentionally external.  Operators create panes and sessions; neither
Promptline nor its child creates, controls, or multiplexes tmux.

## Instance state and policy boundaries

An instance has a private state directory. The default is
`~/.promptline/instances/<name>` for a regular user and
`/var/lib/promptline/instances/<name>` for root. An explicit state root must be
absolute. State directories are private (`0700`), regular state files are
private (`0600`), and a process-lifetime advisory lock prevents two foreground
processes from owning the same instance.

The embedded Unix toolbox is an instance-owned capability.  Promptline decides
approval policy and renders approval requests; the app-server client only
transports typed requests and replies. Promptline requires Codex's thread-scoped
MCP inventory to contain the toolbox and its core tools before entering the
foreground prompt. It also registers those schemas as the model-facing
`toolbox` dynamic namespace and routes `item/tool/call` requests through
app-server's `mcpServer/tool/call` method. This makes inventory discovery and
model visibility separately testable and avoids relying on Codex's automatic
MCP-tool deferral behavior. Both new and resumed threads receive the namespace
and instructions to prefer it for supported Unix operations. Those developer
instructions are embedded from `internal/runtime/init-prompt.md`. The same
toolbox is exposed to other harnesses by the standalone `mcp-server` command
without creating an instance. Promptline appends its own audit events
to an instance-local journal. Future indexing may consume that journal, but
v2 provides no index, search, embeddings, or retrieval service.  Context-mode,
when desired, remains an optional external Codex plugin rather than a
Promptline subsystem.

Child environments are built from an explicit allow policy, never inherited
wholesale.  Values written to diagnostics or audit output pass through central
redaction.  Defaults fail closed: invalid instance names, unsafe state paths,
unknown configuration, ownership mismatches, incompatible Codex binaries, and
unrecognized future state schemas stop startup.

## Codex compatibility

App-server schemas are version-specific. The compatibility evidence for a
Promptline release consists of the user-owned `CODEX_APP_SERVER.md`, a schema
fixture generated with the supported installed Codex CLI, and a startup probe
of the configured executable. The probe must resolve the executable without a
shell and record its well-formed reported version before any thread or turn
mutation. A version differing from the reference fixture is tolerated; actual
process and protocol errors remain startup failures. Stable fields are used by
default, and experimental features are disabled unless an explicit future
configuration enables a tested feature.

After initialization, Promptline reads the app-server account state before any
thread mutation. When the selected provider requires OpenAI authentication and
the instance-private `CODEX_HOME` has no account, startup fails with the exact
instance-scoped `codex login` command. Test fixtures are selected explicitly
with `--mock-codex PATH`; normal operation never silently falls back to a mock.

## Deliberate non-goals

v2 has no Promptline daemon, control socket, internal session multiplexing,
second app-server child, production WebSocket transport, second model/chat
runtime, hidden background service, automatic restart loop, or automatic tmux
management.  It also has no internal Context-mode-like database, FTS,
cross-instance search, embeddings, or generalized repository/SQLite layer.

## Release compatibility

This architecture is the v2 public compatibility boundary. Releases validate
the checked-in stable protocol fixture as reference evidence while runtime
compatibility is determined by the actual app-server exchange, not an exact
Codex CLI version match.
