## 1. Cloud : temporisations (essensys-user-portal-backend) (#53)

- [x] 1.1 Ajouter le catalogue `{id, libellé, clé}` des six lumières et la validation (identifiant connu, entier de 0 à 255) ; vérifier par des tests de domaine (six clés exactes 521, 555, 556, 557, 561, 562 ; 0 et 255 acceptés ; 256, -1, 7.5 et identifiant inconnu refusés)
- [x] 1.2 Ajouter `GET /portal/lights/timers` (cache gateway, `null` si jamais lue) ; vérifier par des tests (valeur connue, clé absente donne `null`, jamais 0)
- [x] 1.3 Ajouter `PUT /portal/admin/lights/timers/{id}` : contrôle « administrateur de cette armoire », file d'actions, mise à jour du cache, audit, dry-run ; vérifier par des tests de routage (admin_local de l'armoire accepté, admin_local d'une autre armoire, user, guest_local et support refusés en 403 sans action, dry-run sans action) et `NR-backend-14`
- [x] 1.4 Ajouter la migration idempotente `018_lights_timers_sync_profile.sql` (plages `[[521,521],[555,557],[561,562]]`, pull et push) ; vérifier par un test de migration (rejouée deux fois sans doublon, aucun UPDATE ni DELETE)
