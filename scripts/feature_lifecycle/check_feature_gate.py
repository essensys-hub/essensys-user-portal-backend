#!/usr/bin/env python3
"""Run repository-specific checks for feature lifecycle manifests."""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


REPO_ROOT = Path(__file__).resolve().parents[2]
FEATURES_DIR = REPO_ROOT / "features"
TEST_TITLE_PATTERN = re.compile(
    r"""(?:test|it)(?:\.describe)?\(\s*(['"])(?P<title>.+?)\1""",
    re.MULTILINE,
)
PYTEST_TITLE_PATTERN = re.compile(r"^\s*def\s+(test_[A-Za-z0-9_]+)\s*\(", re.MULTILINE)
TESTTHAT_TITLE_PATTERN = re.compile(
    r"""test_that\(\s*(['"])(?P<title>.+?)\1""",
    re.MULTILINE,
)
TOKEN_PATTERN = re.compile(r"[a-z0-9]+")


@dataclass
class ManifestResult:
    path: str
    errors: list[str]
    warnings: list[str]

    @property
    def ok(self) -> bool:
        return not self.errors


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Run feature lifecycle repository checks beyond JSON Schema validation."
    )
    parser.add_argument(
        "paths",
        nargs="*",
        help="Feature manifest files to validate. Defaults to features/*.json.",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Exit non-zero when any repository check fails.",
    )
    parser.add_argument(
        "--mirror-repo",
        action="store_true",
        help=(
            "Profile source-of-truth mode: missing declared paths are warnings, not errors "
            "(implementation lives in downstream app repositories)."
        ),
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit machine-readable JSON output.",
    )
    return parser


def discover_manifests(raw_paths: list[str]) -> list[Path]:
    if raw_paths:
        return [Path(path).resolve() for path in raw_paths]
    return sorted(FEATURES_DIR.glob("*.json"))


def load_manifest(path: Path) -> dict[str, object]:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def rel(path: Path) -> str:
    try:
        return str(path.relative_to(REPO_ROOT))
    except ValueError:
        return str(path)


def path_exists(path_str: str) -> bool:
    return (REPO_ROOT / path_str).exists()


def collect_declared_paths(manifest: dict[str, object]) -> list[str]:
    paths: list[str] = []
    paths.extend(manifest.get("implementation", {}).get("paths", []))
    paths.extend(manifest.get("userguide", {}).get("pages", []))
    paths.extend(manifest.get("userguide", {}).get("screenshots", []))
    paths.extend(manifest.get("tests", {}).get("playwright", []))
    paths.extend(manifest.get("tests", {}).get("pytest", []))
    paths.extend(manifest.get("tests", {}).get("testthat", []))
    paths.extend(manifest.get("release", {}).get("paths", []))
    for key in ("proposal", "design", "tasks"):
        value = manifest.get("openspec", {}).get(key)
        if value:
            paths.append(value)
    paths.extend(manifest.get("openspec", {}).get("specs", []))
    return [path for path in paths if path]


def normalize(text: str) -> list[str]:
    return TOKEN_PATTERN.findall(text.lower())


def load_test_titles(paths: Iterable[str]) -> list[str]:
    titles: list[str] = []
    for raw_path in paths:
        path = REPO_ROOT / raw_path
        if not path.exists():
            continue
        try:
            content = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        titles.extend(match.group("title") for match in TEST_TITLE_PATTERN.finditer(content))
        titles.extend(
            match.group(1).replace("test_", "").replace("_", " ")
            for match in PYTEST_TITLE_PATTERN.finditer(content)
        )
        titles.extend(match.group("title") for match in TESTTHAT_TITLE_PATTERN.finditer(content))
    return titles


def title_matches(requirement: str, titles: list[str]) -> bool:
    required_tokens = set(normalize(requirement))
    if not required_tokens:
        return False

    for title in titles:
        title_tokens = set(normalize(title))
        if not title_tokens:
            continue
        intersection = required_tokens & title_tokens
        if len(intersection) >= min(3, len(required_tokens)):
            return True
        coverage = len(intersection) / max(len(required_tokens), 1)
        if coverage >= 0.6:
            return True
    return False


def mtime(path_str: str) -> float | None:
    path = REPO_ROOT / path_str
    if not path.exists():
        return None
    return path.stat().st_mtime


def check_manifest(path: Path, *, mirror_repo: bool = False) -> ManifestResult:
    manifest = load_manifest(path)
    result = ManifestResult(path=rel(path), errors=[], warnings=[])

    declared_paths = collect_declared_paths(manifest)
    for declared_path in declared_paths:
        if path_exists(declared_path):
            continue
        message = f"Declared path does not exist: {declared_path}"
        if mirror_repo:
            result.warnings.append(message)
        else:
            result.errors.append(message)

    test_paths = (
        manifest.get("tests", {}).get("playwright", [])
        + manifest.get("tests", {}).get("pytest", [])
        + manifest.get("tests", {}).get("testthat", [])
    )
    titles = load_test_titles(test_paths)
    for requirement in manifest.get("tests", {}).get("coverage_must_test", []):
        if not title_matches(requirement, titles):
            result.warnings.append(
                f"Coverage requirement not matched by any test title: {requirement}"
            )

    impl_mtimes = [mtime(item) for item in manifest.get("implementation", {}).get("paths", [])]
    doc_mtimes = [mtime(item) for item in manifest.get("userguide", {}).get("pages", [])]
    impl_mtimes = [item for item in impl_mtimes if item is not None]
    doc_mtimes = [item for item in doc_mtimes if item is not None]
    if impl_mtimes and doc_mtimes and max(impl_mtimes) > min(doc_mtimes):
        result.warnings.append(
            "User guide may be stale: at least one implementation file is newer than the guide."
        )

    if manifest.get("status") == "merged" and not manifest.get("git", {}).get("pr_url"):
        result.warnings.append("Merged feature has no PR URL recorded.")

    return result


def emit_markdown(results: list[ManifestResult]) -> str:
    lines = [
        "# Feature Gate Summary",
        "",
        f"- Repository: `{REPO_ROOT.name}`",
        f"- Checked manifests: `{len(results)}`",
        "",
    ]
    for item in results:
        status = "OK" if item.ok else "FAIL"
        lines.append(f"## {status} - `{item.path}`")
        if not item.errors and not item.warnings:
            lines.append("- No issues found.")
        for error in item.errors:
            lines.append(f"- Error: {error}")
        for warning in item.warnings:
            lines.append(f"- Warning: {warning}")
        lines.append("")
    return "\n".join(lines)


def write_github_step_summary(markdown: str) -> None:
    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary_path:
        Path(summary_path).write_text(markdown + "\n", encoding="utf-8")


def main() -> int:
    args = build_parser().parse_args()
    manifest_paths = discover_manifests(args.paths)
    if not manifest_paths:
        print("No feature manifests found.", file=sys.stderr)
        return 0

    results = [check_manifest(path, mirror_repo=args.mirror_repo) for path in manifest_paths]
    has_errors = any(item.errors for item in results)
    markdown = emit_markdown(results)
    write_github_step_summary(markdown)

    if args.json:
        payload = {
            "ok": not has_errors,
            "results": [
                {
                    "path": item.path,
                    "errors": item.errors,
                    "warnings": item.warnings,
                }
                for item in results
            ],
            "markdown": markdown,
        }
        print(json.dumps(payload, indent=2))
    else:
        print(markdown)

    return 1 if args.strict and has_errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
