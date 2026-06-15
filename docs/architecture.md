# Architecture

## Vue d'ensemble

```
cmd/server/main.go
    └── internal/api/router.go
            ├── internal/portal      (toujours actif)
            ├── internal/gateway     (toujours actif)
            └── si CONSOLIDATED_MODE=true :
                    ├── internal/identity
                    ├── internal/admin
                    └── internal/legacyiot
```

## Packages

| Package | Responsabilité |
|---------|----------------|
| `internal/portal` | Liaisons utilisateur, inject, exchange, link-request |
| `internal/gateway` | Heartbeat CM5, actions cloud, push exchange |
| `internal/identity` | Auth email/OAuth, profil, devices nearby |
| `internal/admin` | Stats, users, audit, newsletter, SMTP |
| `internal/legacyiot` | Protocole WAN passif (mystatus, serverinfos, infos) |
| `internal/data` | Stores PostgreSQL |
| `internal/domain` | Types métier, règles gateway eligibility |
| `internal/middleware` | JWT, AdminAuth, BasicAuth, rate limit |

## CONSOLIDATED_MODE

| Valeur | Comportement |
|--------|--------------|
| `false` | Portail + gateway uniquement (mode staging historique :8081) |
| `true` | Hub complet — **production OVH depuis cutover juin 2026** |

## Règle gateway remote

`domain.IsRemoteEligibleGateway()` : `essensys-server` (VPS legacy) **non éligible** au portail distant `mon.essensys.fr`.

## Observabilité

New Relic APM Go (`internal/observability`) — app prod : `essensys-cloud-backend`.

Middleware Chi : `nrgochi` après logger/recoverer.
