# Déploiement

## Ansible (production)

Rôle : `essensys-ansible/roles/cloud_backend`

Playbook : `support-site.yml` quand `cloud_backend_consolidated: true`.

### Variables principales

| Variable | Défaut | Description |
|----------|--------|-------------|
| `cloud_backend_consolidated` | `false` | Active le hub unifié |
| `cloud_backend_legacy_mode` | `false` | Rollback dual backend |
| `cloud_backend_port` | `8080` | Port HTTP |
| `cloud_backend_skip_git_clone` | `false` | `true` si rsync manuel vers `cloud-backend-src` |
| `cloud_backend_import_machines` | `true` | Import `machines.json` → PG |

### Template `.env`

Généré depuis `roles/cloud_backend/templates/cloud-backend.env.j2` :

- PostgreSQL (`DB_*`)
- `CONSOLIDATED_MODE=true`
- `JWT_SECRET`, `ADMIN_TOKEN`, `ADMIN_EMAILS`
- OAuth Google / Apple (`GOOGLE_*`, `APPLE_*`)
- SMTP newsletter (`SMTP_*`)
- New Relic APM (`NEW_RELIC_*`)

### Secrets Ansible (vault)

**Fichier : `essensys-ansible/group_vars/essensys/vault.yml`**

| Clé vault | Usage |
|-----------|--------|
| `portal_db_password` | Mot de passe PostgreSQL |
| `portal_jwt_secret` | JWT partagé support + portail |
| `vault_admin_token` | Token legacy admin scripts |
| `vault_newrelic_license_key` | NR Ingest |
| `vault_google_client_id` / `_secret` | OAuth Google |
| `vault_google_redirect_url` | Callback Google (optionnel, sinon `cloud_hub_public_url`) |
| `vault_apple_*` | OAuth Apple |
| `vault_apple_redirect_url` | Callback Apple (optionnel) |
| `portal_db_name` | Nom base PostgreSQL (défaut template : `essensys_db`) |
| `vault_smtp_*` | Newsletter |

Modèle local : `essensys-ansible/config/.env.example`  
Mot de passe vault : `essensys-ansible/config/.env` → `ANSIBLE_VAULT_PASSWORD`

!!! warning "Ne pas commiter"
    `vault.yml` et `config/.env` sont gitignored. Ne jamais les pousser sur GitHub.

### Déploiement manuel (rsync)

Si le code n'est pas encore sur GitHub :

```bash
rsync -az essensys-user-portal-backend/ user@vps:/opt/essensys/cloud-backend-src/
# group_vars : cloud_backend_skip_git_clone: true
ansible-playbook -i inventory support-site.yml
```

### Nginx post-cutover

```bash
ansible-playbook -i inventory cloud-nginx-only.yml
```

## Variables runtime (référence)

| Variable | Défaut | Description |
|----------|--------|-------------|
| `CONSOLIDATED_MODE` | `false` | Modules identity/admin/legacyiot |
| `PORT` | `8081` local / `8080` prod | Port HTTP |
| `EXCHANGE_STALE_TTL_SECONDS` | `120` | TTL cache exchange |
| `MIGRATIONS_DIR` | `migrations` | SQL auto au démarrage |
| `CORS_ORIGIN` | `https://mon.essensys.fr` | CORS |

## New Relic

| Variable | Prod |
|----------|------|
| `NEW_RELIC_ENABLED` | `true` |
| `NEW_RELIC_APP_NAME` | `essensys-cloud-backend` |
