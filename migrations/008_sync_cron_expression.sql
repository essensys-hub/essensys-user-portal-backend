-- Optional cron schedule for sync profiles (OpenSpec essensys-cloud-sync-scheduler phase 5)

ALTER TABLE sync_profiles
    ADD COLUMN IF NOT EXISTS cron_expression TEXT;

COMMENT ON COLUMN sync_profiles.cron_expression IS
    'Optional standard cron (e.g. 0 */3 * * *). When set, overrides interval_hours for scheduling.';
