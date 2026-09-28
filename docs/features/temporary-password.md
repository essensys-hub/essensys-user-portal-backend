# Mot de passe temporaire administrateur

> Feature : `essensys-temporary-password-2026-09-040`
> OpenSpec : `essensys-memory/openspec/changes/essensys-temporary-password-2026-09-040/`
> Guide d'usage (console admin, transmission téléphonique) : `essensys-support-site/docs/features/temporary-password.md`

## Objectif

Fallback manuel à la réinitialisation par lien (`essensys-password-reset-2026-08-039`) pour un compte dont l'adresse email n'est pas joignable. Un administrateur global émet un mot de passe temporaire à durée de vie bornée (72 h), affiché une seule fois, et le compte est verrouillé côté serveur jusqu'à ce que l'utilisateur le remplace.

## Où le trouver

- `POST /api/admin/users/{id}/temporary-password` — émission (`internal/admin/temporary_password.go`)
- `POST /api/auth/password/change` — changement, seule route exemptée du verrou (`internal/identity/password_change.go`)
- Verrou serveur : `internal/middleware/user_status.go` (`enforceActiveUser`)

## Fonctionnalités

- Génération dans `internal/temppass` : 12 caractères, alphabet excluant `O0Il1`, ~70 bits d'entropie.
- `SetTemporaryPassword` / `ClearPasswordChangeRequired` (`internal/data/user_store.go`) : chaque écriture est un `UPDATE` unique, garantissant que le hash et le drapeau `password_change_required_at` ne sont jamais observables désynchronisés.
- Le verrou est évalué à chaque requête authentifiée (`enforceActiveUser`), pas au moment de l'émission du jeton JWT : une session déjà ouverte est bloquée immédiatement, sans réémission de jeton.
- L'émission invalide tout jeton de réinitialisation par lien encore actif pour ce compte (`PasswordResetStore.InvalidateForUser`).
- Envoi par email optionnel (`{"send_email": true}`), jamais automatique ; un échec d'envoi ne fait pas échouer l'émission (`200` avec `email_sent:false` et un motif).
- Audit : `TEMPORARY_PASSWORD_ISSUED` et `PASSWORD_CHANGED_AFTER_TEMPORARY` — ni l'un ni l'autre ne contient le mot de passe en clair ni son empreinte.

## Permissions

Émission réservée au rôle `admin_global` (`requireAdminGlobal`). Le changement de mot de passe est accessible à tout compte authentifié, sur son propre compte uniquement (email résolu depuis le contexte JWT, jamais depuis le corps de la requête).

## Limites connues

- La création de compte (`POST /api/admin/users`) est hors périmètre : elle garde son comportement existant (mot de passe choisi par l'admin, envoyé dans l'email de bienvenue).
- Aucune purge automatique : un compte verrouillé le reste indéfiniment si l'utilisateur ne se reconnecte jamais. La levée manuelle (hors flux normal) est un `UPDATE users SET password_change_required_at = NULL WHERE id = ...`.
- Durée de validité fixe (72 h), non configurable par émission.

## Liens

- Spécification complète : `essensys-memory/openspec/changes/essensys-temporary-password-2026-09-040/`
- Migration : `migrations/014_temporary_password.sql`
- Tests : `internal/admin/temporary_password_test.go`, `internal/identity/login_test.go`, `internal/identity/password_change_test.go`, `internal/middleware/user_status_test.go`
