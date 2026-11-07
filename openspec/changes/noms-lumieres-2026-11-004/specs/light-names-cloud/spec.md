## Purpose

Stocker, lire, modifier et synchroniser avec la gateway les noms personnalisés des 33 lumières, par armoire, côté cloud.

## ADDED Requirements

### Requirement: Lecture des noms
`GET /portal/lights/names` SHALL renvoyer les 33 lumières du catalogue, dans l'ordre du catalogue, avec `id`, `default`, `name` (`null` si non défini), `version` et `updated_at`, pour l'armoire liée à l'appelant.

#### Scenario: Aucun nom défini
- **WHEN** un utilisateur de l'armoire appelle la route sans qu'aucun nom n'existe
- **THEN** les 33 lumières sont renvoyées avec `name` à `null`, `version` à 0

#### Scenario: Utilisateur sans armoire liée
- **WHEN** un utilisateur sans armoire liée appelle la route
- **THEN** la réponse est 403

### Requirement: Écriture réservée à l'administrateur de l'armoire
`PUT /portal/admin/lights/names/{id}` avec `{"name": "..."}` ou `{"name": null}` SHALL enregistrer le nom pour l'armoire liée à l'appelant, incrémenter la version, poser `updated_at` (UTC) et `updated_by`, et auditer le changement (qui, quand, ancien et nouveau nom). Seul un administrateur de cette armoire MUST pouvoir écrire.

#### Scenario: Administrateur renomme
- **WHEN** l'administrateur envoie `{"name": "WC 3"}` pour `dressing`
- **THEN** la réponse est 200 avec `name` « WC 3 » et `version` incrémentée, et un événement d'audit `light_name` est écrit

#### Scenario: Remise à zéro
- **WHEN** l'administrateur envoie `{"name": null}` ou un nom vide
- **THEN** `name` redevient `null`, la version est incrémentée

#### Scenario: Utilisateur simple ou autre armoire
- **WHEN** un utilisateur de rôle `user`, ou un administrateur d'une autre armoire, envoie une écriture
- **THEN** la réponse est 403 et rien n'est enregistré

### Requirement: Validation du nom
Le nom SHALL être rogné puis contenir de 1 à 40 caractères Unicode, sans caractère de contrôle, sans `<` ni `>`. Un identifiant hors catalogue MUST répondre 400 `unknown_light`, un nom invalide 400 `invalid_name`.

#### Scenario: Trop long ou balise
- **WHEN** le nom compte 41 caractères, ou contient `<b>`
- **THEN** la réponse est 400 `invalid_name` et rien n'est enregistré

### Requirement: Synchronisation avec la gateway
`POST /gateway/lights/names/sync` SHALL fusionner les entrées envoyées par la gateway avec l'état du cloud, lumière par lumière : le plus grand `(updated_at, version, updated_by)` gagne ; un `updated_at` à plus de 5 minutes dans le futur est ramené à l'heure du cloud. La réponse MUST contenir l'état fusionné complet de l'armoire de la gateway.

#### Scenario: Le cloud est plus récent
- **GIVEN** le cloud contient `dressing` = « WC 3 » à 10:15 et la gateway envoie « Dressing 2 » à 10:10
- **THEN** la réponse contient « WC 3 » et le cloud n'est pas modifié

#### Scenario: La gateway est plus récente
- **GIVEN** la gateway envoie `wc1` = « WC Haut » à 10:20, plus récent que le cloud
- **THEN** le cloud enregistre « WC Haut » et le renvoie dans la réponse

#### Scenario: Gateway inconnue ou sans jeton
- **WHEN** la requête n'est pas authentifiée par `GatewayAuth`
- **THEN** la réponse est 401

### Requirement: Migration
La migration `019_light_names.sql` SHALL créer la table `light_names (machine_id, light_id, name, version, updated_at, updated_by)` de façon idempotente (rejouée à chaque démarrage sans erreur ni doublon).

#### Scenario: Migration rejouée
- **WHEN** la migration est jouée deux fois
- **THEN** aucune erreur, aucune donnée perdue

### Requirement: Mode essai à blanc
Avec `X-Essensys-Test-Mode: dry-run`, l'écriture SHALL valider la requête, répondre `"status": "dry_run"` et ne rien enregistrer.

#### Scenario: Dry-run
- **WHEN** une écriture valide est envoyée avec l'en-tête
- **THEN** la réponse est 200 `dry_run` et la table est inchangée
