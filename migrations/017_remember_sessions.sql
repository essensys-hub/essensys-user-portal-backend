-- Remembered sessions (session-30-jours-2026-11-001, #48): a 30-day session the
-- mobile apps swap for a 24 h JWT without asking for the password again. Only
-- the SHA-256 digest of the token is stored, as for password_reset_tokens (013):
-- a 256-bit random token is not dictionary-attackable. expires_at is fixed at
-- login and never rewritten; revoked_at ends a session early. Migrations run at
-- every start, so every statement here must be idempotent.

CREATE TABLE IF NOT EXISTS remember_sessions (
    id           SERIAL PRIMARY KEY,
    user_id      INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   CHAR(64) NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    user_agent   VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_remember_sessions_user_active
    ON remember_sessions(user_id) WHERE revoked_at IS NULL;
