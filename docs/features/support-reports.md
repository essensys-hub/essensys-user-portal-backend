# Signalements Bug et Incident (support-reports-2026-10-004)

Les utilisateurs connectés à www.essensys.fr signalent un bug logiciel ou un incident sur leur installation sans compte GitHub. Le backend crée l'issue GitHub à leur place, au nom de la GitHub App « Essensys Support ».

## API

| Méthode | Route | Rôle |
|---|---|---|
| `POST` | `/api/support/reports` | Créer un signalement |
| `GET` | `/api/support/reports` | Lister les signalements du compte connecté |

Les deux routes exigent une session valide (`Authorization: Bearer <jeton>`) et passent par les mêmes contrôles que le portail : 401 sans session, 403 pour un compte interdit, 409 tant qu'un mot de passe temporaire n'est pas changé.

### Création

Corps JSON : `kind` (`bug` ou `incident`), `title` (120 caractères au plus), `actual` (obligatoire), puis selon le type :

- bug : `app` (obligatoire : `web`, `ios`, `android`, `gateway`, `other`), `version`, `mode` (`cloud`, `lan`, `unknown`), `expected`, `steps` ;
- incident : `impact` (obligatoire : `module`, `local`, `remote`, `hardware`, `unknown`), `since`, `mode`, `tried`.

Chaque texte libre est limité à 4 000 caractères.

| Réponse | Cas |
|---|---|
| `201 {ref, kind, title, status, created_at}` | Signalement enregistré. `status` vaut `open` si l'issue est créée, sinon `pending`. |
| `400 {error:"invalid_field", field, reason}` | Champ manquant, trop long ou valeur inconnue |
| `422 {error:"sensitive_content", field, category}` | Mot de passe, secret, adresse email ou adresse postale détectés. Le texte n'est ni renvoyé, ni enregistré, ni journalisé. |
| `429 {error:"rate_limited", retry_after, retry_at}` | Plus de 5 signalements sur 24 heures glissantes pour ce compte |

## Où vont les signalements

- **Bug** : dépôt public `essensys-hub/essensys-support-site`, labels `bug` et `via-portal`, type Bug.
- **Incident** : dépôt privé `essensys-hub/essensys-support`, labels `incident` et `via-portal`.
- Les deux sont ajoutés au Project #6 si `SUPPORT_PROJECT_ID` est renseigné.

L'issue ne contient que les champs du formulaire et deux références : `U-xxxxxxxx` (pseudonyme stable du compte, HMAC avec `SUPPORT_PSEUDONYM_KEY`) et `R-xxxxxxxx` (référence du signalement). Ni nom, ni email, ni identifiant d'armoire. Les mentions `@…` sont neutralisées et le HTML est échappé.

Une fois l'issue créée, le contenu est effacé de la base (`payload = {}`). Pour un incident, l'API ne renvoie jamais l'adresse de l'issue, afin de ne pas exposer le dépôt privé.

## Envoi et suivi

Le backend tente la création immédiatement. Si GitHub ne répond pas, le signalement reste `pending`. Un worker relance l'envoi toutes les 5 minutes (au plus 10 tentatives, avec un recul exponentiel) et relit l'état des issues ouvertes : `resolved` si l'issue est fermée comme terminée, `closed` sinon. Un verrou Postgres garantit qu'une seule instance fait ce travail.

## Configuration

| Variable | Rôle |
|---|---|
| `SUPPORT_PSEUDONYM_KEY` | Clé HMAC des références `U-` (16 caractères minimum, obligatoire en production) |
| `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, `GITHUB_APP_PRIVATE_KEY_FILE` | GitHub App. Si l'une manque, les signalements restent `pending`. |
| `SUPPORT_PUBLIC_REPO`, `SUPPORT_PRIVATE_REPO` | Dépôts cibles (par défaut `essensys-hub/essensys-support-site` et `essensys-hub/essensys-support`) |
| `SUPPORT_PROJECT_ID` | Identifiant du Project v2 (`PVT_…`) |
| `GITHUB_API_URL` | Par défaut `https://api.github.com` (surchargé dans les tests) |
