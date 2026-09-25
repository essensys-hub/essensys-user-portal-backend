-- Admin-issued temporary password, as a manual fallback to the self-service
-- reset link (013) for accounts that cannot receive mail. Three columns on
-- users rather than a separate table, unlike password_reset_tokens: there is
-- at most one temporary password active per account at a time — it *is* the
-- password, already living in password_hash. temp_password_issued_by is
-- SET NULL on admin deletion so removing an admin account does not cascade
-- into the accounts they once helped.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_change_required_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS temp_password_expires_at    TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS temp_password_issued_by     INT NULL REFERENCES users(id) ON DELETE SET NULL;

-- 013 deliberately rewrote the password_reset template to drop
-- {{temporary_password}} on the grounds that a reusable secret sent by email
-- survives in the mailbox and in SMTP relay logs. This template reintroduces
-- exactly that pattern, so it is opt-in by design rather than the default:
--
--   * screen display is the default channel — the admin reads the password
--     once in the console and relays it by phone;
--   * sending this template happens only when the admin explicitly checks
--     "send by email" at issue time, request by request;
--   * the password expires 72 hours after issue and stops authenticating;
--   * it is cleared the moment the user completes the forced change, so its
--     useful lifetime is one login in the common case.
--
-- Those four conditions are the answer to 013's objection, not a reversal of
-- it. See essensys-memory openspec change essensys-temporary-password-2026-09-040.
INSERT INTO email_templates (slug, name, subject, body_html, body_text, enabled, auto_send)
VALUES
(
    'temporary_password',
    'Mot de passe temporaire',
    'Mot de passe temporaire Essensys',
    '<p>Bonjour {{first_name}},</p>'
        || '<p>Un mot de passe temporaire a été émis pour le compte <strong>{{email}}</strong> : <strong>{{temporary_password}}</strong></p>'
        || '<p>Connectez-vous sur <a href="{{login_url}}">{{login_url}}</a>. Ce mot de passe est valable {{expires_in}} et vous devrez en choisir un nouveau dès votre connexion.</p>'
        || '<p>Si vous n''êtes pas à l''origine de cette demande, contactez {{support_email}} sans vous connecter.</p>',
    'Bonjour {{first_name}}, un mot de passe temporaire a été émis pour {{email}} : {{temporary_password}}. '
        || 'Connexion : {{login_url}} (valable {{expires_in}}, changement obligatoire à la première connexion). '
        || 'Si vous n''êtes pas à l''origine de cette demande, contactez {{support_email}} sans vous connecter.',
    true,
    false
)
ON CONFLICT (slug) DO NOTHING;
