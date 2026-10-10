-- Bug and incident reports filed from www.essensys.fr by signed-in users who
-- have no GitHub account (support-reports-2026-10-004). The backend opens the
-- GitHub issue on their behalf through the Essensys Support GitHub App.
--
-- payload holds the form fields only until the issue exists: once GitHub has
-- the content it is cleared to '{}' so the portal keeps no second copy of what
-- the user wrote. last_error never carries user content, only GitHub's status
-- and a truncated message, so a failed send cannot leak the report into logs.
-- status: pending (not on GitHub yet), open, resolved (closed as completed),
-- closed (closed for any other reason).

CREATE TABLE IF NOT EXISTS support_reports (
    id           SERIAL PRIMARY KEY,
    public_ref   VARCHAR(16) NOT NULL UNIQUE,
    user_id      INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind         VARCHAR(16) NOT NULL CHECK (kind IN ('bug', 'incident')),
    title        VARCHAR(160) NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    status       VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'open', 'resolved', 'closed')),
    repo         VARCHAR(128),
    issue_number INT,
    issue_url    TEXT,
    attempts     INT NOT NULL DEFAULT 0,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    synced_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_support_reports_user_created ON support_reports(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_support_reports_status ON support_reports(status);
