## 1. Complément et regroupement (#42)

- [ ] 1.1 Élargir `ExpandLegacyScenarioBlock` à 623 (déclencheur 605–623, complément 590, 605–622, 623) ; vérifier par `NR-backend-10` et `NR-backend-11`
- [ ] 1.2 Adapter `IsPureBlockAction` et `MergeBlockActions` (623 = 0 regroupable, 623 = 1|2 jamais fusionné) ; vérifier par `NR-backend-12` sur `coalescePendingActions`

## 2. État et livraison (#43)

- [ ] 2.1 Test de lecture de 440 via `/api/portal/exchange` depuis le cache gateway ; vérifier par `go test ./...`
- [ ] 2.2 `/checkup` vert (build, vet, gofmt, tests), déploiement par essensys-ansible avant les clients
