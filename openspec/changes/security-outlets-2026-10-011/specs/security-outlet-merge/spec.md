## Purpose

Garde le regroupement des actions d'éclairage en file (feature portal-action-merge-2026-10-007) compatible avec la commande des prises de sécurité.

## ADDED Requirements

### Requirement: Regroupement compatible
Une action d'éclairage ou de volets complétée par `623 = "0"` SHALL rester un « bloc pur » regroupable. Une action dont l'indice 623 vaut `1` ou `2` NE SHALL PAS être fusionnée : elle SHALL être livrée seule, à sa place dans la file. Le résultat d'une fusion de blocs purs SHALL garder `623 = "0"`.

#### Scenario: Coupure entre deux commandes d'éclairage
- **WHEN** la file contient éclairage A, coupure des prises, éclairage B
- **THEN** la gateway reçoit trois actions dans cet ordre, la coupure n'étant fusionnée avec aucune

#### Scenario: Deux commandes d'éclairage
- **WHEN** la file contient deux actions d'éclairage consécutives avec `623 = "0"`
- **THEN** elles sont fusionnées comme avant, et le résultat contient `623 = "0"`
