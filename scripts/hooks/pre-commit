#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"

if [ -f "${REPO_ROOT}/scripts/feature_lifecycle/validate_feature_manifests.py" ]; then
  python3 "${REPO_ROOT}/scripts/feature_lifecycle/validate_feature_manifests.py" || \
    echo "[pre-commit] warning: feature manifest validation failed." >&2
fi

if command -v gitleaks >/dev/null 2>&1; then
  gitleaks protect --staged --redact >/dev/null 2>&1 || \
    echo "[pre-commit] warning: potential secret detected by gitleaks." >&2
fi

exit 0
