# Shell safety

## Interactive commands versus programs

Interactive commands may rely on a human reviewing expansion and output. Automation must define inputs, error semantics, concurrency, idempotence, logging, cleanup and recovery. Do not turn an incident transcript directly into a root cron job.

Quote expansions unless deliberate splitting/globbing is proven safe. Prefer arrays in Bash, `--` before path operands, and explicit validated paths. Treat placeholders such as `UNIT`, `PACKAGE`, `PID`, `FILE`, and `NAME` as symbolic: validate them and pass them as structured MCP fields or single quoted arguments, never interpolate raw user or host text into shell source. Avoid parsing directory listings, ambiguous globs, `eval`, predictable temporary names, and command construction from untrusted text.

For a temporary artifact created directly by the agent, call `toolbox.mktemp`. A standalone Bash program cannot call the agent's MCP namespace at runtime, so code authored for later independent execution should use the native `mktemp` utility plus tested traps. Keep this execution-context exception explicit; it is not permission for the agent to bypass the toolbox.

Pipelines normally report the last command. Check the statuses that matter. `set -e` has contextual exceptions, `set -u` changes handling of unset/empty/positional values, and `pipefail` changes pipeline status; use them only with tested control flow. A blanket `set -euo pipefail` is not a substitute for explicit error handling.

For privileged scripts:

- validate exact targets and reject empty/root/broad paths;
- set a deliberate `PATH` and avoid environment-controlled tool selection;
- use restrictive umask for secrets;
- handle signals and partial state;
- serialize package/config operations where concurrent execution corrupts state;
- support dry-run only when it accurately describes effects;
- make reruns safe or detect and stop on partial completion;
- validate before replacing config and verify after applying it.

Never execute downloaded code through a pipe. Download to a bounded path, verify provenance/integrity/signature, inspect it, and prefer a Debian package or documented reproducible installation.
