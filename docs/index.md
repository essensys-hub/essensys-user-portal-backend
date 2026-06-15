# essensys-user-portal-backend — documentation

Hub cloud Essensys sur OVH (`mon.essensys.fr`). En production (juin 2026), ce binaire est déployé comme **`essensys-cloud-backend`** sur le port **8080** avec `CONSOLIDATED_MODE=true`.

## Liens

| Sujet | Document |
|-------|----------|
| Architecture modules | [architecture.md](architecture.md) |
| Routes HTTP | [api-routes.md](api-routes.md) |
| Déploiement & secrets | [deployment.md](deployment.md) |
| Migrations PostgreSQL | [migrations.md](migrations.md) |
| OpenSpec | `essensys-raspberry-gateway/openspec/changes/essensys-cloud-backend-consolidation/` |
| Doc gateway (vue d'ensemble) | `essensys-raspberry-gateway/docs/acces/cloud-backend-consolidation.md` |
| Runbook Ansible | `essensys-ansible/docs/cloud-backend-migration.md` |

## Démarrage local

```bash
export DB_HOST=127.0.0.1 DB_PORT=5432 DB_USER=essensys DB_PASSWORD=... DB_NAME=essensys_db
export JWT_SECRET=dev-secret
export CONSOLIDATED_MODE=true
export PORT=8081
go run ./cmd/server
```

```bash
go test ./...
```

## Service systemd (prod)

| Attribut | Valeur |
|----------|--------|
| Unit | `essensys-cloud-backend.service` |
| Binaire | `/opt/essensys/cloud-backend/cloud-server` |
| Env | `/opt/essensys/cloud-backend/.env` |
| Sources | `/opt/essensys/cloud-backend-src` |
| Port | `8080` |

## Import machines legacy

```bash
go run ./cmd/import-machines -input /path/to/machines.json
```

Variables `DB_*` requises (ou DSN `-dsn`).
