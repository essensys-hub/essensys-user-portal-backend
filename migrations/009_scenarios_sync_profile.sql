-- Scénarios sync profile (OpenSpec essensys-scenario-management)

ALTER TABLE sync_profiles
    ADD COLUMN IF NOT EXISTS exclude_indices JSONB NOT NULL DEFAULT '[]';

COMMENT ON COLUMN sync_profiles.exclude_indices IS
    'Exchange indices omitted from cloud push (e.g. 590 trigger reset by firmware)';

INSERT INTO sync_profiles (name, gateway_id, index_ranges, interval_hours, exclude_indices, enabled)
SELECT 'Scénarios', '', '[[591,919]]'::jsonb, 3, '[590]'::jsonb, TRUE
WHERE NOT EXISTS (
    SELECT 1 FROM sync_profiles WHERE name = 'Scénarios'
);
