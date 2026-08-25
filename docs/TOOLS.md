# Promptline toolbox

The toolbox is an instance-owned MCP capability. Promptline starts it as
`promptline mcp-server`; it uses stdio JSONL and never opens a network
listener. Codex discovers tools through stable MCP `initialize`, `tools/list`,
and `tools/call` requests.

The same command can be run directly by any MCP harness. It scopes filesystem
operations to `--cwd` (the current directory when omitted) and does not create a
Promptline instance or start Codex.

Promptline checks the thread-scoped `mcpServerStatus/list` inventory before it
shows the terminal prompt. Startup fails if `promptline-toolbox` or its core
`ls`, `pwd`, and `cat` tools are absent. A successful launch prints the number
of discovered tools, and thread developer instructions direct Codex to prefer
the toolbox over built-in shell execution for operations it supports.

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

To add a tool, add a context-aware implementation and catalog entry under
`internal/tools`. The catalog is authoritative for registration and the
read-only/mutating policy class. Define an object schema, validate arguments,
enforce the scoped-root policy, observe the caller context, and return bounded
structured output. Update the catalog contract tests so registry, MCP list, and
dynamic definitions stay aligned. Do not add provider-specific model types or
shell execution.

New integrations use the stable stdio MCP surface and remain subject to the
same instance approval and audit boundaries.
