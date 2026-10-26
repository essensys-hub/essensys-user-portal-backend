## 1. Session portail : armoire.online (#45)

- [ ] 1.1 Ajouter `Online` à `PortalArmoireInfo` et la fonction partagée `IsOnline` (seuil unique de 2 min) ; vérifier par un test unitaire du seuil (2 min exactement, 2 min + 1 s)
- [ ] 1.2 Renseigner `armoire.online` dans `PortalSession` ; vérifier par `NR-backend-13` (cas A à F, avec et sans gateway)

## 2. Livraison (#46)

- [ ] 2.1 `/checkup` vert (build, vet, gofmt, tests), puis déploiement par essensys-ansible (`deploy-portal-stack.yml`) avant les clients
- [ ] 2.2 Vérifier en lecture seule `GET /api/portal/session` sur une installation sans gateway (champ `armoire.online`)
