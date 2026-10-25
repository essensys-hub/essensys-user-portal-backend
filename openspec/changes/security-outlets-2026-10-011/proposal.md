> Ticket : [essensys-hub/essensys-feature-lifecycle#36](https://github.com/essensys-hub/essensys-feature-lifecycle/issues/36). Feature ID : `security-outlets-2026-10-011`.
> Feature répartie sur 6 dépôts, un change du même nom dans chacun : apps Android et iPhone, portail cloud (`essensys-user-portal-frontend` + `essensys-user-portal-backend`), portail local (`essensys-server-frontend` + `essensys-server-backend`). Ce change ne décrit que le code de **essensys-user-portal-backend**.

## Why

Les utilisateurs demandent à couper et à rétablir les **prises de sécurité** à distance, et à savoir si elles sont coupées. Aucune interface ne le permet aujourd'hui, ni l'ancienne ASP.NET (son `DoActions` ignorait ces prises). Le besoin était déjà décrit dans essensys-hub/essensys-raspberry-install#5, clos le 2026-10-09 avec les tickets d'avant la production.

**Table d'échange** (enum de `client-essensys-legacy/H/TableEchange.h` compilé, recoupé avec `Cumulus_Mode` = 353, `Arrose_Mode` = 363 et `Chauf_zj_Mode` = 349) :

| Rôle | Champ firmware | Indice | Valeurs |
|---|---|---|---|
| Commande | `Scenario_Securite`, décalage 31 du bloc scénario | **623** dans le scénario 1 (592–632), puis déclenchement `590 = 1` | 0 = ne rien faire, 1 = couper, 2 = rétablir |
| État | `Securite_PriseCoupe` | **440** | 1 = coupées, 0 = pilotées |

La doc `exchange-table.md` indique 631 : c'est l'ancienne base 600, avec le même décalage de 8 que pour les lumières.

## What Changes

- `ExpandLegacyScenarioBlock` traite 623 comme l'éclairage (590 ajouté, 605–622 et 623 complétés à 0).
- Le regroupement des actions (`IsPureBlockAction`, `MergeBlockActions`) accepte `623 = 0` et ne fusionne jamais une coupure ou un rétablissement.
- `/api/portal/exchange` renvoie 440 dès que la gateway le pousse (aucun changement des endpoints legacy).

## Capabilities

### New Capabilities

- `security-outlet-command` : complément du bloc scénario 1 avec 623.
- `security-outlet-merge` : regroupement compatible.
- `security-outlet-state` : lecture de 440 depuis le cache.
- `security-outlets-quality` : tests, checkup, validation.

### Modified Capabilities

Aucune (les specs existantes de ce dépôt ne sont pas archivées sous `openspec/specs/`).

## Impact

- **Code** : `internal/domain/order_expansion.go`, `internal/domain/merge_params.go`, `internal/data/store.go` (si besoin), tests Go `NR-backend-10…12`.
- **Table d'échange** : aucun indice modifié ; utilisation de 590, 605–623 (commande) et 440 (état).
- **Dépendances** : aucune nouvelle.
