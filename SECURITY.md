# Security model

Promptline protects the local user's workspace, private instance state, audit
evidence, and the authority granted through terminal approvals.  The Codex
child, model output, MCP clients, workspace contents, and embedded skill data
are untrusted inputs.

The command binary is composed only by `internal/application`; it isolates the
child configuration, gives toolbox operations descriptor-rooted workspace
capabilities, and sends only typed, correlated approval requests to
`internal/governance`. Terminal output is sanitized before display. Private
state and audit journals use rooted handles, atomic replacement, redaction,
and hash-chain verification. A local audit chain detects accidental corruption;
an externally retained anchor is required to detect same-user rewrites.

Embedded skills are read-only catalog data. Their scripts are never MCP tools;
execution requires explicit materialization. The shared bundle license is
`plugins/promptline/skills/LICENSE.txt`.

## Assurance and response

`make test-all` is the release gate. It includes toolchain policy,
vulnerability scanning, workflow authority checks, `security-gates`, unit and
protocol suites, integration/race/stress/fuzz checks, cross-builds, and release
metadata validation. `security-gates` scans tracked material for credentials,
checks shell and Python syntax, and runs pinned gosec. Existing compatibility
hash commands and rooted-handle cleanup findings are explicitly excluded in
the Makefile; new security findings otherwise block CI.

Report suspected credential exposure, path escape, approval confusion, or
audit tampering privately to the maintainers. Preserve the affected state and
external audit anchor; rotate exposed credentials, stop unsafe instances, and
record the remediation in the release notes.
