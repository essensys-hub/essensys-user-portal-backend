## Purpose

Stocker, lire, modifier et synchroniser avec la gateway la disposition (activation et ordre) des 33 lumières et des 16 volets et stores, par armoire et par genre, côté cloud.

## ADDED Requirements

### Requirement: Lecture de la disposition
`GET /portal/lights/layout` et `GET /portal/shutters/layout` SHALL renvoyer le document `{"order":[…],"disabled":[…],"version":n,"updated_at":…}` du genre pour l'armoire liée à l'appelant, sans `updated_by`. Sans enregistrement, la réponse est la disposition par défaut `{"order":[],"disabled":[],"version":0,"updated_at":null}`.

#### Scenario: Aucune disposition définie
- **WHEN** un utilisateur de l'armoire appelle la route sans qu'aucune disposition n'existe
- **THEN** la réponse est 200 avec `order` et `disabled` vides, `version` à 0 et `updated_at` à `null`

#### Scenario: Utilisateur sans armoire liée
- **WHEN** un utilisateur sans armoire liée appelle la route
- **THEN** la réponse est 403

#### Scenario: Les genres sont indépendants
- **GIVEN** une disposition de volets enregistrée
- **WHEN** un utilisateur lit celle des lumières
- **THEN** elle reste la disposition par défaut

### Requirement: Écriture réservée à l'administrateur de l'armoire
`PUT /portal/admin/lights/layout` et `PUT /portal/admin/shutters/layout` avec `{"order":[…],"disabled":[…]}` SHALL enregistrer le document entier pour l'armoire liée à l'appelant, à la version `n+1`, poser `updated_at` (UTC) et `updated_by`, et auditer le changement (événement `object_layout` : qui, quand, genre, ancien et nouveau document). Seul un administrateur de cette armoire (`admin_local`, ou `admin_global` avec l'armoire liée) MUST pouvoir écrire. Tous les objets peuvent être désactivés.

#### Scenario: Administrateur désactive et réordonne
- **WHEN** l'administrateur envoie `{"order":["volet2salon","volet1salon"],"disabled":["store"]}` aux volets
- **THEN** la réponse est 200 avec `"status":"saved"`, `version` incrémentée, et un événement d'audit `object_layout` est écrit

#### Scenario: Retour à l'ordre d'origine
- **WHEN** l'administrateur envoie `{"reset":true}`
- **THEN** le document devient `order` vide et `disabled` vide, la version est incrémentée (le retour se propage à la gateway)

#### Scenario: Tout désactivé
- **WHEN** l'administrateur désactive les 33 lumières
- **THEN** la réponse est 200

#### Scenario: Utilisateur simple ou autre armoire
- **WHEN** un utilisateur non administrateur, ou l'administrateur d'une autre armoire, envoie la requête
- **THEN** la réponse est 403 et rien n'est écrit

#### Scenario: Validation
- **WHEN** `order` ou `disabled` contient un identifiant inconnu (y compris un identifiant de l'autre genre)
- **THEN** la réponse est 400 `unknown_light` ou `unknown_shutter`
- **WHEN** un identifiant est en double, une clé manque (hors `reset`), ou une valeur n'est pas une liste de chaînes
- **THEN** la réponse est 400 `invalid_layout`

#### Scenario: Les noms ne sont pas touchés
- **GIVEN** une lumière renommée (#41)
- **WHEN** l'administrateur désactive et réordonne les lumières
- **THEN** `GET /portal/lights/names` et la table `light_names` sont inchangés

### Requirement: Mode essai à blanc
Avec l'en-tête `X-Essensys-Test-Mode: dry-run`, l'écriture SHALL valider la requête et répondre `"status":"dry_run"` sans modifier la base ni la version.

#### Scenario: Dry-run
- **WHEN** une requête valide est envoyée avec le mode essai
- **THEN** la réponse est 200 `dry_run` et aucune ligne de `object_layout` n'est écrite ni auditée

### Requirement: Synchronisation avec la gateway
`POST /gateway/lights/layout/sync` et `POST /gateway/shutters/layout/sync` (authentifiés par `GatewayAuth`) SHALL fusionner le document envoyé par la gateway avec celui du cloud pour son armoire, **document entier** : le plus grand `(updated_at, version, updated_by)` gagne, une égalité parfaite donne la valeur du cloud, un `updated_at` à plus de 5 minutes dans le futur est ramené à l'heure du cloud. La réponse SHALL être l'état fusionné complet (avec `updated_by`). Un document de version 0, illisible ou contenant un identifiant inconnu est ignoré sans bloquer la réponse (le cloud renvoie son état).

#### Scenario: Cloud plus récent
- **GIVEN** une disposition cloud plus récente que celle de la gateway
- **WHEN** la gateway synchronise
- **THEN** la réponse contient le document du cloud

#### Scenario: Gateway plus récente
- **GIVEN** une disposition gateway plus récente
- **WHEN** la gateway synchronise
- **THEN** le cloud adopte le document, journalise une trace d'audit `object_layout` (source `gateway_sync`) et le renvoie

#### Scenario: Égalité parfaite
- **THEN** la valeur du cloud est conservée

#### Scenario: Horloge de la gateway dans le futur
- **GIVEN** un `updated_at` à plus de 5 minutes dans le futur
- **THEN** il est ramené à l'heure du cloud avant la comparaison

#### Scenario: Sans jeton
- **WHEN** l'appel n'est pas authentifié
- **THEN** la réponse est 401

### Requirement: Stockage et migration
La disposition SHALL être stockée dans la table `object_layout (machine_id, kind, order_json, disabled_json, version, updated_at, updated_by)` de clé primaire `(machine_id, kind)`, créée par la migration idempotente `021_object_layout.sql` (uniquement des créations gardées par `IF NOT EXISTS`, rejouable à chaque démarrage). Les tables `light_names` et `shutter_names` MUST rester inchangées. Si le stockage est indisponible, les routes répondent 503 `names_unavailable`.

#### Scenario: Migration rejouée
- **WHEN** la migration est exécutée deux fois
- **THEN** elle réussit et les tables de noms et leurs lignes sont inchangées

#### Scenario: Base indisponible
- **WHEN** la base ne répond pas
- **THEN** la réponse est 503 `names_unavailable`
