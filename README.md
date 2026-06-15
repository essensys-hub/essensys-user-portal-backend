# essensys-user-portal-backend

Hub cloud Essensys sur OVH — portail domotique distant + backend support-site unifié (prod : `essensys-cloud-backend` :8080).

**Documentation** : [docs/](docs/index.md)

OpenSpec : `essensys-raspberry-gateway/openspec/changes/essensys-cloud-backend-consolidation/`

## Modules

| Package | Routes | Statut |
|---------|--------|--------|
| `internal/portal` | `/api/portal/*` | ✅ Actif |
| `internal/gateway` | `/api/gateway/*` | ✅ Actif (+ `POST /exchange`) |
| `internal/identity` | `/api/auth/*`, `/api/profile/*` | ✅ `CONSOLIDATED_MODE` |
| `internal/admin` | `/api/admin/*`, `/api/newsletter/subscribe` | ✅ `CONSOLIDATED_MODE` |
| `internal/legacyiot` | `/api/mystatus`, … | ✅ `CONSOLIDATED_MODE` |

## Variables d'environnement

| Variable | Défaut | Description |
|----------|--------|-------------|
| `CONSOLIDATED_MODE` | `false` | Active le scaffold identity/admin/legacyiot |
| `EXCHANGE_STALE_TTL_SECONDS` | `120` | TTL cache exchange portail |
| `JWT_SECRET` | — | Partagé avec support-site |
| `ADMIN_TOKEN` | `essensys-admin-secret` | Token legacy admin (scripts) |
| `SMTP_*` | — | Envoi newsletter (Phase 3) |
| `PORT` | `8080` prod / `8081` dev | Port HTTP |

## Démarrage local

```bash
export DB_HOST=127.0.0.1 DB_PORT=5432 DB_USER=essensys DB_PASSWORD=... DB_NAME=essensys_db
export JWT_SECRET=same-as-support-site
export PORT=8081
go run ./cmd/server
```

## Matrice migration (support-site → cloud backend)

| Ancien handler (support-site/backend) | Nouveau package |
|---------------------------------------|-----------------|
| `handlers_auth.go`, `handlers_oauth.go` | `internal/identity` |
| `handlers_admin.go`, `handlers_newsletter.go` | `internal/admin` |
| `handlers.go` (mystatus, serverinfos) | `internal/legacyiot` |
| `handlers_portal.go` | `internal/portal` (via `internal/api`) |
| `handlers_gateway.go` | `internal/gateway` (via `internal/api`) |
| `gatewayrules/rules.go` | `internal/domain/gateway.go` |

## New Relic APM (optionnel)

Désactivé par défaut en local. Variables :

| Variable | Description |
|----------|-------------|
| `NEW_RELIC_ENABLED` | `true` pour activer l’agent Go |
| `NEW_RELIC_LICENSE_KEY` | License key Ingest (vault Ansible en prod) |
| `NEW_RELIC_APP_NAME` | Nom de l’app APM (prod : `essensys-cloud-backend`) |
| `NEW_RELIC_DISTRIBUTED_TRACING_ENABLED` | `true` par défaut |

Aucune donnée sensible (JWT, tokens gateway, payloads domotiques) n’est envoyée en attributs custom.

## Tests

```bash
go test ./...
```

## Licence

MIT
