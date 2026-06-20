-- Configurable gateway ↔ cloud exchange sync profiles (OpenSpec essensys-cloud-sync-scheduler)

CREATE TABLE IF NOT EXISTS sync_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    index_ranges JSONB NOT NULL DEFAULT '[]',
    interval_hours INT NOT NULL DEFAULT 3 CHECK (interval_hours >= 1),
    pull_from_armoire BOOLEAN NOT NULL DEFAULT TRUE,
    push_to_cloud BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sync_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES sync_profiles(id) ON DELETE CASCADE,
    gateway_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'success', 'partial', 'failed')),
    expected_count INT NOT NULL DEFAULT 0,
    received_count INT NOT NULL DEFAULT 0,
    pushed_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    log_lines JSONB NOT NULL DEFAULT '[]',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_profiles_gateway ON sync_profiles(gateway_id);
CREATE INDEX IF NOT EXISTS idx_sync_profiles_enabled ON sync_profiles(enabled) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_sync_runs_profile_created ON sync_runs(profile_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_runs_gateway_pending ON sync_runs(gateway_id, status)
    WHERE status IN ('pending', 'running');

-- Default templates (gateway_id '' = all gateways) — interval 3 h
INSERT INTO sync_profiles (name, gateway_id, index_ranges, interval_hours)
SELECT v.name, '', v.ranges::jsonb, 3
FROM (VALUES
    ('Planning Zone Jour', '[[13,96]]'),
    ('Planning Zone Nuit', '[[97,180]]'),
    ('Planning SDB1', '[[181,264]]'),
    ('Planning SDB2', '[[265,348]]'),
    ('Modes immédiats chauffage', '[[349,352]]'),
    ('Temps volets', '[[566,585]]')
) AS v(name, ranges)
WHERE NOT EXISTS (SELECT 1 FROM sync_profiles LIMIT 1);
