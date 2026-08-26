# debian-sysadmin

An Agent Skill for disciplined administration of modern Debian systems. It emphasizes evidence-first diagnosis, Debian-native mechanisms, remote survivability, rollback before mutation, and verification of actual runtime behavior.

The repository intentionally exposes one skill. `SKILL.md` routes subsystem work to focused references and incident playbooks so an agent does not load an encyclopedia for every task.

## Scope

Primary targets are Debian stable and oldstable on physical hosts, VMs, VPS instances, servers reached through SSH, and local workstations. Debian testing is supported only when identified explicitly. Ubuntu compatibility is secondary; other distribution families are out of scope.

## Install

The preferred installation is the repository's `sysadmin` Codex plugin, which bundles this skill with the Promptline toolbox MCP configuration. For a standalone human-managed skill installation, copy this reviewed directory into the Codex skills directory. If an agent performs installation inside Promptline and both paths are inside its configured roots, it must call `toolbox.mkdir` and recursive `toolbox.cp` rather than invoke the same-named host binaries.

```sh
mkdir -p "${CODEX_HOME:-$HOME/.codex}/skills"
cp -a ./plugins/promptline/skills/debian-sysadmin "${CODEX_HOME:-$HOME/.codex}/skills/debian-sysadmin"
```

For development, a symlink keeps the installed skill synchronized:

```sh
mkdir -p "${CODEX_HOME:-$HOME/.codex}/skills"
ln -s "$PWD/plugins/promptline/skills/debian-sysadmin" "${CODEX_HOME:-$HOME/.codex}/skills/debian-sysadmin"
```

Use the symlink only with a trusted, access-controlled working tree: later checkout or workspace changes immediately become installed instructions. An agent creating the link must call `toolbox.ln` with `symbolic: true`; if source or destination is outside toolbox authority, it must stop rather than fall back to host `ln`. Prefer a reviewed copy for normal use.

Run `scripts/validate-skill.sh` before distributing changes. Repository CI also runs ShellCheck, a repository-local Skill Creator compatibility validator, and `scripts/check_toolbox_catalog.py` against a freshly built Promptline MCP server. The catalog check compares the documented names with the live `tools/list` result and validates that every live definition has a name, description, and object input schema.

See `docs/DESIGN.md`, `docs/UPSTREAM-REVIEW.md`, and `docs/PROVENANCE.md` for design and source history.
