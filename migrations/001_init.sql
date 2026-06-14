-- Portal schema for essensys-user-portal-backend (same PostgreSQL instance as support-site)

CREATE TABLE IF NOT EXISTS link_requests (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    machine_serial VARCHAR(255) NOT NULL,
    message TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by VARCHAR(255),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_link_requests_user_id ON link_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_link_requests_status ON link_requests(status);

CREATE TABLE IF NOT EXISTS cloud_actions (
    guid VARCHAR(64) PRIMARY KEY,
    user_id INT NOT NULL,
    machine_id INT,
    params JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    done_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_cloud_actions_status ON cloud_actions(status);

CREATE TABLE IF NOT EXISTS gateway_sessions (
    gateway_id VARCHAR(255) PRIMARY KEY,
    token_hash VARCHAR(128) NOT NULL,
    machine_id INT,
    last_seen TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS portal_audit_log (
    id SERIAL PRIMARY KEY,
    user_email VARCHAR(255),
    action VARCHAR(64) NOT NULL,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
