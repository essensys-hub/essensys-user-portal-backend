-- Prises de sécurité sync profile (OpenSpec security-outlets-2026-10-011)
-- Securite_PriseCoupe (440): 1 = outlets cut, 0 = piloted. The gateway reads it
-- in its serverinfos rotation and pushes it with the other profile keys, so the
-- portal and the apps can show the state in cloud mode. Existing profiles are
-- left untouched; re-running the migration is a no-op.

INSERT INTO sync_profiles (name, gateway_id, index_ranges, interval_hours, pull_from_armoire, push_to_cloud, enabled)
SELECT 'Prises de sécurité', '', '[[440,440]]'::jsonb, 3, TRUE, TRUE, TRUE
WHERE NOT EXISTS (
    SELECT 1 FROM sync_profiles WHERE name = 'Prises de sécurité'
);
