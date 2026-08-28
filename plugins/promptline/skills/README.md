# Embedded skills

`debian-sysadmin/` is the canonical, reviewable on-disk source for the Promptline skill. Go embeds the complete `skills/` tree at build time, so a released Promptline executable does not need a checkout, plugin directory, or relative source path to list, read, or export its public skill material.

The current public catalog has 35 `debian-sysadmin` files. It includes `SKILL.md` and the reviewed Markdown, license, and metadata files exposed by the catalog. `scripts/` and `tests/` are deliberately excluded from the supported API and from materialization. `go:embed *` cannot express those exclusions, so development-file bytes can be present in the executable; they are not listable or readable through the catalog or MCP resource surface.

Catalog discovery is automatic: each top-level directory with a `SKILL.md` is a skill, and its non-excluded regular text files become public. Skill names and every file-name segment use only RFC 3986 unreserved ASCII characters (`A-Z`, `a-z`, digits, `-`, `.`, `_`, and `~`), excluding the special `.` and `..` segments, so every listed `skill://` URI has one canonical representation. Public files must contain valid UTF-8 because MCP serves them as exact text. Adding a future skill in this directory therefore needs no registry edit, but it does need the same review and test coverage as any other embedded input.

## Discovery, transport, and bootstrap

The executable offers these source-independent commands:

```text
promptline list-skills
promptline list-skill-files debian-sysadmin
promptline materialize-skill /absolute/or-relative/destination
```

`materialize-skill` writes only public files. It creates skill directories without overwriting an existing one, rejects symlinks and escaping paths, and uses a staging directory before installing the completed tree. Use it only for a compatibility/debugging export or for a separately launched, filesystem-discovering Codex environment; it does not create Promptline instance state.

`promptline mcp-server` is an ordinary stdio MCP server. Its `resources/list` response inventories `skill://debian-sysadmin/...` URIs, and `resources/read` returns the exact public file bytes for one URI. This is MCP resource discovery and reading, not an invented MCP "skill" extension. The OpenAI documentation describes skills as directories headed by `SKILL.md` and documents stdio MCP servers as a supported Codex connection type; it does not define a server-provided-skill discovery protocol.

When an interactive Promptline instance starts, it writes an instance-private Codex configuration which points `promptline-toolbox` at the same executable's `mcp-server` command. The plugin's `skills/debian-sysadmin` directory remains the filesystem discovery path when the plugin is installed into an independently launched Codex environment. That on-disk discovery is separate from the executable's embedded resource transport.

Promptline-owned app-server children always receive both `skills.include_instructions=false` and `skills.bundled.enabled=false`. These required isolation overrides prevent system/user skill instructions and Codex-bundled skills from influencing that child. Consequently, if the MCP client in an executable-only run cannot list/read Promptline resources, the authoritative Debian skill cannot be activated during that run. Materializing the files is not a bypass for those two overrides; it is only the fallback for another filesystem-based client or independently launched Codex process.

For the current OpenAI behavior, see [Build skills](https://developers.openai.com/codex/skills) and [Model Context Protocol](https://developers.openai.com/codex/mcp).
