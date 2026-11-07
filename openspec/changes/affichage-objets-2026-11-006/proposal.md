> Ticket : [essensys-hub/essensys-feature-lifecycle#55](https://github.com/essensys-hub/essensys-feature-lifecycle/issues/55). Feature ID : `affichage-objets-2026-11-006`. Sub-issue de ce dépôt : [essensys-hub/essensys-user-portal-backend#59](https://github.com/essensys-hub/essensys-user-portal-backend/issues/59).
> Feature répartie sur plusieurs dépôts ; le change commun (proposal, design, contrat d'API, specs des autres surfaces) est dans `essensys-feature-lifecycle/openspec/changes/affichage-objets-2026-11-006`. Ce change ne décrit que le code de **essensys-user-portal-backend**.

## Why

Toutes les installations n'ont pas les mêmes équipements : l'administrateur de l'armoire doit pouvoir désactiver des lumières ou des volets et choisir leur ordre d'affichage. Le cloud est l'arbitre : il stocke la disposition de l'armoire, un document par genre, et fusionne celui de la gateway. Cette Feature est la cousine de `noms-lumieres-2026-11-004` et `noms-volets-2026-11-005`.

## What Changes

- Modèle `ObjectLayout`, validation (identifiants du catalogue du genre, pas de doublon, `unknown_light` / `unknown_shutter` / `invalid_layout`), fonction `EffectiveLayout`, droits et fusion repris des noms : `internal/domain/object_layout.go`. Les catalogues des noms ne sont pas modifiés.
- Migration idempotente `021_object_layout.sql` (table `object_layout`, clé `(machine_id, kind)`) et magasin `internal/data/object_layout_store.go`. Les tables `light_names` et `shutter_names` ne sont pas touchées.
- `GET /api/portal/{lights|shutters}/layout` : document du genre pour l'armoire liée, sans `updated_by`, défaut `version` 0 et `updated_at` nul.
- `PUT /api/portal/admin/{lights|shutters}/layout` : réservé à `admin_local` (ou `admin_global`) avec armoire liée, version `n+1`, `{"reset":true}`, audit `object_layout`, dry-run.
- `POST /api/gateway/{lights|shutters}/layout/sync` (`GatewayAuth`) : fusion du document entier par `(updated_at, version, updated_by)`, futur borné à 5 minutes, réponse = état fusionné avec `updated_by`.

## Capabilities

### New Capabilities

- `object-layout-cloud` : lecture, écriture réservée, validation, migration, synchronisation avec la gateway, mode test.

## Impact

- **Code** : `internal/domain/object_layout.go`, `internal/data/object_layout_store.go`, `internal/handlers/object_layout.go`, `internal/portal/routes.go`, `internal/gateway/routes.go`, `migrations/021_object_layout.sql`.
- **Table d'échange** : aucune clé touchée, aucune écriture dans l'armoire. Endpoints legacy, routes et tables des noms inchangés.
- **Dépendances** : aucune nouvelle.
