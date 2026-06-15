# Migrations PostgreSQL

Appliquées au démarrage depuis `MIGRATIONS_DIR` (tri alphabétique).

| Fichier | Contenu |
|---------|---------|
| `001_init.sql` | Portail : link_requests, cloud_actions, gateway_sessions |
| `002_gateway_identity.sql` | Identité gateway (MAC, tokens) |
| `003_legacy_iot.sql` | `machines`, `machine_telemetry`, `gateway_push_status` |
| `004_gateway_exchange_cache.sql` | Cache exchange portail |
| `005_newsletter.sql` | Abonnés + brouillons newsletter |

Tables identity/admin créées aussi via `EnsureTableExists()` quand `CONSOLIDATED_MODE=true` (`users`, `audit_logs`).

## Import legacy

```bash
go run ./cmd/import-machines -input /opt/essensys/backend/data/machines.json
```

Source JSON : ancien `MemoryStore` du support-site (`machines.json`).
