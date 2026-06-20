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
| POST | `/portal/admin/link-requests/{id}/approve` | JWT admin |
| POST | `/portal/admin/gateways/register` | JWT admin |

## Gateway CM5 (`internal/gateway`)

| Méthode | Route | Auth |
|---------|-------|------|
| GET | `/gateway/pending-actions` | Gateway headers |
| POST | `/gateway/heartbeat` | Gateway headers |
| POST | `/gateway/actions/{guid}/done` | Gateway headers |
| POST | `/gateway/exchange` | Gateway headers |

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
