#!/usr/bin/env python3
"""Aggregate security gate reports and determine the blocking status."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


SEVERITY_ORDER = ("critical", "high", "medium", "low")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Summarize Security Gate artifacts and decide if the PR must be blocked."
    )
    parser.add_argument(
        "--artifacts-dir",
        default=".artifacts/security-gate",
        help="Directory containing JSON reports emitted by security-gate.",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit machine-readable JSON.",
    )
    return parser


def read_json(path: Path) -> object:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def count_gitleaks(path: Path) -> tuple[int, list[str]]:
    if not path.exists():
        return 0, []
    payload = read_json(path)
    findings = payload if isinstance(payload, list) else payload.get("findings", [])
    messages = []
    for finding in findings[:10]:
        rule = finding.get("RuleID") or finding.get("Description") or "secret"
        file_name = finding.get("File") or "<unknown>"
        messages.append(f"{rule} in {file_name}")
    return len(findings), messages


def count_npm_audit(path: Path) -> tuple[dict[str, int], list[str]]:
    counts = {key: 0 for key in SEVERITY_ORDER}
    details: list[str] = []
    if not path.exists():
        return counts, details

    payload = read_json(path)
    vulnerabilities = payload.get("metadata", {}).get("vulnerabilities", {})
    for key in SEVERITY_ORDER:
        counts[key] = int(vulnerabilities.get(key, 0) or 0)

    vuln_map = payload.get("vulnerabilities", {})
    for package_name, data in list(vuln_map.items())[:10]:
        severity = str(data.get("severity") or "unknown").lower()
        if severity in {"critical", "high"}:
            details.append(f"{package_name} ({severity})")
    return counts, details


def count_pip_audit(path: Path) -> tuple[dict[str, int], list[str]]:
    counts = {key: 0 for key in SEVERITY_ORDER}
    details: list[str] = []
    if not path.exists():
        return counts, details

    payload = read_json(path)
    dependencies = payload.get("dependencies", [])
    for dependency in dependencies:
        for vuln in dependency.get("vulns", []):
            severity = (
                str(vuln.get("severity") or vuln.get("fix_versions") or "high")
                .lower()
            )
            if severity not in counts:
                severity = "high"
            counts[severity] += 1
            if severity in {"critical", "high"} and len(details) < 10:
                details.append(
                    f"{dependency.get('name', '<unknown>')} ({vuln.get('id', 'unknown')})"
                )
    return counts, details


def count_trivy(path: Path) -> tuple[dict[str, int], list[str]]:
    """Parse a Trivy JSON report (fs scan): CVEs + misconfigurations."""
    counts = {key: 0 for key in SEVERITY_ORDER}
    details: list[str] = []
    if not path.exists():
        return counts, details

    payload = read_json(path)
    results = payload.get("Results", []) if isinstance(payload, dict) else []
    for result in results:
        target = result.get("Target", "<unknown>")
        for vuln in result.get("Vulnerabilities", []) or []:
            severity = str(vuln.get("Severity") or "unknown").lower()
            if severity not in counts:
                continue
            counts[severity] += 1
            if severity in {"critical", "high"} and len(details) < 10:
                details.append(
                    f"{vuln.get('VulnerabilityID', 'CVE')} in "
                    f"{vuln.get('PkgName', target)} ({severity})"
                )
        for misc in result.get("Misconfigurations", []) or []:
            severity = str(misc.get("Severity") or "unknown").lower()
            if severity not in counts:
                continue
            counts[severity] += 1
            if severity in {"critical", "high"} and len(details) < 10:
                details.append(
                    f"{misc.get('ID', 'misconfig')} in {target} ({severity})"
                )
    return counts, details


def count_bandit(path: Path) -> tuple[int, list[str]]:
    if not path.exists():
        return 0, []
    payload = read_json(path)
    findings = [
        item
        for item in payload.get("results", [])
        if str(item.get("issue_severity", "")).upper() == "HIGH"
    ]
    messages = [
        f"{item.get('test_name', 'bandit')} in {item.get('filename', '<unknown>')}:{item.get('line_number', '?')}"
        for item in findings[:10]
    ]
    return len(findings), messages


def count_eslint(path: Path) -> tuple[int, list[str]]:
    if not path.exists():
        return 0, []
    payload = read_json(path)
    findings: list[str] = []
    error_count = 0
    for file_entry in payload:
        for message in file_entry.get("messages", []):
            if int(message.get("severity", 0) or 0) == 2:
                error_count += 1
                if len(findings) < 10:
                    findings.append(
                        f"{message.get('ruleId', 'eslint')} in {file_entry.get('filePath', '<unknown>')}:{message.get('line', '?')}"
                    )
    return error_count, findings


def markdown(summary: dict[str, object]) -> str:
    lines = [
        "<!-- security-gate -->",
        "# Security Gate",
        "",
        f"- Blocking findings: **{summary['blocking_findings']}**",
        f"- Secrets detected: **{summary['secret_count']}**",
        f"- Dependency Critical: **{summary['dependency_counts']['critical']}**",
        f"- Dependency High: **{summary['dependency_counts']['high']}**",
        f"- Trivy Critical: **{summary['trivy_counts']['critical']}**",
        f"- Trivy High: **{summary['trivy_counts']['high']}**",
        f"- Bandit High: **{summary['bandit_high']}**",
        f"- ESLint security errors: **{summary['eslint_errors']}**",
        "",
    ]

    if summary["blocking_findings"]:
        lines.append("> This PR is blocked until Critical/High findings are fixed or formally triaged.")
    else:
        lines.append("> No blocking security finding detected by the local security gate.")
    lines.append("")

    for section, items in (
        ("Secret scan", summary["secret_details"]),
        ("Dependency audit", summary["dependency_details"]),
        ("Trivy (CVE / IaC)", summary["trivy_details"]),
        ("Bandit", summary["bandit_details"]),
        ("ESLint security", summary["eslint_details"]),
    ):
        lines.append(f"## {section}")
        if not items:
            lines.append("- No notable finding.")
        else:
            for item in items:
                lines.append(f"- {item}")
        lines.append("")

    return "\n".join(lines)


def main() -> int:
    args = build_parser().parse_args()
    artifacts_dir = Path(args.artifacts_dir)

    secret_count, secret_details = count_gitleaks(artifacts_dir / "gitleaks.json")
    npm_counts, npm_details = count_npm_audit(artifacts_dir / "npm-audit.json")
    pip_counts, pip_details = count_pip_audit(artifacts_dir / "pip-audit.json")
    trivy_counts, trivy_details = count_trivy(artifacts_dir / "trivy.json")
    bandit_high, bandit_details = count_bandit(artifacts_dir / "bandit.json")
    eslint_errors, eslint_details = count_eslint(artifacts_dir / "eslint-security.json")

    dependency_counts = {
        key: npm_counts.get(key, 0) + pip_counts.get(key, 0) for key in SEVERITY_ORDER
    }
    dependency_details = npm_details + [item for item in pip_details if item not in npm_details]

    blocking_findings = (
        secret_count
        + dependency_counts["critical"]
        + dependency_counts["high"]
        + trivy_counts["critical"]
        + trivy_counts["high"]
        + bandit_high
        + eslint_errors
    )

    summary = {
        "blocking_findings": blocking_findings,
        "secret_count": secret_count,
        "secret_details": secret_details,
        "dependency_counts": dependency_counts,
        "dependency_details": dependency_details[:10],
        "trivy_counts": trivy_counts,
        "trivy_details": trivy_details,
        "bandit_high": bandit_high,
        "bandit_details": bandit_details,
        "eslint_errors": eslint_errors,
        "eslint_details": eslint_details,
        "markdown": markdown(
            {
                "blocking_findings": blocking_findings,
                "secret_count": secret_count,
                "dependency_counts": dependency_counts,
                "secret_details": secret_details,
                "dependency_details": dependency_details[:10],
                "trivy_counts": trivy_counts,
                "trivy_details": trivy_details,
                "bandit_high": bandit_high,
                "bandit_details": bandit_details,
                "eslint_errors": eslint_errors,
                "eslint_details": eslint_details,
            }
        ),
    }

    if args.json:
        print(json.dumps(summary, indent=2))
    else:
        print(summary["markdown"])

    return 1 if blocking_findings else 0


if __name__ == "__main__":
    raise SystemExit(main())
