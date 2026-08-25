#!/usr/bin/env python3
"""Validate the Skill Creator frontmatter and unfinished-placeholder contract.

This is a repository-local compatibility implementation so CI does not depend
on a particular Codex installation path. It intentionally stays independent of
the deeper, project-specific checks in validate-skill.sh.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml


MAX_NAME_LENGTH = 64
ALLOWED_KEYS = {"name", "description", "license", "allowed-tools", "metadata"}


def validate(skill_dir: Path) -> tuple[bool, str]:
    skill_md = skill_dir / "SKILL.md"
    try:
        content = skill_md.read_text(encoding="utf-8")
    except OSError as exc:
        return False, f"cannot read SKILL.md: {exc}"

    match = re.match(r"^---\n(.*?)\n---(?:\n|$)", content, re.DOTALL)
    if not match:
        return False, "invalid or missing YAML frontmatter"
    try:
        frontmatter = yaml.safe_load(match.group(1))
    except yaml.YAMLError as exc:
        return False, f"invalid YAML frontmatter: {exc}"
    if not isinstance(frontmatter, dict):
        return False, "frontmatter must be a YAML mapping"

    unexpected = sorted(repr(key) for key in frontmatter if key not in ALLOWED_KEYS)
    if unexpected:
        return False, "unexpected frontmatter keys: " + ", ".join(unexpected)
    for key in ("name", "description"):
        if key not in frontmatter:
            return False, f"missing {key!r} in frontmatter"

    name = frontmatter["name"]
    if not isinstance(name, str) or not name:
        return False, "name must be a non-empty string"
    if len(name) > MAX_NAME_LENGTH:
        return False, f"name exceeds {MAX_NAME_LENGTH} characters"
    if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", name):
        return False, "name must use lowercase hyphen-case"

    description = frontmatter["description"]
    if not isinstance(description, str) or not description.strip():
        return False, "description must be a non-empty string"
    if len(description.strip()) > 1024:
        return False, "description exceeds 1024 characters"
    if "<" in description or ">" in description:
        return False, "description cannot contain angle brackets"
    if description.lstrip().startswith("[TODO:"):
        return False, "description contains an unfinished TODO"

    fence_character: str | None = None
    fence_length = 0
    for line in content[match.end() :].splitlines():
        fence = re.match(
            r"^[ \t]*(?:(?:[-+*]|\d+[.)])[ \t]+)?(`{3,}|~{3,})(.*)$", line
        )
        if fence:
            marker = fence.group(1)
            if fence_character is None:
                fence_character = marker[0]
                fence_length = len(marker)
            elif (
                marker[0] == fence_character
                and len(marker) >= fence_length
                and not fence.group(2).strip()
            ):
                fence_character = None
                fence_length = 0
            continue
        if fence_character is None and re.fullmatch(r" {0,3}\[TODO:[^\n]*\][ \t]*", line):
            return False, "instructions contain an unfinished TODO"

    return True, "Skill Creator validation passed"


def main() -> int:
    if len(sys.argv) != 2:
        print(f"usage: {Path(sys.argv[0]).name} SKILL_DIRECTORY", file=sys.stderr)
        return 2
    valid, message = validate(Path(sys.argv[1]))
    print(message, file=sys.stdout if valid else sys.stderr)
    return 0 if valid else 1


if __name__ == "__main__":
    raise SystemExit(main())
