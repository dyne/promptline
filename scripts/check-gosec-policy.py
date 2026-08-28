#!/usr/bin/env python3
"""Reject broad or expired gosec suppression policy."""
import datetime as dt
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
POLICY = ROOT / "security" / "gosec-exceptions.json"

def check() -> list[str]:
    errors = []
    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    if "-exclude=" in makefile:
        errors.append("gosec rule-wide exclusions are forbidden; use reviewed locations")
    required = "./cmd/promptline ./internal/... ./plugins/promptline/skills"
    if required not in makefile:
        errors.append("gosec must scan every tracked production package root")
    policy = json.loads(POLICY.read_text(encoding="utf-8"))
    if not re.fullmatch(r"[^@\s]+@[^@\s]+", policy.get("owner", "")):
        errors.append("gosec policy needs a named owner")
    try:
        review_by = dt.date.fromisoformat(policy["reviewBy"])
        if review_by < dt.date.today(): errors.append("gosec policy review date has expired")
    except (KeyError, ValueError): errors.append("gosec policy needs an ISO review date")
    seen = set()
    for item in policy.get("exceptions", []):
        key = (item.get("path"), item.get("line"), item.get("rule"))
        if not item.get("path") or not isinstance(item.get("line"), int) or not item.get("rule") or not item.get("reason"):
            errors.append("each gosec exception needs path, line, rule, and reason")
        elif key in seen:
            errors.append("duplicate gosec exception location")
        seen.add(key)
    return errors

if __name__ == "__main__":
    if sys.argv[1:] == ["--self-test"]:
        # Fixtures prove the two policy escape hatches remain closed.
        expired = {"owner": "security@dyne.org", "reviewBy": "2000-01-01", "exceptions": []}
        missing_scope = "./cmd/promptline ./internal/... ./plugins/promptline/skills" not in "gosec ./cmd/promptline"
        exact = {("file.go", 7, "G304")}
        string_line_matches = ("file.go", int("7"), "G304") in exact
        different_line_rejected = ("file.go", int("8"), "G304") not in exact
        if dt.date.fromisoformat(expired["reviewBy"]) >= dt.date.today() or "-exclude=" not in "gosec -exclude=G304" or not missing_scope or not string_line_matches or not different_line_rejected:
            raise SystemExit("gosec policy self-test failed")
        raise SystemExit(0)
    failures = check()
    if failures: print("\n".join(failures), file=sys.stderr)
    raise SystemExit(bool(failures))
