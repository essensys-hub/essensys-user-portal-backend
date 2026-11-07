## 1. Cloud : noms des lumières (essensys-user-portal-backend) (#55)

- [x] 1.1 Ajouter le catalogue des 33 lumières (`id`, nom d'origine, ordre) et la validation du nom (rogné, 1 à 40 caractères, sans contrôle ni `<` `>`) ; vérifier par des tests de domaine (33 identifiants exacts, 40 accepté, 41 refusé, balise refusée, vide = remise à zéro)
- [x] 1.2 Ajouter la migration idempotente `019_light_names.sql` et le magasin `light_names` ; vérifier par un test de migration (rejouée deux fois) et des tests du magasin
- [x] 1.3 Ajouter `GET /portal/lights/names` ; vérifier par des tests (33 éléments, `null` par défaut, 403 sans armoire liée)
- [x] 1.4 Ajouter `PUT /portal/admin/lights/names/{id}` (administrateur de l'armoire, version, audit `light_name`, dry-run) ; vérifier par des tests de routage (admin_local de l'armoire accepté, user et autre armoire 403, remise à zéro, 400 `unknown_light` et `invalid_name`)
- [x] 1.5 Ajouter `POST /gateway/lights/names/sync` avec la fusion (plus grand `(updated_at, version, updated_by)`, futur borné à 5 minutes) ; vérifier par des tests (cloud plus récent, gateway plus récente, égalité, horloge dans le futur, 401 sans jeton)
