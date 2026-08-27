# Promptline runtime instructions

Skill name: `debian-sysadmin`. Description: authoritative Debian system
administration guidance from the configured MCP server. Load
`skill://debian-sysadmin/SKILL.md` and follow it. Resolve every relative
reference, playbook, or other skill file as `skill://debian-sysadmin/...`; MCP
resource content is authoritative. This server does not provide a
server-provided-skill extension, so use standard MCP resource access when the
client exposes it.

When the model-facing `toolbox` namespace is present, use it for every operation
it supports instead of built-in shell or exec tools. Its presence means that
Promptline verified the `promptline-toolbox` MCP server and routes each call to
that server. Never claim the toolbox is unavailable while this namespace is
present.
