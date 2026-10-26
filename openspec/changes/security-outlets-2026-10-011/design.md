## Context

Motivation : voir `proposal.md`. État des lieux (2026-10-26) : aucune interface ne pilote les prises ; les backends ne complètent le bloc du scénario 1 que pour 605–622 (une commande 623 seule n'est pas déclenchée) ; l'indice 440 n'est lu ni par la gateway ni par le cloud ; le cloud regroupe les actions d'éclairage consécutives (portal-action-merge-2026-10-007).

**Décisions communes aux 6 dépôts** (le détail propre à chaque dépôt suit) :
- **C1 — Commande** : les clients envoient un lot `[590 = "1", 623 = "<1|2>"]`. Les backends le complètent (605–622 à `0`), comme pour l'éclairage. Envoyer 590 côté client garde la commande valable même avant le déploiement des backends.
- **C2 — Pas de recoupure** : les commandes d'éclairage et de volets portent désormais `623 = "0"` (« ne rien faire »). Une valeur restée dans le tableau du BP ne peut donc pas être rejouée par la commande d'éclairage suivante.
- **C3 — État** : l'indice 440 est lu par la gateway (rotation `serverinfos` et snapshot), poussé au cloud par un profil de synchronisation, et affiché par les clients (« Coupées », « Pilotées » ou « État inconnu »).
- **C4 — Sécurité d'usage** : une confirmation est demandée avant de couper. La commande est désactivée quand l'armoire est hors ligne et respecte le mode test (dry-run).
- **C5 — Validation réelle** : sur le banc BP, par un humain, jamais par les tests automatiques. La feature n'est annoncée qu'après cette validation.

## Goals / Non-Goals

**Goals :** voir les specs de ce change.

**Non-Goals :** la coupure des machines à laver (indice 624, même mécanisme, à traiter séparément) ; les endpoints legacy du protocole IoT ; l'installation sans gateway pour l'état.

## Decisions

### D1 — Complément du bloc (`order_expansion.go`)
`IndexLightEnd` reste 622 ; une nouvelle constante `IndexSecurityOutlets = 623` élargit le déclencheur à 605–623. Le complément ajoute 590, 605–622 et 623 manquants. Tests table-driven `NR-backend-10` (coupure seule), `NR-backend-11` (éclairage → 623 = 0).

### D2 — Regroupement (`merge_params.go`)
`IsPureBlockAction` accepte 623 seulement s'il vaut "0" ; `MergeBlockActions` force `623 = "0"` dans le résultat. Une action avec 623 = 1 ou 2 reste hors fusion (livrée seule, à sa place). Test `NR-backend-12` sur `coalescePendingActions` (éclairage, coupure, éclairage → 3 actions).

### D3 — Lecture de 440
Rien à coder : `/api/portal/exchange` sert déjà toute clé présente dans le cache. Un test vérifie que 440 poussé par la gateway est renvoyé. Les endpoints legacy (`serverinfos`, `mystatus`, `myactions`, `done`) ne changent pas : une installation sans gateway n'aura pas l'état (« État inconnu »).

### D4 — Profil de synchronisation « Prises de sécurité »
Les profils `sync_profiles` vivent dans ce dépôt (migrations 007 et 009). La migration `016_security_outlets_sync_profile.sql` crée le profil [440, 440], pull + push, activé, sans toucher aux profils existants. Elle est idempotente (`WHERE NOT EXISTS`), car les migrations sont rejouées à chaque démarrage. La gateway reçoit ce profil par `sync-config` et pousse 440 à chaque cycle.

## Risks / Trade-offs

- [Le BP garde 623 après exécution du scénario 1] → C2 : 623 = 0 sur chaque commande d'éclairage ou de volets.
- [Le BP garde les masques 605–622] → la commande des prises est complétée à 0 sur 605–622 par les backends.
- [Indice 440 non transmis (installation sans gateway, ancienne gateway)] → « État inconnu », commandes toujours utilisables.
- [Couper les prises par erreur] → confirmation obligatoire, rétablissement en un appui.
- [Comportement réel non vérifiable en test automatique] → validation sur le banc BP (C5).

## Migration Plan

1. Backends d'abord (cloud et gateway), car ils assurent C2 (pas de recoupure) et C3 (état). Les clients restent fonctionnels avant ce déploiement grâce à C1.
2. Puis portails et apps, une PR par dépôt avec `Feature: security-outlets-2026-10-011` et `Refs`/`Closes` vers les sub-issues.
3. Validation sur le banc BP par un humain, puis annonce aux utilisateurs.
4. Rollback : les commandes envoyées par les anciens clients ne changent pas ; retirer la carte suffit côté client.
