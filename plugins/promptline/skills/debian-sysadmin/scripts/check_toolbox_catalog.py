#!/usr/bin/env python3
"""Compare the documented Promptline toolbox catalog with live MCP tools/list."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


REQUESTS = (
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n'
    '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n'
)
TABLE_HEADER = "| Category | Toolbox tools |"
TOOL_NAME = re.compile(r"^[a-z][a-z0-9_]*$")
BACKTICKED = re.compile(r"`([a-z][a-z0-9_]*)`")
MAX_RESPONSE_BYTES = 8 * 1024 * 1024


class CatalogError(RuntimeError):
    """A catalog or MCP protocol contract was not satisfied."""


def documented_names(path: Path) -> list[str]:
    """Extract tool names from the designated Markdown catalog table."""
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise CatalogError(f"cannot read catalog {path}: {exc}") from exc

    try:
        start = lines.index(TABLE_HEADER)
    except ValueError as exc:
        raise CatalogError(f"catalog table header not found in {path}") from exc

    rows: list[str] = []
    for line in lines[start + 2 :]:
        if not line.startswith("|"):
            break
        rows.extend(BACKTICKED.findall(line))
    if not rows:
        raise CatalogError(f"catalog table in {path} contains no tool names")
    return rows


def live_definitions(binary: Path) -> list[dict[str, object]]:
    """Start Promptline with isolated roots and request its live tool schema."""
    try:
        resolved = binary.resolve(strict=True)
    except OSError as exc:
        raise CatalogError(f"cannot resolve Promptline binary {binary}: {exc}") from exc
    if not resolved.is_file() or not os.access(resolved, os.X_OK):
        raise CatalogError(f"Promptline binary is not an executable regular file: {resolved}")

    with tempfile.TemporaryDirectory(prefix="promptline-catalog-cwd-") as cwd:
        with tempfile.TemporaryDirectory(prefix="promptline-catalog-state-") as state:
            try:
                completed = subprocess.run(
                    [
                        str(resolved),
                        "mcp-server",
                        "--cwd",
                        cwd,
                        "--state-root",
                        state,
                    ],
                    input=REQUESTS,
                    text=True,
                    capture_output=True,
                    timeout=20,
                    check=False,
                )
            except (OSError, subprocess.TimeoutExpired) as exc:
                raise CatalogError(f"cannot query Promptline MCP server: {exc}") from exc

    if completed.returncode != 0:
        detail = completed.stderr.strip()[:1000]
        raise CatalogError(
            f"Promptline MCP server exited {completed.returncode}"
            + (f": {detail}" if detail else "")
        )
    if len(completed.stdout.encode("utf-8")) > MAX_RESPONSE_BYTES:
        raise CatalogError("Promptline MCP response exceeds 8 MiB")

    responses: dict[object, dict[str, object]] = {}
    for number, line in enumerate(completed.stdout.splitlines(), 1):
        if not line.strip():
            continue
        try:
            response = json.loads(line)
        except json.JSONDecodeError as exc:
            raise CatalogError(f"invalid JSON response on line {number}: {exc}") from exc
        if not isinstance(response, dict):
            raise CatalogError(f"MCP response on line {number} is not an object")
        response_id = response.get("id")
        if response_id in responses:
            raise CatalogError(f"duplicate MCP response id: {response_id!r}")
        responses[response_id] = response

    initialize = responses.get(1, {}).get("result")
    if not isinstance(initialize, dict):
        raise CatalogError("missing successful initialize response")
    server_info = initialize.get("serverInfo")
    if not isinstance(server_info, dict) or server_info.get("name") != "promptline-toolbox":
        raise CatalogError("initialize response is not from promptline-toolbox")

    listing = responses.get(2, {}).get("result")
    if not isinstance(listing, dict) or not isinstance(listing.get("tools"), list):
        raise CatalogError("missing successful tools/list response")
    return listing["tools"]


def validate_definitions(definitions: list[dict[str, object]]) -> list[str]:
    """Validate the MCP catalog shape and return names in live order."""
    names: list[str] = []
    for index, definition in enumerate(definitions):
        if not isinstance(definition, dict):
            raise CatalogError(f"tools[{index}] is not an object")
        name = definition.get("name")
        description = definition.get("description")
        schema = definition.get("inputSchema")
        if not isinstance(name, str) or not TOOL_NAME.fullmatch(name):
            raise CatalogError(f"tools[{index}] has an invalid name: {name!r}")
        if not isinstance(description, str) or not description.strip():
            raise CatalogError(f"tool {name!r} has no description")
        if not isinstance(schema, dict) or schema.get("type") != "object":
            raise CatalogError(f"tool {name!r} has no object inputSchema")
        names.append(name)
    duplicates = sorted({name for name in names if names.count(name) > 1})
    if duplicates:
        raise CatalogError(f"duplicate live tool names: {', '.join(duplicates)}")
    return names


def compare(documented: list[str], live: list[str]) -> None:
    duplicates = sorted({name for name in documented if documented.count(name) > 1})
    if duplicates:
        raise CatalogError(f"duplicate documented tool names: {', '.join(duplicates)}")
    missing = sorted(set(live) - set(documented))
    stale = sorted(set(documented) - set(live))
    if missing or stale:
        details = []
        if missing:
            details.append("missing from documentation: " + ", ".join(missing))
        if stale:
            details.append("not present in live MCP: " + ", ".join(stale))
        raise CatalogError("toolbox catalog drift detected; " + "; ".join(details))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--promptline", required=True, type=Path, help="Promptline executable")
    parser.add_argument(
        "--catalog",
        type=Path,
        default=Path(__file__).resolve().parent.parent / "references" / "toolbox.md",
        help="Markdown file containing the toolbox catalog table",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        documented = documented_names(args.catalog)
        live = validate_definitions(live_definitions(args.promptline))
        compare(documented, live)
    except CatalogError as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1
    print(f"Promptline toolbox catalog matches {len(live)} live MCP tool definitions")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
