#!/usr/bin/env python3
"""Validate feature manifest files against the shared JSON Schema."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from jsonschema import Draft202012Validator


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_SCHEMA = REPO_ROOT / "features" / "schema" / "feature.schema.json"
DEFAULT_FEATURES_DIR = REPO_ROOT / "features"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Validate Skills Portal feature manifests against the shared JSON Schema."
    )
    parser.add_argument(
        "paths",
        nargs="*",
        help="Feature manifest files to validate. Defaults to features/*.json (excluding schema/).",
    )
    parser.add_argument(
        "--schema",
        default=str(DEFAULT_SCHEMA),
        help=f"Path to the JSON Schema (default: {DEFAULT_SCHEMA}).",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit machine-readable JSON instead of a human summary.",
    )
    return parser


def discover_paths(raw_paths: list[str]) -> list[Path]:
    if raw_paths:
        return [Path(path).resolve() for path in raw_paths]

    return sorted(
        path.resolve()
        for path in DEFAULT_FEATURES_DIR.glob("*.json")
        if path.name != "schema"
    )


def load_json(path: Path) -> object:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def validate_manifest(path: Path, validator: Draft202012Validator) -> dict[str, object]:
    result: dict[str, object] = {
        "path": str(path.relative_to(REPO_ROOT)),
        "ok": True,
        "errors": [],
    }

    if not path.exists():
        result["ok"] = False
        result["errors"] = ["File does not exist."]
        return result

    try:
        payload = load_json(path)
    except json.JSONDecodeError as exc:
        result["ok"] = False
        result["errors"] = [f"Invalid JSON: {exc.msg} at line {exc.lineno}, column {exc.colno}."]
        return result

    errors = sorted(validator.iter_errors(payload), key=lambda item: list(item.path))
    if errors:
        rendered: list[str] = []
        for error in errors:
            location = ".".join(str(part) for part in error.path) or "<root>"
            rendered.append(f"{location}: {error.message}")
        result["ok"] = False
        result["errors"] = rendered

    return result


def main() -> int:
    args = build_parser().parse_args()
    schema_path = Path(args.schema).resolve()

    try:
        schema = load_json(schema_path)
    except FileNotFoundError:
        print(f"Schema not found: {schema_path}", file=sys.stderr)
        return 2
    except json.JSONDecodeError as exc:
        print(
            f"Schema JSON is invalid: {exc.msg} at line {exc.lineno}, column {exc.colno}.",
            file=sys.stderr,
        )
        return 2

    validator = Draft202012Validator(schema)
    manifest_paths = discover_paths(args.paths)
    if not manifest_paths:
        print("No feature manifests found to validate.", file=sys.stderr)
        return 0

    results = [validate_manifest(path, validator) for path in manifest_paths]
    ok = all(bool(item["ok"]) for item in results)

    if args.json:
        print(json.dumps({"ok": ok, "results": results}, indent=2))
    else:
        for result in results:
            prefix = "OK" if result["ok"] else "FAIL"
            print(f"[{prefix}] {result['path']}")
            for error in result["errors"]:
                print(f"  - {error}")

    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
