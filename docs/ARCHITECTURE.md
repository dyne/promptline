# Promptline v2 architecture

Promptline is a foreground Unix terminal host for exactly one named instance,
one `codex app-server --stdio` child, and one primary Codex thread. It is not
an OpenAI-compatible chat-completions client.

## Ownership

```
operator / external tmux pane
  └─ promptline --instance NAME
       └─ codex app-server --stdio
```

`cmd/promptline/main.go` composes the instance, app-server process, runtime,
governance policy, audit journal, and optional toolbox MCP server. Promptline
exclusively owns the child lifecycle and its stdin/stdout/stderr: app-server
stdout is JSONL protocol, while terminal output remains separate. The runtime
selects one stored or explicitly new primary thread, serializes foreground
turns, and reaps the child during shutdown.

`internal/appserver` owns bounded JSONL RPC, protocol compatibility checks,
thread/turn primitives, events, server requests, and one child process.
`internal/runtime` owns line-oriented terminal interaction, thread selection,
turn state, interrupts, and shutdown. Codex is authoritative for conversation
and turn history; Promptline persists only the last primary-thread ID and its
own operational state.

`internal/governance` evaluates structured effect requests, asks or declines
through the controlling terminal, and writes redacted append-only audit events.
`internal/tools` is a provider-neutral embedded Unix toolbox. It is exposed to
Codex through the instance-scoped stdio MCP server in `internal/mcp`.

## Boundaries and defaults

State is private to `<state-root>/<instance>`: directories use mode `0700`,
files use `0600`, and an advisory lock prevents duplicate foreground owners.
Child environments are allow-listed and redacted. Filesystem toolbox access is
limited to configured roots. Mutations and privilege expansion fail closed when
approval input is unavailable or invalid.

tmux remains external: Promptline neither creates nor manages sessions or
panes. There is no daemon, control socket, WebSocket transport, second model
runtime, local conversation history, internal index, search service, or
automatic restart loop. See [ARCHITECTURE_V2.md](ARCHITECTURE_V2.md) for the
complete compatibility and non-goal contract.

## Upgrade boundary

Version 2 is intentionally incompatible with the former chat-completions
architecture. A running process owns one Codex app-server child and one
primary thread; authentication and conversation history are no longer
Promptline configuration or state.
