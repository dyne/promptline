# Promptline v2 toolbox

The toolbox is an instance-owned MCP capability. Promptline starts it as
`promptline toolbox serve`; it uses stdio JSONL and never opens a network
listener. Codex discovers tools through stable MCP `initialize`, `tools/list`,
and `tools/call` requests.

Tools are implemented in Go and u-root. Promptline does not execute `sh -c`,
`bash -c`, or installed host utilities. Filesystem authority comes from the
configured instance roots; traversal, absolute-path ambiguity, symlink escape,
and unsupported special files are denied.

Every tool is bounded by instance output, execution, traversal, and directory
entry limits. Effects require Promptline's approval policy. A missing terminal
or an interrupted prompt fails closed. Audit events are redacted, append-only,
and retained under the instance audit directory.

The portable surface includes filesystem, directory, text-processing, checksum,
system-information, and utility operations registered by `internal/tools`.
`md5sum` and `shasum` are compatibility/file-identification tools, not security
primitives. Unix-only operations such as `mkfifo` report unsupported-platform
errors where unavailable.

To add a tool, register a context-aware implementation in
`internal/tools/builtin.go` or `internal/tools/builtin_uroot.go`. Define a
schema, validate arguments, enforce the scoped-root policy, observe the caller
context, and return bounded structured output. Do not add provider-specific
model types or shell execution.
