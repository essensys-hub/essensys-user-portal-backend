## Purpose

Permet au portail cloud et aux apps en cloud de lire l'état des prises de sécurité (indice 440) poussé par la gateway.

## ADDED Requirements

### Requirement: Lecture de l'indice 440
`GET /api/portal/exchange?keys=440` SHALL renvoyer la valeur de l'indice 440 dès que la gateway l'a poussée dans le cache cloud, avec les mêmes champs `stale` et `source` que les autres indices. Aucune modification des endpoints legacy (`/api/serverinfos`, `mystatus`, `myactions`, `done`) n'est faite par ce change.

#### Scenario: Valeur poussée
- **WHEN** la gateway a poussé `440 = "1"`
- **THEN** la lecture renvoie `{"k": 440, "v": "1"}`

#### Scenario: Valeur jamais poussée
- **WHEN** la gateway n'a jamais poussé 440 (installation sans gateway)
- **THEN** la lecture ne renvoie pas 440 et le client affiche « État inconnu »
