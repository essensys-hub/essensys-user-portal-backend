# Routes API

Base URL prod : `https://mon.essensys.fr/api`

## Portail (`internal/portal`)

| Méthode | Route | Auth |
|---------|-------|------|
| GET | `/portal/health` | — |
| POST | `/portal/link-request` | JWT user |
| GET | `/portal/link-request/status` | JWT user |
| GET | `/portal/session` | JWT user — profil, gateway et armoire liés |
| GET | `/portal/gateway/status` | JWT user |
| POST | `/portal/inject` | JWT user |
| POST | `/portal/inject/batch` | JWT user — plusieurs k/v en une action cloud (planning chauffage) |
| GET | `/portal/exchange` | JWT user |
| GET | `/portal/history/latest` | JWT user |
| POST | `/portal/web/actions` | JWT user |
| GET | `/portal/lights/timers` | JWT user — les six temporisations des lumières (`minutes` 0 à 255, `null` si jamais lue) |
| PUT | `/portal/admin/lights/timers/{id}` | JWT admin — `{"minutes":n}`, réservé à `admin_local` (ou `admin_global`) avec armoire liée ; 400 `invalid_minutes` / `unknown_light`, 403 `forbidden` ; mode test : `status` `dry_run` |
| GET | `/portal/lights/names` | JWT user — les 33 lumières `{id, default, name, version, updated_at}` (`name` `null` si aucun nom personnalisé), dans l'ordre du catalogue |
| GET | `/portal/shutters/names` | JWT user — les 16 volets et stores `{id, default, name, version, updated_at}` sous la clé `shutters` (`name` `null` si aucun nom personnalisé), dans l'ordre du catalogue |
| PUT | `/portal/admin/lights/names/{id}` | JWT admin — `{"name":"WC 3"}` ou `{"name":null}` (remise au nom d'origine), réservé à `admin_local` (ou `admin_global`) avec armoire liée ; nom rogné de 1 à 40 caractères sans contrôle ni `<` `>` ; 400 `invalid_name` / `unknown_light`, 403 `forbidden` ; audit `light_name` (ancien et nouveau nom) ; mode test : `status` `dry_run`, rien d'écrit |
| PUT | `/portal/admin/shutters/names/{id}` | JWT admin — `{"name":"Volet baie vitrée"}` ou `{"name":null}` (remise au nom d'origine), mêmes droits et même règle de nom que les lumières ; 400 `invalid_name` / `unknown_shutter`, 403 `forbidden` ; audit `shutter_name` (ancien et nouveau nom) ; mode test : `status` `dry_run`, rien d'écrit |
| GET | `/portal/lights/layout` | JWT user — disposition des 33 lumières de l'armoire liée `{order, disabled, version, updated_at}` (sans `updated_by`) ; défaut `order` et `disabled` vides, `version` 0, `updated_at` `null` ; 403 sans armoire liée |
| GET | `/portal/shutters/layout` | JWT user — idem pour les 16 volets et stores (document indépendant de celui des lumières) |
| PUT | `/portal/admin/lights/layout` | JWT admin — `{"order":[…],"disabled":[…]}` (les deux clés) ou `{"reset":true}`, réservé à `admin_local` (ou `admin_global`) avec armoire liée ; version `n+1` ; 400 `unknown_light` / `invalid_layout` (doublon, clé absente, corps de plus de 16 Ko), 403 `forbidden`, 503 `names_unavailable` ; audit `object_layout` ; mode test : `status` `dry_run`, rien d'écrit |
| PUT | `/portal/admin/shutters/layout` | JWT admin — idem pour les volets ; 400 `unknown_shutter` / `invalid_layout` |
| POST | `/portal/admin/link-requests/{id}/approve` | JWT admin |
| POST | `/portal/admin/gateways/register` | JWT admin |

## Gateway CM5 (`internal/gateway`)

| Méthode | Route | Auth |
|---------|-------|------|
| GET | `/gateway/pending-actions` | Gateway headers |
| POST | `/gateway/heartbeat` | Gateway headers |
| POST | `/gateway/actions/{guid}/done` | Gateway headers |
| POST | `/gateway/exchange` | Gateway headers |
| POST | `/gateway/lights/names/sync` | Gateway headers — `{"names":[{id,name,version,updated_at}…]}` (0 à 33 entrées) fusionné lumière par lumière (plus grand `(updated_at, version, updated_by)` gagne, égalité : le cloud reste, `updated_at` à plus de 5 min dans le futur ramené à l'heure du cloud, entrées illisibles ignorées) ; répond l'état complet `{"lights":[…]}` |
| POST | `/gateway/shutters/names/sync` | Gateway headers — `{"names":[{id,name,version,updated_at}…]}` (0 à 16 entrées), même fusion que les lumières, volet par volet ; répond l'état complet `{"shutters":[…]}` |
| POST | `/gateway/lights/layout/sync` | Gateway headers — document de la gateway `{order,disabled,version,updated_at,updated_by}` fusionné en entier (plus grand `(updated_at, version, updated_by)` gagne, égalité : le cloud reste, futur de plus de 5 min ramené à l'heure du cloud ; version 0 ou document illisible ignoré) ; répond l'état fusionné avec `updated_by` ; 400 `invalid_body` |
| POST | `/gateway/shutters/layout/sync` | Gateway headers — idem pour les volets |

Headers gateway : `X-Gateway-ID`, `Authorization: Bearer`, `X-Gateway-Eth0-MAC`, `X-Gateway-Eth1-MAC`.

## Identity (`CONSOLIDATED_MODE`)

| Méthode | Route |
|---------|-------|
| POST | `/auth/register`, `/auth/login`, `/auth/logout` |
| GET | `/auth/google/login`, `/auth/google/callback` |
| GET | `/auth/apple/login`, POST `/auth/apple/callback` |
| GET/PUT/DELETE | `/profile` |
| PUT | `/profile/links` |
| GET | `/devices/nearby` |

## Admin (`CONSOLIDATED_MODE`)

| Méthode | Route | Auth |
|---------|-------|------|
| POST | `/admin/login` | body token |
| GET | `/admin/stats`, `/machines`, `/gateways`, `/users`, `/audit` | AdminAuth |
| CRUD | `/admin/newsletters/*`, `/admin/subscribers` | AdminAuth |
| POST | `/newsletter/subscribe` | public |

`AdminAuth` : Bearer `ADMIN_TOKEN` **ou** JWT rôle admin/support.

## Legacy IoT (`CONSOLIDATED_MODE`)

| Méthode | Route | Basic Auth |
|---------|-------|------------|
| POST | `/mystatus` | strict |
| GET | `/myactions` | strict → `cloud_actions` pending pour la machine |
| POST | `/done/{guid}` | strict → acquittement firmware |
| GET | `/serverinfos` | optional |
| POST | `/infos` | optional (gateway push) |
