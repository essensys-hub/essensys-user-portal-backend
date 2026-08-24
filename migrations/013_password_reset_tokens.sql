-- Password reset tokens. Only the SHA-256 digest of the token is stored: a
-- 256-bit random token is not dictionary-attackable, so bcrypt would only add
-- cost on a hot path. Three distinct nullable timestamps rather than one flag,
-- so diagnostics can tell an expired link from a used one from one superseded
-- by a newer request.

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id             SERIAL PRIMARY KEY,
    user_id        INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash     CHAR(64) NOT NULL UNIQUE,
    expires_at     TIMESTAMPTZ NOT NULL,
    consumed_at    TIMESTAMPTZ,
    invalidated_at TIMESTAMPTZ,
    request_ip     VARCHAR(64),
    consumed_ip    VARCHAR(64),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_created_at ON password_reset_tokens(created_at DESC);

-- The template shipped in 006 carried {{temporary_password}}, which would have
-- meant emailing a reusable secret that survives in the mailbox and in SMTP
-- relay logs. Rewritten around a single-use {{reset_url}} and enabled, since it
-- has never been sent and there is nothing to preserve.
UPDATE email_templates
SET name = 'Réinitialisation mot de passe',
    subject = 'Réinitialisation de votre mot de passe Essensys',
    body_html = '<p>Bonjour {{first_name}},</p>'
        || '<p>Une réinitialisation de mot de passe a été demandée pour le compte <strong>{{email}}</strong>.</p>'
        || '<p><a href="{{reset_url}}">Choisir un nouveau mot de passe</a></p>'
        || '<p>Ce lien est valable {{expires_in}} minutes et ne peut servir qu''une seule fois.</p>'
        || '<p>Si vous n''êtes pas à l''origine de cette demande, ignorez ce message : votre mot de passe actuel reste inchangé.</p>'
        || '<p>Contact : {{support_email}}</p>',
    body_text = 'Bonjour {{first_name}}, une réinitialisation de mot de passe a été demandée pour {{email}}. '
        || 'Lien (valable {{expires_in}} minutes, usage unique) : {{reset_url}} '
        || 'Si vous n''êtes pas à l''origine de cette demande, ignorez ce message. Contact : {{support_email}}',
    enabled = true,
    updated_at = NOW()
WHERE slug = 'password_reset';
