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

The embedded-skill contracts additionally verify that the catalog exposes 34 public Debian files, excludes `scripts/` and `tests/`, discovers future top-level `SKILL.md` directories, and preserves bytes when materialized. Command and MCP tests cover `list-skills`, `list-skill-files`, `materialize-skill`, and raw `resources/list`/`resources/read` without creating instance state. App-server integration asserts the two required skill-isolation arguments exactly.

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

## Local targets and release gate

Run all commands from the repository root. The Makefile defaults to Go 1.24 at
`/usr/local/go/bin/go` and puts compilation cache data in `.gocache`; set `GO`
or `GOCACHE` only when an explicit local override is needed.

- `make test-unit` is the fast default loop.
- `make test-protocol` runs the app-server and MCP wire-contract packages.
- `make test-integration` and `make test-race-integration` run the isolated
  subprocess scenarios, with and without the race detector.
- `make test-stress` repeats the concurrent boundary packages under `-race`.
- `make test-fuzz-smoke` fuzzes selected bounded protocol and path parsers for
  five seconds each; it is a smoke gate, not a coverage measurement.
- `make check-coverage` prints and enforces the behavioral-boundary floors.
- `make vet`, `make benchmarks`, and `make build-linux`, `make build-darwin`,
  and `make build-windows` supply static, performance-smoke, and portability
  evidence. `make test-all` runs the complete local release gate.

All targets fail on their first failed command. Temporary coverage profiles are
created under the system temporary directory and removed by their script; no
target writes generated files into tracked package directories.

## Behavior-weighted coverage policy

`scripts/check-coverage.sh` reports statement coverage separately for the
app-server, governance, instance, MCP, paths, runtime, and toolbox packages.
Its floors are 87%, 84%, 76%, 81%, 84%, 74%, and 67%, respectively. They are
rounded down from the post-refactor behavioral suites so the gate catches a
meaningful regression without turning small compiler or Go-version differences
into noise.

The command entrypoint, `main`, and replaceable terminal adapters are excluded
from hard floors. Their low-value statement count would otherwise reward prompt
snapshots, formatting branches, and trivial getters. Coverage is one signal:
race, stress, fuzz, integration, and portability gates remain independent
requirements. To check that the failure path works locally, temporarily raise
one package floor in the script, run `make check-coverage`, and restore the
unchanged floor before committing.
