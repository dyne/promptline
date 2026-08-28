# Embedded skills

Each top-level skill directory is the canonical, reviewable on-disk source for that Promptline skill. Go embeds the complete `skills/` tree at build time, so a released Promptline executable does not need a checkout, plugin directory, or relative source path to list, read, or export its public skill material.

The catalog includes every skill's `SKILL.md` and its readable references, playbooks, metadata, and operational scripts. Tests are excluded. The `debian-sysadmin/scripts/` directory contains build-time validation rather than agent runtime material, so it is also excluded. Operational scripts owned by other skills are exposed only as static resources; MCP never executes them. Use `materialize-skill` before running one. `LICENSE.txt` is the single consolidated license file for the complete embedded skill bundle; it maps components to their MIT or Apache-2.0 terms and is exposed and materialized once rather than copied into every skill.

Catalog discovery is automatic: each top-level directory with a `SKILL.md` is a skill, and its non-excluded regular UTF-8 text files become public. The catalog reads `name` and `description` from each skill's YAML frontmatter for startup discovery. Skill names and every file-name segment use only RFC 3986 unreserved ASCII characters (`A-Z`, `a-z`, digits, `-`, `.`, `_`, and `~`), excluding the special `.` and `..` segments, so every listed `skill://` URI has one canonical representation. Adding a future skill in this directory needs no registry or init-prompt edit, but it does need the same review and test coverage as any other embedded input.

## Discovery, transport, and bootstrap

The executable offers these source-independent commands:

```text
promptline list-skills
promptline list-skill-files debian-sysadmin
promptline materialize-skill /absolute/or-relative/destination
```

`materialize-skill` writes only public files plus the one shared `LICENSE.txt`. The requested destination must not exist. Promptline builds the complete tree in a sibling staging directory, then atomically installs it without replacement; existing data is refused intact. Parent paths are created safely, and symlinks or escaping paths are rejected. Use it only for compatibility/debugging, to run a skill-owned script, or for a separately launched filesystem-discovering Codex environment; it does not create Promptline instance state.

`promptline mcp-server` is an ordinary stdio MCP server. Its initialize response supplies the names and frontmatter descriptions of every embedded skill. `resources/list` inventories `skill://<name>/...` resources and the shared `skill-bundle://promptline/LICENSE.txt`; `resources/read` returns exact bytes. This is MCP resource discovery and reading, not an invented MCP "skill" extension. The installed MCP protocol has no server-provided-skill extension, so Promptline uses standard server instructions and resources with progressive disclosure.

When an interactive Promptline instance starts, it writes an instance-private Codex configuration which points `promptline-toolbox` at the same executable's `mcp-server` command. The init prompt includes the same catalog orientation: match against startup summaries, then load the authoritative `skill://<name>/SKILL.md` and its relative resources only when applicable. The plugin's `skills/` directory remains the filesystem discovery path when the plugin is installed into an independently launched Codex environment. That on-disk discovery is separate from the executable's embedded resource transport.

Promptline-owned app-server children always receive both `skills.include_instructions=false` and `skills.bundled.enabled=false`. These required isolation overrides prevent system/user skill instructions and Codex-bundled skills from influencing that child. Consequently, if the MCP client in an executable-only run cannot list/read Promptline resources, the authoritative embedded skills cannot be activated during that run. Materializing the files is not a bypass for those two overrides; it is only the fallback for another filesystem-based client or independently launched Codex process.

For the current OpenAI behavior, see [Build skills](https://developers.openai.com/codex/skills) and [Model Context Protocol](https://developers.openai.com/codex/mcp).
