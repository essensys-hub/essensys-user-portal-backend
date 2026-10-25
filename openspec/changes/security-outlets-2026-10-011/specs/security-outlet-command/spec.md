## Purpose

Garantit que la commande des prises de sécurité (indice 623 du scénario 1) est bien déclenchée par l'armoire, et qu'une commande d'éclairage ou de volets ne peut pas rejouer une coupure.

## ADDED Requirements

### Requirement: Complément du bloc scénario 1
`ExpandLegacyScenarioBlock` (`internal/domain/order_expansion.go`) SHALL traiter l'indice 623 comme l'éclairage et les volets : dès qu'un lot contient un indice de 605 à 623, il SHALL ajouter `590 = "1"` s'il manque, compléter 605–622 manquants à `"0"` et compléter 623 manquant à `"0"`. Les autres indices du lot SHALL être conservés tels quels.

#### Scenario: Commande seule de coupure
- **WHEN** un client envoie le seul couple `623 = "1"`
- **THEN** l'action mise en file contient `590 = "1"`, `605` à `622` à `"0"` et `623 = "1"`

#### Scenario: Commande d'éclairage
- **WHEN** un client allume le salon (`612 = "128"`)
- **THEN** l'action contient aussi `623 = "0"`, et ne peut donc pas recouper les prises

### Requirement: Non-régression
Le complément SHALL être verrouillé par des tests `NR-backend-10…12` : coupure seule, rétablissement seul, éclairage seul (623 à 0), lot mixte éclairage + coupure.

#### Scenario: Régression du complément
- **WHEN** une modification retire l'ajout de `623 = "0"` aux commandes d'éclairage
- **THEN** le test `NR-backend-10…12` correspondant échoue
