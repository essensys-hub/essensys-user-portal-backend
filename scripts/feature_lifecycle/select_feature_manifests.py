#!/usr/bin/env python3
"""Select feature manifests impacted by a set of changed files."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
FEATURES_DIR = REPO_ROOT / "features"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Return the manifest files impacted by a list of changed repository paths."
    )
    parser.add_argument("changed_files", nargs="*", help="Changed repository-relative paths.")
    return parser


def load_manifest(path: Path) -> dict[str, object]:
    return json.loads(path.read_text(encoding="utf-8"))


def rel(path: Path) -> str:
    return str(path.relative_to(REPO_ROOT))


def main() -> int:
    args = build_parser().parse_args()
    changed = {item for item in args.changed_files if item}

    impacted: list[str] = []
    for manifest_path in sorted(FEATURES_DIR.glob("*.json")):
        manifest_rel = rel(manifest_path)
        if manifest_rel in changed:
            impacted.append(manifest_rel)
            continue

        manifest = load_manifest(manifest_path)
        linked_paths = set(manifest.get("userguide", {}).get("pages", []))
        linked_paths.update(manifest.get("implementation", {}).get("paths", []))
        linked_paths.update(manifest.get("tests", {}).get("playwright", []))
        linked_paths.update(manifest.get("tests", {}).get("pytest", []))
        linked_paths.update(manifest.get("tests", {}).get("testthat", []))
        if changed & linked_paths:
            impacted.append(manifest_rel)

    for item in impacted:
        print(item)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
