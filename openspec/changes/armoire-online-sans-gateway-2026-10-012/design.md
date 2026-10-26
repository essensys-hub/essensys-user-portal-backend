## Context

Motivation : voir `proposal.md`. État des lieux (2026-10-26) : le cloud calcule `online` pour la gateway seulement ; les clients n'ont aucun moyen de savoir qu'une armoire sans gateway est joignable ; les tests n'ont jamais simulé une installation sans gateway.

**Décisions communes aux 4 dépôts** :
- **E1 — Le cloud expose l'état de l'armoire** : `GET /api/portal/session` ajoute `armoire.online` (dernier contact de l'armoire de 2 min au plus, même seuil que la gateway). `gateway.online` et les endpoints legacy (`serverinfos`, `mystatus`, `myactions`, `done`) ne changent pas.
- **E2 — Règle client** : l'armoire est « en ligne » si `gateway.online` **ou** `armoire.online` est vrai. Un champ absent vaut faux : face à un ancien backend, le comportement actuel est conservé.
- **E3 — LAN inchangé** : en mode réseau local, les commandes restent actives.
- **E4 — Tests dans les deux configurations** : avec gateway et sans gateway, matrice A à G ci-dessous, dans chaque dépôt. Le cas D (sans gateway, armoire vue) est celui du signalement et porte la référence `#37`.
- **E5 — Validation réelle** : sur le banc BP, par un humain, avec et sans gateway.

| Cas | Session cloud | Attendu |
|---|---|---|
| A. Avec gateway, en ligne | `gateway.online = true` | en ligne, commandes actives |
| B. Avec gateway, hors ligne, armoire vue | `gateway.online = false`, `armoire.online = true` | en ligne, commandes actives |
| C. Avec gateway, tout hors ligne | `gateway.online = false`, `armoire.online = false` | hors ligne, commandes désactivées |
| D. Sans gateway, armoire vue | pas de bloc `gateway`, `armoire.online = true` | en ligne, commandes actives |
| E. Sans gateway, armoire ancienne | pas de bloc `gateway`, `armoire.online = false` | hors ligne, commandes désactivées |
| F. Sans gateway, armoire jamais vue | ni `gateway` ni `armoire` | hors ligne, commandes désactivées |
| G. Mode LAN | sans objet | commandes actives, inchangé |

## Goals / Non-Goals

**Goals :** voir les specs de ce change.

**Non-Goals :** l'état de la gateway elle-même, les endpoints legacy du protocole IoT, l'affichage de l'état des prises (indice 440) sans gateway (il reste « inconnu », la lecture de la table d'échange dépendant de la gateway).

## Decisions

### D1 — Fonction d'état partagée
Constante `OnlineTimeout = 2 * time.Minute` et `IsOnline(lastSeen time.Time, now time.Time, timeout time.Duration) bool` (faux si `lastSeen` est nul). `IsGatewayOnline` et `PortalSession` s'en servent : une seule règle, un seul seuil.

### D2 — Session portail
Le bloc `armoire` n'est construit que si `h.inventory != nil && portalAccess` (inchangé) : `Online` = `IsOnline(arm.LastSeen, now, OnlineTimeout)`. Sans armoire résolue, il n'y a pas de bloc, et les clients appliquent « champ absent = faux ».

### D3 — Tests `NR-backend-13`
Table de cas A à F sur `PortalSession` avec un inventaire simulé (gateway absente / en ligne / hors ligne × armoire vue / ancienne / absente), et cas limite à exactement 2 min avec une horloge injectée. Les endpoints legacy ne sont pas touchés (aucun test à adapter).

## Risks / Trade-offs

- [Armoire vue il y a moins de 2 min mais hors service côté BP] → l'état suit le dernier contact réseau, comme pour la gateway ; une commande peut rester en file jusqu'au prochain `myactions`.
- [Ancien backend déployé après le client] → champ absent = faux : comportement actuel, aucune régression.
- [Horloges] → le seuil est évalué côté serveur : aucune dépendance à l'horloge du téléphone.

## Migration Plan

1. Backend cloud d'abord (`armoire.online`), puis portail cloud, puis apps Android et iPhone.
2. Une PR par dépôt avec `Feature: armoire-online-sans-gateway-2026-10-012` et `Refs` vers les sub-issues et #37.
3. Validation sur le banc BP par un humain (sans gateway et avec gateway), puis email de suivi à l'utilisateur pilote.
4. Rollback : retirer `armoire.online` du backend ramène les clients à `gateway.online`.
