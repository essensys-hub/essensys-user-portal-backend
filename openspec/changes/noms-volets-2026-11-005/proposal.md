> Ticket : [essensys-hub/essensys-feature-lifecycle#52](https://github.com/essensys-hub/essensys-feature-lifecycle/issues/52). Feature ID : `noms-volets-2026-11-005`. Sub-issue de ce dépôt : [essensys-hub/essensys-user-portal-backend#57](https://github.com/essensys-hub/essensys-user-portal-backend/issues/57).
> Feature répartie sur plusieurs dépôts ; le change commun (proposal, design, contrat d'API, specs des autres surfaces) est dans `essensys-feature-lifecycle/openspec/changes/noms-volets-2026-11-005`. Ce change ne décrit que le code de **essensys-user-portal-backend**.

## Why

Permettre à l'administrateur de l'armoire de renommer chacun des 16 volets et stores (« Volet 1 Salon » devient « Volet baie vitrée »), avec un nom partagé entre le cloud, la gateway, le portail et les apps. Le cloud est l'arbitre : il stocke les noms de l'armoire et fusionne ceux de la gateway. Cette Feature est le jumeau de `noms-lumieres-2026-11-004`.

## What Changes

- Catalogue des 16 volets et stores (`id`, nom d'origine, ordre) dans `internal/domain/shutters_names.go` ; la règle du nom (rogné, 1 à 40 caractères, sans contrôle ni `<` `>`), la fusion et la borne d'horloge sont celles des lumières.
- Migration idempotente `020_shutter_names.sql` (table `shutter_names`) et magasin dans `internal/data/shutter_names_store.go`. La table `light_names` n'est pas modifiée.
- `GET /api/portal/shutters/names` : les 16 volets de l'armoire liée, `name` à `null` par défaut.
- `PUT /api/portal/admin/shutters/names/{id}` : réservé à `admin_local` (ou `admin_global`) avec armoire liée, version incrémentée, audit `shutter_name`, dry-run.
- `POST /api/gateway/shutters/names/sync` (`GatewayAuth`) : fusion par `(updated_at, version, updated_by)`, futur borné à 5 minutes, réponse = état complet.

## Capabilities

### New Capabilities

- `shutter-names-cloud` : lecture, écriture réservée, validation, migration, synchronisation avec la gateway, mode test.

## Impact

- **Code** : `internal/domain/shutters_names.go`, `internal/data/shutter_names_store.go`, `internal/handlers/shutter_names.go`, `internal/portal/routes.go`, `internal/gateway/routes.go`, `migrations/020_shutter_names.sql`.
- **Table d'échange** : aucune clé touchée, aucune écriture dans l'armoire. Endpoints legacy et routes des lumières inchangés.
- **Dépendances** : aucune nouvelle.
