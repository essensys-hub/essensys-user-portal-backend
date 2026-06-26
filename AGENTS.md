# Agent Context — Feature Lifecycle

Ce dépôt utilise le **feature lifecycle Essensys** (Git-first, IA orchestratrice). Trois contraintes non négociables : **sécurité**, **open source**, **traçabilité complète**.

- `features/<id>.json` est la source de vérité d'une feature
- OpenSpec, docs (`docs/features/`), tests et release notes pointent vers le manifest
- `feature-gate` et `security-gate` tournent automatiquement sur les PRs

## Process

```
Idée → Jira (SCRUM) → OpenSpec → Tasks Jira → Code → Test×N → Gate sécurité → Deploy → Revue
         └────────  Documentation · Mémoire (essensys-memory)  en continu ────────┘
```

Backlog Jira : <https://essensys-hub.atlassian.net/jira/software/projects/SCRUM/boards/1/backlog>

## Gate sécurité (open source, bloquante)

- **Secrets** : gitleaks (`.gitleaks.toml`)
- **CVE / SCA + IaC** : Trivy (deps Go/npm/pip + Docker/IaC) + Dependabot (`.github/dependabot.yml`)
- Critical/High = bloquant. Secrets jamais mutés (scrub historique + rotation).
- Triage : skill `security-gate-triage`. Pas de scanner propriétaire (GitGuardian = dashboard secrets optionnel).

## Secrets & SOPS

Aucun secret en clair. Tous chiffrés avec SOPS + age dans `essensys-ansible/secrets/`. Extraction en variable d'env éphémère uniquement (cf. `essensys-feature-lifecycle/AGENTS.md`).

## Installer les skills

```bash
git clone https://github.com/essensys-hub/essensys-feature-lifecycle.git
cd essensys-feature-lifecycle
./scripts/install-skills.sh --global
```

## Prompts recommandés

```text
new feature SCRUM-123
sync user guide for this feature
generate e2e tests for this feature
triage security findings
```
