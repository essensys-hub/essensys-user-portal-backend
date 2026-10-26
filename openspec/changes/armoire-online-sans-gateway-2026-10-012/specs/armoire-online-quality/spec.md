## Purpose

Fixe le niveau de preuve exigé pour ce correctif.

## ADDED Requirements

### Requirement: Checkup et validation
`/checkup` SHALL être vert sur essensys-user-portal-backend avant la PR. Le correctif SHALL être validé sur le banc BP par un humain, avec une installation sans gateway et avec une gateway, avant l'envoi de la nouvelle version à l'utilisateur pilote.

#### Scenario: Validation sur le banc
- **WHEN** un humain ouvre l'app ou le portail sur une installation sans gateway dont l'armoire est joignable
- **THEN** l'armoire est « en ligne » et un scénario rapide en mode test est accepté
