#!/usr/bin/env python3
"""Small, deterministic security checks for tracked repository material."""

from __future__ import annotations

import argparse
import ast
import pathlib
import re
import subprocess
import sys
import tempfile

SECRET_PATTERNS = (
    ("private key", re.compile(r"-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----")),
    ("GitHub token", re.compile(r"\bgh[pousr]_[A-Za-z0-9]{36,}\b")),
    ("AWS access key", re.compile(r"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b")),
)


def tracked_files(root: pathlib.Path) -> list[pathlib.Path]:
    result = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"], check=True, capture_output=True
    )
    return [root / name for name in result.stdout.decode().split("\0") if name]


def check_files(files: list[pathlib.Path]) -> list[str]:
    failures: list[str] = []
    for path in files:
        if not path.is_file():
            continue
        if path.suffix == ".sh":
            syntax = subprocess.run(["bash", "-n", str(path)], capture_output=True, text=True)
            if syntax.returncode:
                failures.append(f"{path}: invalid shell: {syntax.stderr.strip()}")
        if path.suffix == ".py":
            try:
                ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
            except (OSError, SyntaxError) as err:
                failures.append(f"{path}: invalid Python: {err}")
            continue
        if path.suffix not in {".go", ".md", ".org", ".sh", ".yml", ".yaml", ".json", ".toml", ""}:
            continue
        try:
            content = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        for label, pattern in SECRET_PATTERNS:
            if pattern.search(content):
                failures.append(f"{path}: possible {label}")
    return failures


def self_test() -> int:
    with tempfile.TemporaryDirectory() as directory:
        fixture = pathlib.Path(directory) / "fixture.md"
        fixture.write_text("token ghp_abcdefghijklmnopqrstuvwxyz0123456789ABCD\n", encoding="utf-8")
        if not check_files([fixture]):
            print("security gate self-test did not detect fixture", file=sys.stderr)
            return 1
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--root", type=pathlib.Path, default=pathlib.Path("."))
    args = parser.parse_args()
    if args.self_test:
        return self_test()
    failures = check_files(tracked_files(args.root.resolve()))
    if failures:
        print("\n".join(failures), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
