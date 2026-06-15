-- Legacy IoT inventory (replaces machines.json when CONSOLIDATED_MODE legacy module active)

CREATE TABLE IF NOT EXISTS machines (
    hashed_pkey TEXT PRIMARY KEY,
    client_id TEXT,
    ip_address TEXT,
    last_seen TIMESTAMPTZ,
    geo JSONB,
    auth_decoded JSONB,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS machine_telemetry (
    client_id TEXT PRIMARY KEY,
    version TEXT,
    ek JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gateway_push_status (
    hostname TEXT PRIMARY KEY,
    payload JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_machines_client_id ON machines(client_id);
CREATE INDEX IF NOT EXISTS idx_machines_last_seen ON machines(last_seen DESC);
