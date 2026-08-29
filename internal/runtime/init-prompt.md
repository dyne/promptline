# Promptline runtime instructions

Promptline provides the authoritative skill catalog through its configured MCP server.

At startup, the server exposes a short name and description for each available skill. Keep these summaries available for routing, but do not load every skill in full.

When the current task matches a skill:

1. Load its main instructions from:

   `skill://<name>/SKILL.md`

2. Follow the instructions in that document.

3. Resolve files referenced by the skill relative to the same skill root. For example:

   `playbooks/disk-full.md`

   resolves to:

   `skill://<name>/playbooks/disk-full.md`

   The same rule applies to references, scripts and other files belonging to the skill.

Content served through the configured Promptline MCP server is authoritative for the skills it provides. Do not substitute another local copy of the same skill unless the Promptline resource explicitly instructs you to do so.

## Progressive loading

Promptline exposes skills through standard MCP resources. The current MCP protocol does not provide a native server-defined "skill" object, so Promptline represents skill documents and their files as resources under the `skill://` URI scheme.

Use progressive disclosure:

* keep skill names and descriptions available for deciding which skill applies;
* load `SKILL.md` only when a task matches that skill;
* load additional references, playbooks or scripts only when required by the skill instructions.

Do not preload complete skill bundles unnecessarily.

## Scripts

Files under a skill, including scripts, are MCP resources. A script resource is not automatically an executable MCP tool.

If a skill instructs you to execute one of its scripts, first materialize the relevant skill bundle or script onto the local filesystem, then execute the resulting local file using an appropriate execution tool.

Do not attempt to invoke a `skill://.../script` resource as though it were an MCP tool.

## Toolbox precedence

If the model-facing `toolbox` namespace is present, Promptline has successfully connected and verified the `promptline-toolbox` MCP server.

For every operation supported by `toolbox`, use the corresponding `toolbox` operation instead of a built-in shell, exec or equivalent tool.

Use built-in execution tools only for operations that `toolbox` does not support.

Do not report that the Promptline toolbox is unavailable while the `toolbox` namespace is present.
