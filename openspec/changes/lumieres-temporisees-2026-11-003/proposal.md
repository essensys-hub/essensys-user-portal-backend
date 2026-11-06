> Ticket : [essensys-hub/essensys-feature-lifecycle#45](https://github.com/essensys-hub/essensys-feature-lifecycle/issues/45). Feature ID : `lumieres-temporisees-2026-11-003`. Sub-issue de ce dépôt : [essensys-hub/essensys-user-portal-backend#53](https://github.com/essensys-hub/essensys-user-portal-backend/issues/53).
> Feature répartie sur plusieurs dépôts ; le change commun (proposal, design, contrat d'API, specs des autres surfaces) est dans `essensys-feature-lifecycle/openspec/changes/lumieres-temporisees-2026-11-003`. Ce change ne décrit que le code de **essensys-user-portal-backend**.

## Why

Régler le délai d'extinction automatique de six lumières (Dressing, WC 1, WC 2, Service, Annexe 1, Annexe 2) depuis les apps et le portail cloud, pour l'administrateur de l'armoire seulement. Aucune route ne valide aujourd'hui la clé, la plage ni le rôle.

## What Changes

- Catalogue `{id, libellé, clé}` (521, 555, 556, 557, 561, 562) et validation (entier 0 à 255) dans `internal/domain/lights_timers.go`.
- `GET /api/portal/lights/timers` : les six valeurs depuis le cache gateway, `null` si jamais lue.
- `PUT /api/portal/admin/lights/timers/{id}` : réservé à `admin_local` (ou `admin_global`) avec armoire liée, file d'actions, cache, audit, dry-run.
- Migration idempotente `018_lights_timers_sync_profile.sql` (profil de synchro, pull et push).

## Capabilities

### New Capabilities

- `light-timers-cloud` : lecture, écriture réservée, validation, remontée des valeurs, mode test.

## Impact

- **Code** : `internal/domain/lights_timers.go`, `internal/handlers/light_timers.go`, `internal/portal/routes.go`, `migrations/018_lights_timers_sync_profile.sql`.
- **Table d'échange** : clés 521, 555-557, 561-562, hors 605-623 (aucune expansion ni coalescence). Endpoints legacy inchangés.
- **Dépendances** : aucune nouvelle.
