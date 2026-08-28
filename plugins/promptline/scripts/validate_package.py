#!/usr/bin/env python3
"""Validate Promptline's Codex plugin, MCP, skill, and marketplace wiring."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from typing import Any


PLUGIN_ROOT = Path(__file__).resolve().parent.parent
REPOSITORY_ROOT = PLUGIN_ROOT.parents[1]
SEMVER = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:[-+][0-9A-Za-z.-]+)?$")
MCP_REQUESTS = (
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n'
    '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n'
    '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"pwd","arguments":{}}}\n'
)


def load_object(path: Path, failures: list[str]) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        failures.append(f"cannot load {path.relative_to(REPOSITORY_ROOT)}: {exc}")
        return {}
    if not isinstance(value, dict):
        failures.append(f"{path.relative_to(REPOSITORY_ROOT)} must contain an object")
        return {}
    return value


def require_string(value: Any, label: str, failures: list[str]) -> None:
    if not isinstance(value, str) or not value.strip():
        failures.append(f"{label} must be a non-empty string")


def validate(promptline: Path | None = None) -> list[str]:
    failures: list[str] = []
    manifest = load_object(PLUGIN_ROOT / ".codex-plugin" / "plugin.json", failures)
    mcp = load_object(PLUGIN_ROOT / ".mcp.json", failures)
    marketplace = load_object(
        REPOSITORY_ROOT / ".agents" / "plugins" / "marketplace.json", failures
    )

    if manifest.get("name") != "sysadmin":
        failures.append("plugin name must be sysadmin")
    version = manifest.get("version")
    if not isinstance(version, str) or SEMVER.fullmatch(version) is None:
        failures.append("plugin version must use semantic versioning")
    for field in ("description", "homepage", "repository", "license"):
        require_string(manifest.get(field), f"plugin {field}", failures)
    if manifest.get("skills") != "./skills/":
        failures.append("plugin skills must resolve to ./skills/")
    if manifest.get("mcpServers") != "./.mcp.json":
        failures.append("plugin mcpServers must resolve to ./.mcp.json")
    skills_root = PLUGIN_ROOT / "skills"
    if not skills_root.is_dir():
        failures.append("plugin is missing its skills directory")
        skill_names: list[str] = []
    else:
        skill_names = sorted(
            path.name for path in skills_root.iterdir()
            if path.is_dir() and (path / "SKILL.md").is_file()
        )
    if not skill_names:
        failures.append("plugin must contain at least one skill")
    if not (skills_root / "LICENSE.txt").is_file():
        failures.append("plugin skills must have one shared LICENSE.txt")
    else:
        license_text = (skills_root / "LICENSE.txt").read_text(encoding="utf-8")
        for required_notice in (
            "MIT License",
            "Apache License",
            "Copyright (c) 2024 Seth Hobson",
            "Copyright (c) 2026 Antigravity User",
            "security-ownership-map",
            "security-threat-model",
        ):
            if required_notice not in license_text:
                failures.append(
                    f"shared skills license is missing {required_notice!r}"
                )
    for skill_name in skill_names:
        if (skills_root / skill_name / "LICENSE").exists() or (
            skills_root / skill_name / "LICENSE.txt"
        ).exists():
            failures.append(f"skill {skill_name} duplicates the shared license")

    interface = manifest.get("interface")
    if not isinstance(interface, dict):
        failures.append("plugin interface must be an object")
    else:
        if interface.get("displayName") != "Sysadmin":
            failures.append("plugin interface.displayName must be Sysadmin")
        for field in (
            "displayName",
            "shortDescription",
            "longDescription",
            "developerName",
            "category",
        ):
            require_string(interface.get(field), f"plugin interface.{field}", failures)
        prompts = interface.get("defaultPrompt")
        if not isinstance(prompts, list) or not prompts or not all(
            isinstance(prompt, str) and prompt.strip() for prompt in prompts
        ):
            failures.append("plugin interface.defaultPrompt must contain strings")

    servers = mcp.get("mcpServers")
    if not isinstance(servers, dict) or set(servers) != {"promptline-toolbox"}:
        failures.append(".mcp.json must expose only promptline-toolbox")
    else:
        server = servers["promptline-toolbox"]
        if not isinstance(server, dict):
            failures.append("promptline-toolbox configuration must be an object")
        else:
            if server.get("command") != "promptline":
                failures.append("promptline-toolbox must invoke the promptline executable")
            if server.get("args") != ["mcp-server"]:
                failures.append("promptline-toolbox must run only the mcp-server command")
            if "cwd" in server:
                failures.append("promptline-toolbox must inherit the Codex working directory")
            forbidden = {"url", "shell", "bash", "app", "resume", "new"}
            if forbidden.intersection(server):
                failures.append("promptline-toolbox contains an interactive or remote launch field")

    if marketplace.get("name") != "promptline":
        failures.append("repository marketplace name must be promptline")
    entries = marketplace.get("plugins")
    if not isinstance(entries, list) or len(entries) != 1 or not isinstance(entries[0], dict):
        failures.append("repository marketplace must contain exactly one plugin entry")
    else:
        entry = entries[0]
        if entry.get("name") != "sysadmin":
            failures.append("marketplace plugin name must be sysadmin")
        if entry.get("source") != {
            "source": "local",
            "path": "./plugins/promptline",
        }:
            failures.append("marketplace source must point to ./plugins/promptline")
        policy = entry.get("policy")
        if policy != {"installation": "AVAILABLE", "authentication": "ON_INSTALL"}:
            failures.append("marketplace policy must use the reviewed install defaults")

    installer = REPOSITORY_ROOT / "install.sh"
    if not installer.is_file() or not os.access(installer, os.X_OK):
        failures.append("repository install.sh must be an executable regular file")

    for path in PLUGIN_ROOT.rglob("*"):
        if path.is_symlink():
            failures.append(f"plugin archive must not contain symlink: {path.relative_to(PLUGIN_ROOT)}")
    if promptline is not None and isinstance(servers, dict):
        server = servers.get("promptline-toolbox")
        if isinstance(server, dict):
            validate_live_mcp(promptline, server, failures)
    return failures


def validate_live_mcp(
    promptline: Path, server: dict[str, Any], failures: list[str]
) -> None:
    try:
        executable = promptline.resolve(strict=True)
    except OSError as exc:
        failures.append(f"cannot resolve live Promptline executable: {exc}")
        return
    if not executable.is_file() or not os.access(executable, os.X_OK):
        failures.append("live Promptline path must be an executable regular file")
        return
    args = server.get("args")
    if not isinstance(args, list) or not all(isinstance(arg, str) for arg in args):
        return

    with tempfile.TemporaryDirectory(prefix="promptline-plugin-mcp-") as working_directory:
        try:
            completed = subprocess.run(
                [str(executable), *args],
                cwd=working_directory,
                input=MCP_REQUESTS,
                text=True,
                capture_output=True,
                timeout=20,
                check=False,
            )
        except (OSError, subprocess.TimeoutExpired) as exc:
            failures.append(f"cannot launch packaged MCP command: {exc}")
            return
        if completed.returncode != 0:
            detail = completed.stderr.strip()[:1000]
            failures.append(
                f"packaged MCP command exited {completed.returncode}"
                + (f": {detail}" if detail else "")
            )
            return

        responses: dict[Any, dict[str, Any]] = {}
        for number, line in enumerate(completed.stdout.splitlines(), 1):
            try:
                response = json.loads(line)
            except json.JSONDecodeError as exc:
                failures.append(f"packaged MCP response line {number} is invalid JSON: {exc}")
                return
            if not isinstance(response, dict):
                failures.append(f"packaged MCP response line {number} is not an object")
                return
            responses[response.get("id")] = response

        initialized = responses.get(1, {}).get("result")
        if not isinstance(initialized, dict):
            failures.append("packaged MCP command did not initialize")
        else:
            server_info = initialized.get("serverInfo")
            if not isinstance(server_info, dict) or server_info.get("name") != "promptline-toolbox":
                failures.append("packaged MCP command initialized the wrong server")
        listed = responses.get(2, {}).get("result")
        if not isinstance(listed, dict) or not isinstance(listed.get("tools"), list):
            failures.append("packaged MCP command did not return tools/list")
        called = responses.get(3, {}).get("result")
        content = called.get("content") if isinstance(called, dict) else None
        text = content[0].get("text") if isinstance(content, list) and content and isinstance(content[0], dict) else None
        if text != working_directory:
            failures.append("packaged MCP command did not inherit its launch working directory")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--promptline",
        type=Path,
        help="fresh Promptline executable used to exercise the packaged MCP command",
    )
    return parser.parse_args()


def main() -> int:
    failures = validate(parse_args().promptline)
    if failures:
        for failure in failures:
            print(f"FAIL: {failure}", file=sys.stderr)
        return 1
    print("Promptline Codex plugin package validation passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
