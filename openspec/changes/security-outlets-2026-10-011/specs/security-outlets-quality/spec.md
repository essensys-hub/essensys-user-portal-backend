## Purpose

Garantit que la commande des prises de sécurité est testée sans armoire réelle, vérifiée par `/checkup` et validée sur le banc BP avant annonce.

## ADDED Requirements

### Requirement: Tests sans armoire
Les tests de essensys-user-portal-backend SHALL couvrir la commande, l'annulation, l'état et le mode test contre un backend simulé, garde no-armoire active ; aucune armoire réelle NE SHALL être pilotée.

#### Scenario: Suite de tests
- **WHEN** on lance les tests de essensys-user-portal-backend
- **THEN** ils passent sans accès réseau à une armoire réelle

### Requirement: Checkup et validation
`/checkup` SHALL être vert sur essensys-user-portal-backend avant la PR. La feature SHALL être validée sur le banc BP par un humain (coupure, rétablissement, puis commande d'éclairage sans recoupure) avant d'être annoncée.

#### Scenario: Validation sur le banc
- **WHEN** un humain coupe les prises depuis essensys-user-portal-backend sur le banc, puis allume une lumière
- **THEN** les prises restent coupées jusqu'au « Rétablir », et l'indice 440 passe à 1 puis à 0
