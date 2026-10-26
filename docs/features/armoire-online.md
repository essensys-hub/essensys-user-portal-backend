# Armoire en ligne, avec ou sans gateway

> Feature `armoire-online-sans-gateway-2026-10-012` — essensys-hub/essensys-feature-lifecycle#37 (statut : en cours)

`GET /api/portal/session` indique si l'armoire est joignable, que l'installation ait une gateway ou non :

- `gateway.online` : la gateway (CM5) a été vue depuis 2 minutes au plus. Le bloc `gateway` n'existe que si une gateway est liée.
- `armoire.online` : l'armoire a été vue depuis 2 minutes au plus (`machines.last_seen`, mis à jour à chaque requête authentifiée de l'armoire : `serverinfos`, `mystatus`, `myactions`). Présent dès que l'armoire est résolue.
- Les deux utilisent le même seuil (`domain.OnlineTimeout`) et la même fonction (`domain.IsOnline`).

Règle pour les clients (apps, portail cloud) : l'armoire est **en ligne** si `gateway.online` **ou** `armoire.online`. Un champ absent vaut faux.

Avant ce correctif, les clients ne regardaient que la gateway : une installation sans gateway apparaissait « hors ligne » et ses commandes étaient bloquées, alors que l'armoire dialoguait avec le cloud.

Tests : `NR-backend-13` (`internal/domain/portal_session_test.go`), cas avec gateway et sans gateway.
Conception : `openspec/changes/armoire-online-sans-gateway-2026-10-012/`.
