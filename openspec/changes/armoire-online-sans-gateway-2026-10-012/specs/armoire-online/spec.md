## Purpose

Fait dire au cloud si l'armoire est en ligne, avec ou sans gateway, pour que les apps et le portail cloud n'aient plus à le déduire de la gateway.

## ADDED Requirements

### Requirement: État en ligne de l'armoire dans la session portail
`GET /api/portal/session` SHALL renvoyer dans le bloc `armoire` un booléen `online`, vrai si le dernier contact de l'armoire (`machines.last_seen`) date de 2 minutes au plus, faux s'il est plus ancien ou absent. Le seuil SHALL être le même que celui de `gateway.online`. Le bloc `gateway`, `gateway.online` et les endpoints legacy (`serverinfos`, `mystatus`, `myactions`, `done`) NE SHALL PAS changer.

#### Scenario: Sans gateway, armoire vue
- **WHEN** l'utilisateur n'a pas de gateway liée et que son armoire a été vue il y a 10 secondes
- **THEN** la réponse n'a pas de bloc `gateway` et contient `armoire.online = true`

#### Scenario: Sans gateway, armoire ancienne
- **WHEN** l'armoire a été vue il y a 5 minutes
- **THEN** `armoire.online = false`

#### Scenario: Avec gateway
- **WHEN** la gateway est en ligne et l'armoire vue il y a 10 secondes
- **THEN** `gateway.online = true` et `armoire.online = true`, comme avant pour la gateway

#### Scenario: Seuil
- **WHEN** l'armoire a été vue il y a exactement 2 minutes
- **THEN** `armoire.online = true` ; une seconde de plus et `armoire.online = false`

### Requirement: Non-régression
Les cas avec gateway et sans gateway SHALL être verrouillés par un test `NR-backend-13` (cas A à F), qui porte la référence `#37`.

#### Scenario: Régression de la règle
- **WHEN** une modification fait dépendre `armoire.online` de la gateway
- **THEN** le test du cas « sans gateway, armoire vue » échoue
