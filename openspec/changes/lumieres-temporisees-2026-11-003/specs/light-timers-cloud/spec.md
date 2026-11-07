## Purpose

Régler et lire, depuis le cloud, le délai d'extinction automatique de six lumières, pour l'administrateur de l'armoire liée.

## ADDED Requirements

### Requirement: Lecture des temporisations
`GET /portal/lights/timers` SHALL renvoyer, pour chacune des six lumières (`dressing` 521, `wc1` 555, `wc2` 556, `pieceserv` 557, `ann1` 561, `ann2` 562), son libellé, sa clé et sa valeur en minutes. Une valeur jamais lue MUST être `null` (inconnue), jamais 0.

#### Scenario: Valeurs connues
- **GIVEN** le cache contient 555 = "15"
- **WHEN** un utilisateur de l'armoire appelle la route
- **THEN** `wc1` vaut 15 et une lumière sans valeur vaut `null`

### Requirement: Écriture réservée à l'administrateur
`PUT /portal/admin/lights/timers/{id}` avec `{"minutes": n}` SHALL mettre en file une action d'écriture sur la clé de la lumière, pour l'armoire liée à l'appelant. Seul un administrateur de cette armoire MUST pouvoir écrire ; les autres rôles MUST recevoir 403 et aucune action n'est créée.

#### Scenario: Administrateur
- **WHEN** l'administrateur envoie `{"minutes": 30}` pour `wc2`
- **THEN** une action `556 = "30"` est en file, le cache est mis à jour et l'écriture est auditée

#### Scenario: Utilisateur simple
- **WHEN** un utilisateur de rôle `user` envoie la même requête
- **THEN** la réponse est 403 et aucune action n'est créée

### Requirement: Validation
L'identifiant MUST être l'un des six connus et `minutes` un entier de 0 à 255. Sinon la réponse MUST être 400 et aucune action n'est créée.

#### Scenario: Valeur hors plage
- **WHEN** l'administrateur envoie `{"minutes": 256}` ou `{"minutes": 7.5}`
- **THEN** la réponse est 400

### Requirement: Remontée des valeurs
Une migration idempotente MUST ajouter un profil de synchro couvrant les clés 521, 555 à 557 et 561 à 562, en lecture depuis l'armoire et en poussée vers le cloud, sans modifier ni supprimer un profil existant. Rejouée, elle MUST ne rien changer.

#### Scenario: Migration rejouée
- **WHEN** la migration est exécutée deux fois
- **THEN** un seul profil existe et aucune erreur n'est levée

### Requirement: Mode test
Avec le mode dry-run, l'écriture MUST être validée et auditée sans mettre d'action en file.

#### Scenario: Dry-run
- **WHEN** l'administrateur envoie la requête avec le dry-run
- **THEN** la réponse indique une commande validée non exécutée et la file reste vide
