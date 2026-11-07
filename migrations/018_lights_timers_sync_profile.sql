-- Temporisations lumières sync profile (OpenSpec lumieres-temporisees-2026-11-003)
-- Light auto-off delays, in minutes (0 = never): 521 (dressing), 555-557 (WC 1,
-- WC 2, service), 561-562 (annexes 1 and 2). The gateway reads them from the
-- cabinet and pushes them with the other profile keys, so the portal and the
-- apps can show them in cloud mode. Existing profiles are left untouched;
-- re-running the migration is a no-op.

INSERT INTO sync_profiles (name, gateway_id, index_ranges, interval_hours, pull_from_armoire, push_to_cloud, enabled)
SELECT 'Temporisations lumières', '', '[[521,521],[555,557],[561,562]]'::jsonb, 3, TRUE, TRUE, TRUE
WHERE NOT EXISTS (
    SELECT 1 FROM sync_profiles WHERE name = 'Temporisations lumières'
);
