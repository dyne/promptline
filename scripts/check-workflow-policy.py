#!/usr/bin/env python3
"""Reject mutable workflow dependencies and overly broad release authority."""

import json
import pathlib
import re
import sys
import tempfile

SHA = re.compile(r"^\s*-\s+uses:\s+[^@\s]+@[0-9a-f]{40}(?:\s+#\s+v[^\s]+)?\s*$")
USES = re.compile(r"^\s*-\s+uses:\s+")


def check(workflow: pathlib.Path, release: pathlib.Path) -> list[str]:
    errors: list[str] = []
    lines = workflow.read_text(encoding="utf-8").splitlines()
    if "permissions:\n  contents: read" not in workflow.read_text(encoding="utf-8"):
        errors.append("workflow must default to permissions: contents: read")
    for number, line in enumerate(lines, 1):
        if USES.match(line) and not SHA.match(line):
            errors.append(f"{workflow}:{number}: action reference is not a full commit digest")
    jobs_text = workflow.read_text(encoding="utf-8").split("jobs:\n", 1)[1]
    job_blocks = re.split(r"(?m)^  ([a-z][a-z0-9-]+):\n", jobs_text)[1:]
    for index in range(0, len(job_blocks), 2):
        name, block = job_blocks[index], job_blocks[index + 1]
        if "permissions:" not in block:
            errors.append(f"{workflow}: job {name} has no explicit permissions")
        if re.search(r"\b(contents|attestations|id-token): write", block) and name not in {
            "semantic-versioning", "binary-releases", "remove-tag-on-fail", "upload-releases"
        }:
            errors.append(f"{workflow}: job {name} has unexpected write authority")
        if re.search(r"\b(contents|attestations|id-token): write", block) and "github.event_name == 'push'" not in block:
            errors.append(f"{workflow}: write job {name} must be limited to push events")
    plugins = json.loads(release.read_text(encoding="utf-8")).get("plugins", [])
    if any(not re.fullmatch(r"@semantic-release/[a-z-]+@\d+\.\d+\.\d+", plugin) for plugin in plugins):
        errors.append(f"{release}: semantic-release plugins must include immutable versions")
    return errors


def self_test() -> int:
    with tempfile.TemporaryDirectory() as directory:
        base = pathlib.Path(directory)
        workflow = base / "ci.yml"
        release = base / ".releaserc"
        workflow.write_text("name: x\npermissions:\n  contents: read\njobs:\n  test:\n    permissions:\n      contents: read\n    steps:\n      - uses: actions/checkout@v4\n", encoding="utf-8")
        release.write_text('{"plugins":["@semantic-release/changelog"]}', encoding="utf-8")
        failures = check(workflow, release)
        if len(failures) != 2:
            print("policy self-test did not reject mutable references", file=sys.stderr)
            return 1
    return 0


if __name__ == "__main__":
    if sys.argv[1:] == ["--self-test"]:
        raise SystemExit(self_test())
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} WORKFLOW RELEASERC", file=sys.stderr)
        raise SystemExit(2)
    problems = check(pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]))
    if problems:
        print("\n".join(problems), file=sys.stderr)
        raise SystemExit(1)
