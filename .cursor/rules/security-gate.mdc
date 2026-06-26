---
description: Block secrets and Critical or High security findings on PRs.
globs:
  - "**/package.json"
  - "**/requirements*.txt"
  - "**/Dockerfile"
  - ".github/workflows/**/*.y*ml"
  - ".github/dependabot.yml"
  - "features/*.json"
alwaysApply: false
---

# Security Gate

- SCA is **open source**: **Trivy** (CVE deps Go/npm/pip + Docker/IaC) + Dependabot alerts + the `security-gate` workflow. No proprietary scanner.
- Secrets: **gitleaks** (open source) is the blocking layer. Never commit secrets.
- Treat Critical and High findings as blocking.
- A mute is a Dependabot alert dismissal (or a documented Trivy `.trivyignore` entry) recorded in `security.muted_findings[]` with a real `justification_url`. Secrets are never muted.
