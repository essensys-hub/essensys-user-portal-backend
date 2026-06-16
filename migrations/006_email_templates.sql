-- Transactional email templates and send log

CREATE TABLE IF NOT EXISTS email_templates (
    slug VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    subject TEXT NOT NULL,
    body_html TEXT NOT NULL DEFAULT '',
    body_text TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT false,
    auto_send BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_send_log (
    id SERIAL PRIMARY KEY,
    recipient VARCHAR(255) NOT NULL,
    template_slug VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    admin_id INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_send_log_created_at ON email_send_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_send_log_recipient ON email_send_log(recipient);

INSERT INTO email_templates (slug, name, subject, body_html, body_text, enabled, auto_send)
VALUES
(
    'user_welcome',
    'Bienvenue — création de compte',
    'Votre compte Essensys',
    '<p>Bonjour {{first_name}},</p><p>Votre compte a été créé sur <a href="{{portal_url}}">{{portal_url}}</a>.</p><p>Email : <strong>{{email}}</strong><br>Mot de passe temporaire : <strong>{{temporary_password}}</strong></p><p>Contact : {{support_email}}</p>',
    'Bonjour {{first_name}}, votre compte Essensys est créé. URL: {{portal_url}} Email: {{email}} Mot de passe: {{temporary_password}}',
    false,
    false
),
(
    'device_allocation',
    'Allocation gateway / armoire',
    'Vos équipements Essensys',
    '<p>Bonjour {{first_name}},</p><p>Gateway : <strong>{{gateway_name}}</strong> ({{gateway_ip}})<br>Armoire : <strong>{{armoire_label}}</strong> ({{armoire_ip}})</p><p>Portail : <a href="{{portal_url}}">{{portal_url}}</a></p>',
    'Bonjour {{first_name}}, Gateway: {{gateway_name}} ({{gateway_ip}}), Armoire: {{armoire_label}} ({{armoire_ip}}), Portail: {{portal_url}}',
    false,
    false
),
(
    'password_reset',
    'Réinitialisation mot de passe',
    'Réinitialisation de votre mot de passe Essensys',
    '<p>Bonjour {{first_name}},</p><p>Utilisez ce mot de passe temporaire : <strong>{{temporary_password}}</strong></p><p>Connectez-vous sur {{portal_url}}</p>',
    'Bonjour {{first_name}}, mot de passe temporaire: {{temporary_password}}, portail: {{portal_url}}',
    false,
    false
),
(
    'role_updated',
    'Changement de rôle',
    'Mise à jour de votre rôle Essensys',
    '<p>Bonjour {{first_name}},</p><p>Votre rôle est maintenant : <strong>{{role}}</strong>.</p>',
    'Bonjour {{first_name}}, votre rôle est maintenant: {{role}}',
    false,
    false
)
ON CONFLICT (slug) DO NOTHING;
