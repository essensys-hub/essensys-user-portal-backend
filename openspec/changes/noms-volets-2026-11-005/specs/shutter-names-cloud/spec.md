## Purpose

Stocker, lire, modifier et synchroniser avec la gateway les noms personnalisés des 16 volets et stores, par armoire, côté cloud.

## ADDED Requirements

### Requirement: Lecture des noms
`GET /portal/shutters/names` SHALL renvoyer `{"shutters":[…]}` avec les 16 volets et stores du catalogue, dans l'ordre du catalogue, avec `id`, `default`, `name` (`null` si non défini), `version` et `updated_at`, pour l'armoire liée à l'appelant.

#### Scenario: Aucun nom défini
- **WHEN** un utilisateur de l'armoire appelle la route sans qu'aucun nom n'existe
- **THEN** les 16 éléments sont renvoyés avec `name` à `null`, `version` à 0

#### Scenario: Utilisateur sans armoire liée
- **WHEN** un utilisateur sans armoire liée appelle la route
- **THEN** la réponse est 403

### Requirement: Écriture réservée à l'administrateur de l'armoire
`PUT /portal/admin/shutters/names/{id}` avec `{"name": "..."}` ou `{"name": null}` SHALL enregistrer le nom pour l'armoire liée à l'appelant, incrémenter la version, poser `updated_at` (UTC) et `updated_by`, et auditer le changement (événement `shutter_name` : qui, quand, ancien et nouveau nom). Seul un administrateur de cette armoire MUST pouvoir écrire.

#### Scenario: Administrateur renomme
- **WHEN** l'administrateur envoie `{"name": "Volet baie vitrée"}` pour `volet1salon`
- **THEN** la réponse est 200 avec `name` « Volet baie vitrée » et `version` incrémentée, et un événement d'audit `shutter_name` est écrit

#### Scenario: Remise à zéro
- **WHEN** l'administrateur envoie `{"name": null}` ou un nom vide
- **THEN** `name` redevient `null`, la version est incrémentée

#### Scenario: Utilisateur simple ou autre armoire
- **WHEN** un utilisateur de rôle `user`, ou un administrateur d'une autre armoire, envoie une écriture
- **THEN** la réponse est 403 et rien n'est enregistré

### Requirement: Validation du nom
Le nom SHALL être rogné puis contenir de 1 à 40 caractères Unicode, sans caractère de contrôle, sans `<` ni `>`. Un identifiant hors catalogue MUST répondre 400 `unknown_shutter`, un nom invalide 400 `invalid_name`. Un identifiant de lumière (par exemple `salon`) MUST répondre 400 `unknown_shutter` sur cette route.

#### Scenario: Trop long ou balise
- **WHEN** le nom compte 41 caractères, ou contient `<b>`
- **THEN** la réponse est 400 `invalid_name` et rien n'est enregistré

#### Scenario: Identifiant hors catalogue
- **WHEN** l'identifiant est `volet99` ou `dressing`
- **THEN** la réponse est 400 `unknown_shutter`

### Requirement: Synchronisation avec la gateway
`POST /gateway/shutters/names/sync` SHALL fusionner les entrées envoyées par la gateway (zéro à 16) avec l'état du cloud, volet par volet : le plus grand `(updated_at, version, updated_by)` gagne, une égalité parfaite donne la valeur du cloud ; un `updated_at` à plus de 5 minutes dans le futur est ramené à l'heure du cloud. La réponse MUST contenir l'état fusionné complet (`{"shutters":[…]}`, 16 éléments) de l'armoire de la gateway.

#### Scenario: Le cloud est plus récent
- **GIVEN** le cloud contient `volet1salon` = « Volet baie vitrée » à 10:15 et la gateway envoie « Volet 1 » à 10:10
- **THEN** la réponse contient « Volet baie vitrée » et le cloud n'est pas modifié

#### Scenario: La gateway est plus récente
- **GIVEN** la gateway envoie `store` = « Store terrasse » à 10:20, plus récent que le cloud
- **THEN** le cloud enregistre « Store terrasse » et le renvoie dans la réponse

#### Scenario: Égalité
- **GIVEN** mêmes `updated_at`, `version` et `updated_by`, noms différents
- **THEN** la valeur du cloud est conservée

#### Scenario: Gateway inconnue ou sans jeton
- **WHEN** la requête n'est pas authentifiée par `GatewayAuth`
- **THEN** la réponse est 401

### Requirement: Migration
La migration `020_shutter_names.sql` SHALL créer la table `shutter_names (machine_id, shutter_id, name, version, updated_at, updated_by)` de façon idempotente (rejouée à chaque démarrage sans erreur ni doublon) et MUST NOT modifier la table `light_names`.

#### Scenario: Migration rejouée
- **WHEN** la migration est jouée deux fois
- **THEN** aucune erreur, aucune donnée perdue, `light_names` inchangée

### Requirement: Mode essai à blanc
Avec `X-Essensys-Test-Mode: dry-run`, l'écriture SHALL valider la requête, répondre `"status": "dry_run"` et ne rien enregistrer.

#### Scenario: Dry-run
- **WHEN** une écriture valide est envoyée avec l'en-tête
- **THEN** la réponse est 200 `dry_run` et la table est inchangée

### Requirement: Noms des lumières inchangés
Les routes `/lights/names` et la table `light_names` MUST se comporter exactement comme avant cette Feature (non-régression des tests de #41).

#### Scenario: Lumières et volets indépendants
- **WHEN** un volet est renommé
- **THEN** `GET /portal/lights/names` renvoie les mêmes données qu'avant
