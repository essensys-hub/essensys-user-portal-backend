# essensys-user-portal-backend

BFF domotique pour le portail utilisateur distant Essensys (`https://mon.essensys.fr`).

- File d’actions cloud normalisées (590 + 605..622)
- Workflow demande de liaison armoire (approbation admin)
- API gateway HTTPS (poll / heartbeat / done)

## Démarrage local

```bash
export DB_HOST=127.0.0.1 DB_PORT=5432 DB_USER=essensys DB_PASSWORD=... DB_NAME=essensys_db
export JWT_SECRET=same-as-support-site
export PORT=8081
go run ./cmd/server
```

## Tests

```bash
go test ./...
```

## Licence

MIT
