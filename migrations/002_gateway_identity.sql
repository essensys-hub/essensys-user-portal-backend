-- Gateway identity triplet: machine_id + CM5 eth0 MAC + armoire bus eth1 MAC

ALTER TABLE gateway_sessions
    ADD COLUMN IF NOT EXISTS eth0_mac VARCHAR(17),
    ADD COLUMN IF NOT EXISTS eth1_mac VARCHAR(17);

CREATE UNIQUE INDEX IF NOT EXISTS idx_gateway_sessions_eth0_mac
    ON gateway_sessions (eth0_mac)
    WHERE eth0_mac IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cloud_actions_machine_id
    ON cloud_actions (machine_id)
    WHERE status = 'pending';
