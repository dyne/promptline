# Promptline architecture

Promptline is a foreground Unix terminal host for exactly one named instance,
one `codex app-server --stdio` child, and one primary Codex thread. A named
instance is the unit of ownership and isolation.

## Process and stream ownership

```
operator / external tmux pane
  └─ promptline --instance NAME (one primary Codex thread)
       └─ codex app-server --stdio (one child process)
```

Each Promptline process owns exactly one named instance, one app-server child,
and one primary thread. Promptline exclusively owns the child's stdin, stdout,
stderr, lifecycle, and reaping. Stdout is JSONL protocol only, stderr remains
separate from terminal rendering, and the terminal belongs to the Promptline
parent. The app-server owns Codex thread and turn history; Promptline stores
only instance-local operational state, including the last primary-thread ID and
Promptline-owned audit metadata.

tmux is intentionally external. Operators create panes and sessions; neither
Promptline nor its child creates, controls, or multiplexes tmux.

## Instance state and policy boundaries

An instance has a private state directory. The default is
`~/.promptline/instances/<name>` for a regular user and
`/var/lib/promptline/instances/<name>` for root. An explicit state root must be
absolute. State directories are private (`0700`), regular state files are
private (`0600`), and a process-lifetime advisory lock prevents two foreground
processes from owning the same instance.

The embedded Unix toolbox is an instance-owned capability. Promptline decides
approval policy and renders approval requests; the app-server client transports
typed requests and replies. When the toolbox is enabled, Promptline registers
its schemas in the model-facing `toolbox` dynamic namespace and routes
`item/tool/call` requests through the app-server's `mcpServer/tool/call` method.
It also requires the thread-scoped MCP inventory to contain the toolbox and its
core tools before entering the foreground prompt. Both new and resumed threads
receive the namespace and developer instructions to prefer it for supported
Unix operations. The same toolbox is exposed to other harnesses by the
standalone `mcp-server` command without creating an instance.

The executable also embeds the plugin skill catalog. The standalone MCP server publishes every public catalog file as an ordinary `skill://` resource; it is not a custom skill-discovery protocol. Interactive startup writes an instance-private Codex configuration that invokes the same executable's `mcp-server` command. In contrast, installed plugin files provide normal filesystem skill discovery to independently launched Codex environments.

Promptline starts its owned app-server child with the exact overrides `skills.include_instructions=false` and `skills.bundled.enabled=false`. This prevents user/system instructions and Codex-bundled skills from entering the managed child. Therefore an executable-only child whose MCP client cannot expose the embedded resources has no supported path to activate the Debian skill in that run. Materialization is deliberately limited to a safe export for filesystem-based compatibility/debugging, not an isolation bypass.

Child environments are built from an explicit allow policy, never inherited
wholesale. Values written to diagnostics or audit output pass through central
redaction. Defaults fail closed: invalid instance names, unsafe state paths,
ownership mismatches, unavailable authentication, and unrecognized future
state schemas stop startup.

## Codex protocol compatibility

Promptline validates its checked-in stable app-server protocol fixture and
performs a startup probe of the configured executable before any thread or turn
mutation. The executable is resolved without a shell and its well-formed
reported version is recorded. A version differing from the reference fixture is
tolerated; actual process and protocol errors remain startup failures. Dynamic
tool support uses the app-server experimental capability only when the instance
toolbox is enabled.

After initialization, Promptline reads the app-server account state before any
thread mutation. When the selected provider requires OpenAI authentication and
the instance-private `CODEX_HOME` has no account, startup fails with the exact
instance-scoped `codex login` command. Test fixtures are selected explicitly
with `--mock-codex PATH`; normal operation never silently falls back to a mock.

## Package ownership

`cmd/promptline` parses canonical command forms and owns process signals.
`internal/application` composes the instance, app-server process, toolbox,
governance journal, runtime, and ordered cleanup. `internal/runtime` owns
session and turn orchestration, semantic events, dynamic-tool routing, and the
terminal adapter at the edge. `internal/appserver` owns bounded JSON-RPC
transport and child lifecycle; `internal/mcp` owns the standalone server and
dynamic definitions.

`internal/instance`, `internal/paths`, and `internal/governance` enforce
durable state, filesystem confinement, approvals, and audit records.
`internal/tools` owns the authoritative tool catalog, policy classification,
schemas, and capability implementations. The test layers and targets are in
[TESTING.md](TESTING.md).

## Deliberate non-goals

Promptline has no daemon, control socket, internal session multiplexing, second
app-server child, production WebSocket transport, second model runtime, hidden
background service, automatic restart loop, or automatic tmux management. It
also has no internal database, FTS, cross-instance search, embeddings, or
generalized repository/SQLite layer.
