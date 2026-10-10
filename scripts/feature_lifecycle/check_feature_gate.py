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
JUNIT_TITLE_PATTERN = re.compile(
    r"@Test\b[\s\S]{0,300}?\b(?:fun|func|void)\s+`?(?P<title>[A-Za-z_][A-Za-z0-9_ ]*)`?\s*\(",
)
# XCTest (Swift/Objective-C) : méthodes `func test…()` sans annotation.
XCTEST_TITLE_PATTERN = re.compile(r"^\s*(?:@MainActor\s+)?func\s+test_?(?P<title>[A-Za-z0-9_]+)\s*\(", re.MULTILINE)
# Go : `func TestXxx(t *testing.T)` ; le CamelCase est découpé en mots.
GO_TEST_TITLE_PATTERN = re.compile(r"^func\s+Test(?P<title>[A-Za-z0-9_]+)\s*\(\s*\w+\s+\*testing\.T\s*\)", re.MULTILINE)
CAMEL_BOUNDARY = re.compile(r"(?<=[a-z0-9])(?=[A-Z])|(?<=[A-Z])(?=[A-Z][a-z])")
TOKEN_PATTERN = re.compile(r"[a-z0-9]+")
MANDATORY_UX_DEVICES = {"desktop", "iphone", "ipad"}
UI_SURFACE_VALUES = {"react-user", "react-admin", "mixed"}
# Apps natives : la matrice UX web (Playwright desktop/iPhone/iPad) ne s'applique pas ; la preuve est
# portée par les tests instrumentés (émulateur / simulateur) et les captures du manifest.
NATIVE_MOBILE_SURFACES = {"android", "ios"}
# Surfaces sans interface : le nom du dépôt (ex. « user-portal-backend ») ne doit pas faire croire à une UI.
NON_UI_SURFACES = {"api", "cli-node", "cli-go", "ci", "ops"}
UI_PATH_HINTS = (
    "src/pages/",
    "src/components/",
    "tests/e2e/",
    "e2e/",
    "playwright",
    "frontend",
    "portal",
    ".tsx",
    ".jsx",
    ".css",
    ".scss",
)
DOMOTIC_HINTS = (
    "armoire",
    "api/admin/inject",
    "api/portal/inject",
    "myactions",
    "table-d-echange",
    "table_echange",
    "scenario",
    "scenarios",
    "shutter",
    "volet",
    "lighting",
    "gateway",
    "server-frontend",
    "user-portal-frontend",
    "essensys-server-frontend",
    "essensys-user-portal-frontend",
)


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
    paths.extend(manifest.get("tests", {}).get("junit", []))
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
        titles.extend(
            match.group("title").replace("_", " ") for match in JUNIT_TITLE_PATTERN.finditer(content)
        )
        titles.extend(
            match.group("title").replace("_", " ") for match in XCTEST_TITLE_PATTERN.finditer(content)
        )
        titles.extend(go_title(match.group("title")) for match in GO_TEST_TITLE_PATTERN.finditer(content))
    return titles


def go_title(name: str) -> str:
    """`NR_backend_3_sixthReportIs429` → `NR backend 3 sixth Report Is 429`."""
    return CAMEL_BOUNDARY.sub(" ", name).replace("_", " ")



def read_existing_text(paths: Iterable[str]) -> str:
    chunks: list[str] = []
    for raw_path in paths:
        path = REPO_ROOT / raw_path
        if not path.exists():
            continue
        try:
            chunks.append(path.read_text(encoding="utf-8"))
        except UnicodeDecodeError:
            continue
    return "\n".join(chunks)


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



def list_value(manifest: dict[str, object], *keys: str) -> list[str]:
    value: object = manifest
    for key in keys:
        if not isinstance(value, dict):
            return []
        value = value.get(key, [])
    return value if isinstance(value, list) else []


def string_value(manifest: dict[str, object], *keys: str) -> str | None:
    value: object = manifest
    for key in keys:
        if not isinstance(value, dict):
            return None
        value = value.get(key)
    return value if isinstance(value, str) else None


def is_ui_feature(manifest: dict[str, object]) -> bool:
    primary = string_value(manifest, "implementation", "primary_surface")
    if primary in NATIVE_MOBILE_SURFACES or primary in NON_UI_SURFACES:
        return False
    if primary in UI_SURFACE_VALUES:
        return True
    text_parts: list[str] = []
    text_parts.extend(list_value(manifest, "implementation", "paths"))
    text_parts.extend(list_value(manifest, "tests", "playwright"))
    text_parts.extend(list_value(manifest, "userguide", "screenshots"))
    text_parts.extend(list_value(manifest, "release", "surfaces"))
    haystack = "\n".join(text_parts).lower()
    return any(hint in haystack for hint in UI_PATH_HINTS)


def is_domotic_ui_feature(manifest: dict[str, object]) -> bool:
    text_parts: list[str] = []
    text_parts.extend(list_value(manifest, "implementation", "paths"))
    text_parts.extend(list_value(manifest, "tests", "playwright"))
    text_parts.extend(list_value(manifest, "release", "surfaces"))
    text_parts.extend(list_value(manifest, "tests", "coverage_must_test"))
    haystack = "\n".join(text_parts).lower()
    return any(hint in haystack for hint in DOMOTIC_HINTS)


def device_coverage_from_projects(projects: Iterable[str]) -> set[str]:
    found: set[str] = set()
    for project in projects:
        p = project.lower()
        for device in MANDATORY_UX_DEVICES:
            if device in p:
                found.add(device)
    return found


def annotated_devices(text: str) -> set[str]:
    found: set[str] = set()
    lower = text.lower()
    for device in MANDATORY_UX_DEVICES:
        patterns = (
            f"@device: {device}",
            f"@device={device}",
            f"@{device}",
            f"-{device}",
            f" {device}",
        )
        if any(pattern in lower for pattern in patterns):
            found.add(device)
    if "@devices:" in lower:
        for device in MANDATORY_UX_DEVICES:
            if device in lower:
                found.add(device)
    return found


def check_ux_matrix(manifest: dict[str, object], result: ManifestResult, *, mirror_repo: bool = False) -> None:
    if not is_ui_feature(manifest):
        return

    tests = manifest.get("tests", {})
    tests = tests if isinstance(tests, dict) else {}
    ux_matrix = tests.get("ux_matrix")
    if not isinstance(ux_matrix, dict):
        result.errors.append(
            "UI feature must declare tests.ux_matrix with desktop, iphone and ipad coverage."
        )
        return

    if ux_matrix.get("required") is not True:
        result.errors.append("UI feature tests.ux_matrix.required must be true.")

    devices = set(ux_matrix.get("devices", [])) if isinstance(ux_matrix.get("devices"), list) else set()
    missing_devices = sorted(MANDATORY_UX_DEVICES - devices)
    if missing_devices:
        result.errors.append(
            "UI feature tests.ux_matrix.devices missing mandatory devices: "
            + ", ".join(missing_devices)
        )

    projects = ux_matrix.get("required_projects", [])
    projects = projects if isinstance(projects, list) else []
    project_devices = device_coverage_from_projects(projects)
    missing_project_devices = sorted(MANDATORY_UX_DEVICES - project_devices)
    if missing_project_devices:
        result.errors.append(
            "UI feature tests.ux_matrix.required_projects must include projects covering: "
            + ", ".join(missing_project_devices)
        )

    if ux_matrix.get("screenshots_required") is not True:
        result.errors.append("UI feature tests.ux_matrix.screenshots_required must be true.")
    if ux_matrix.get("visual_regression_required") is not True:
        result.errors.append("UI feature tests.ux_matrix.visual_regression_required must be true.")
    if is_domotic_ui_feature(manifest) and ux_matrix.get("no_armoire_required") is not True:
        result.errors.append("ESSENSYS/domotic UI feature must set tests.ux_matrix.no_armoire_required=true.")

    playwright_paths = list_value(manifest, "tests", "playwright")
    if not playwright_paths:
        result.errors.append("UI feature must declare at least one tests.playwright spec path.")
        return

    existing_paths = [path for path in playwright_paths if path_exists(path)]
    if not existing_paths:
        message = "UX matrix annotation check skipped because no declared Playwright spec exists in this repository."
        if mirror_repo:
            result.warnings.append(message)
        else:
            result.errors.append(message)
        return

    text = read_existing_text(playwright_paths)
    evidence_devices = annotated_devices(text) | project_devices
    missing_evidence = sorted(MANDATORY_UX_DEVICES - evidence_devices)
    if missing_evidence:
        result.errors.append(
            "Declared Playwright specs/projects do not evidence mandatory UX devices: "
            + ", ".join(missing_evidence)
            + "; add @device/@devices annotations or matching project names."
        )

    ux_evidence = tests.get("ux_evidence")
    if isinstance(ux_evidence, dict) and ux_evidence.get("status") == "passed":
        validated = set(ux_evidence.get("devices_validated", [])) if isinstance(ux_evidence.get("devices_validated"), list) else set()
        missing_validated = sorted(MANDATORY_UX_DEVICES - validated)
        if missing_validated:
            result.errors.append(
                "tests.ux_evidence.status=passed but devices_validated is missing: "
                + ", ".join(missing_validated)
            )


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
        + manifest.get("tests", {}).get("junit", [])
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

    check_ux_matrix(manifest, result, mirror_repo=mirror_repo)
    if string_value(manifest, "implementation", "primary_surface") in NATIVE_MOBILE_SURFACES:
        if not list_value(manifest, "tests", "junit"):
            result.errors.append("Native mobile feature must declare tests.junit (unit + instrumented tests).")
        if not list_value(manifest, "userguide", "screenshots"):
            result.warnings.append("Native mobile feature has no userguide.screenshots (emulator/simulator captures).")

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
