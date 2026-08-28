#!/usr/bin/env python3
"""Run full gosec and fail unless every finding is a reviewed exception."""
import json, pathlib, subprocess, sys

root = pathlib.Path(__file__).resolve().parents[1]
policy = json.loads((root / "security/gosec-exceptions.json").read_text())
allowed = {(item["path"], item["line"], item["rule"]) for item in policy["exceptions"]}
command = [sys.argv[1], "-fmt=json", *sys.argv[2:]]
result = subprocess.run(command, cwd=root, capture_output=True, text=True)
try:
    report = json.loads(result.stdout)
except json.JSONDecodeError:
    sys.stderr.write(result.stderr or result.stdout)
    raise SystemExit(result.returncode or 1)
unexpected = []
for issue in report.get("Issues", []):
    path = pathlib.Path(issue["file"]).resolve().relative_to(root).as_posix()
    try:
        line = int(issue["line"])
    except (KeyError, TypeError, ValueError):
        unexpected.append(f"{path}: malformed gosec line {issue.get('line')!r}")
        continue
    if line < 1 or (path, line, issue["rule_id"]) not in allowed:
        unexpected.append(f"{path}:{line} {issue['rule_id']}: {issue['details']}")
if unexpected:
    sys.stderr.write("untriaged gosec findings:\n" + "\n".join(unexpected) + "\n")
    raise SystemExit(1)
print(f"gosec full-scope findings reviewed: {len(report.get('Issues', []))}")
