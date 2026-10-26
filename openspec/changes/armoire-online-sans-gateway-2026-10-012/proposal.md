> Ticket : [essensys-hub/essensys-feature-lifecycle#37](https://github.com/essensys-hub/essensys-feature-lifecycle/issues/37) (Bug). Feature ID : `armoire-online-sans-gateway-2026-10-012`.
> Correctif réparti sur 4 dépôts, un change du même nom dans chacun : backend cloud (`essensys-user-portal-backend`), portail cloud (`essensys-user-portal-frontend`), apps Android et iPhone. Ce change ne décrit que le code de **essensys-user-portal-backend**.

## Why

Signalement du 2026-10-26 d'un utilisateur pilote dont l'installation **n'a pas de gateway** : l'app Android 2.1.1 affiche « Armoire hors ligne » et désactive toutes les commandes, alors que son armoire dialogue bien avec le cloud (« dernier contact » à la minute près dans Réglages).

Cause : `GET /api/portal/session` ne calcule `online` que pour la **gateway** (`IsGatewayOnline`, `gateway_sessions.last_seen` de moins de 2 min), et le bloc `gateway` est absent sans gateway liée. Les clients (apps, portail cloud) en déduisent « armoire en ligne » uniquement par `gateway.online`. Or une armoire sans gateway interroge directement `mon.essensys.fr` (`serverinfos`, `myactions`, `mystatus`) : le cloud connaît son dernier contact (`machines.last_seen`, mis à jour par le middleware d'authentification à chaque requête de l'armoire, donc toutes les quelques secondes) et sait lui remettre les actions.

Les tests des clients ne simulaient qu'une session **avec** gateway : le cas sans gateway n'a jamais été couvert.

## What Changes

- `PortalArmoireInfo` gagne `Online bool` (`json:"online"`).
- `PortalSession` le renseigne avec une fonction partagée avec la gateway (`IsOnline(lastSeen, now, timeout)`), seuil de 2 min unique.
- Aucun changement du bloc `gateway`, de `GatewayStatus` ni des endpoints legacy.

## Capabilities

### New Capabilities
- `armoire-online` : `armoire.online` dans `GET /api/portal/session`.
- `armoire-online-quality` : tests avec et sans gateway, checkup, validation.

### Modified Capabilities
Aucune.

## Impact

`internal/domain/portal_session.go`, `internal/handlers/portal.go`, tests Go `NR-backend-13`.
