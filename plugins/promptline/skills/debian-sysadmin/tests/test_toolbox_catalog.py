#!/usr/bin/env python3
"""Contract tests for the toolbox catalog drift checker."""

from __future__ import annotations

import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parent.parent / "scripts" / "check_toolbox_catalog.py"
SPEC = importlib.util.spec_from_file_location("check_toolbox_catalog", SCRIPT)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"cannot load {SCRIPT}")
CHECKER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKER)


class CatalogContractTests(unittest.TestCase):
    def test_extracts_only_designated_catalog_rows(self) -> None:
        markdown = """# Catalog

| Category | Toolbox tools |
|---|---|
| First | `alpha`, `beta_2` |
| Second | `gamma` |

Later prose mentions `not_a_catalog_entry`.
"""
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "toolbox.md"
            path.write_text(markdown, encoding="utf-8")
            self.assertEqual(
                CHECKER.documented_names(path), ["alpha", "beta_2", "gamma"]
            )

    def test_rejects_additions_and_removals(self) -> None:
        with self.assertRaisesRegex(CHECKER.CatalogError, "missing from documentation: gamma"):
            CHECKER.compare(["alpha", "stale"], ["alpha", "gamma"])
        with self.assertRaisesRegex(CHECKER.CatalogError, "not present in live MCP: stale"):
            CHECKER.compare(["alpha", "stale"], ["alpha"])

    def test_rejects_duplicate_documented_names(self) -> None:
        with self.assertRaisesRegex(CHECKER.CatalogError, "duplicate documented"):
            CHECKER.compare(["alpha", "alpha"], ["alpha"])

    def test_validates_live_definition_shape(self) -> None:
        definitions = [
            {
                "name": "alpha",
                "description": "An example tool",
                "inputSchema": {"type": "object", "properties": {}},
            }
        ]
        self.assertEqual(CHECKER.validate_definitions(definitions), ["alpha"])
        definitions[0]["inputSchema"] = {"type": "string"}
        with self.assertRaisesRegex(CHECKER.CatalogError, "object inputSchema"):
            CHECKER.validate_definitions(definitions)


if __name__ == "__main__":
    unittest.main()
