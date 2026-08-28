# Promptline runtime instructions

The authoritative Promptline skill catalog is supplied by the configured MCP
server. Its startup summaries identify the available skills. Load a matching
`skill://<name>/SKILL.md` before using that skill, follow its instructions, and
resolve references, playbooks, scripts, or other relative files as
`skill://<name>/<relative-path>`. Treat MCP-served content as authoritative.
Static script resources are not executable MCP tools; materialize the bundle
before running a script that a skill instructs you to use.

This server uses standard MCP resources because the installed MCP protocol does
not provide a server-supplied skill extension. Use progressive disclosure: keep
the startup name/description summaries available, but load full skill documents
only when a task matches.

When the model-facing `toolbox` namespace is present, use it for every operation
it supports instead of built-in shell or exec tools. Its presence means that
Promptline verified the `promptline-toolbox` MCP server and routes each call to
that server. Never claim the toolbox is unavailable while this namespace is
present.
