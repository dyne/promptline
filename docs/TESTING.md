# Testing Promptline

Promptline tests describe observable behavior. They intentionally avoid
asserting terminal presentation so the console can evolve independently of
runtime, protocol, and authority behavior.

## Taxonomy

### Unit contracts

Package tests cover deterministic values, typed errors, state transitions, and
small interface contracts. They should not require a process, clock delay, or
terminal output.

### Protocol contracts

App-server and MCP tests assert JSON-RPC method ordering, request correlation,
typed payloads, replies, and protocol failures. Test doubles record calls in
typed form rather than inspecting rendered status text.

### Subprocess integration

Integration tests exercise the real command and MCP-server process boundaries.
They use temporary instance roots and `CODEX_HOME`, bounded deadlines, and
assert child exit, persisted state, and tool results.

### Filesystem and authority

Instance, state, path-confinement, locking, approval, and audit tests verify
durable records and authorization decisions. Temporary directories are owned by
`t.TempDir` and cleanup is registered with `t.Cleanup` when an additional
resource needs closing.

### Race and stress

Concurrency-sensitive packages run under `go test -race`; focused helpers may
also run repeatedly with `-count` to expose ordering and cleanup defects.

### Fuzz

Fuzz targets cover parsers and path or protocol inputs where generated data can
exercise malformed or adversarial cases without depending on presentation.

## Excluded presentation behavior

Core tests do not assert prompts, brackets, colors, ANSI sequences, spacing,
progress wording, help layout, golden console output, or exact human-facing
error prose. Presentation adapters may be smoke-tested for write errors, but
runtime behavior is proved with typed errors, semantic events, recorded calls,
turn completion, tool results, persisted state, and clean shutdown.

Secret redaction is an exception: it is a safety transformation independent of
layout and remains covered by semantic tests.
