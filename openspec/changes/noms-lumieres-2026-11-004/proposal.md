> Ticket : [essensys-hub/essensys-feature-lifecycle#41](https://github.com/essensys-hub/essensys-feature-lifecycle/issues/41). Feature ID : `noms-lumieres-2026-11-004`. Sub-issue de ce dépôt : [essensys-hub/essensys-user-portal-backend#55](https://github.com/essensys-hub/essensys-user-portal-backend/issues/55).
> Feature répartie sur plusieurs dépôts ; le change commun (proposal, design, contrat d'API, specs des autres surfaces) est dans `essensys-feature-lifecycle/openspec/changes/noms-lumieres-2026-11-004`. Ce change ne décrit que le code de **essensys-user-portal-backend**.

## Why

Permettre à l'administrateur de l'armoire de renommer chacune des 33 lumières (« Dressing » devient « WC 3 »), avec un nom partagé entre le cloud, la gateway, le portail et les apps. Le cloud est l'arbitre : il stocke les noms de l'armoire et fusionne ceux de la gateway.

## What Changes

- Catalogue des 33 lumières (`id`, nom d'origine, ordre) et règle du nom (rogné, 1 à 40 caractères, sans contrôle ni `<` `>`) dans `internal/domain/lights_names.go`.
- Migration idempotente `019_light_names.sql` (table `light_names`) et magasin dans `internal/data/light_names_store.go`.
- `GET /api/portal/lights/names` : les 33 lumières de l'armoire liée, `name` à `null` par défaut.
- `PUT /api/portal/admin/lights/names/{id}` : réservé à `admin_local` (ou `admin_global`) avec armoire liée, version incrémentée, audit `light_name`, dry-run.
- `POST /api/gateway/lights/names/sync` (`GatewayAuth`) : fusion par `(updated_at, version, updated_by)`, futur borné à 5 minutes, réponse = état complet.

## Capabilities

### New Capabilities

- `light-names-cloud` : lecture, écriture réservée, validation, migration, synchronisation avec la gateway, mode test.

## Impact

- **Code** : `internal/domain/lights_names.go`, `internal/data/light_names_store.go`, `internal/handlers/light_names.go`, `internal/portal/routes.go`, `internal/gateway/routes.go`, `migrations/019_light_names.sql`.
- **Table d'échange** : aucune clé touchée, aucune écriture dans l'armoire. Endpoints legacy inchangés.
- **Dépendances** : aucune nouvelle.
