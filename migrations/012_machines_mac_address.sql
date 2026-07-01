-- Ethernet MAC from armoire exchange indices 947–952 (mystatus / serverinfos identity chunk).
ALTER TABLE machines ADD COLUMN IF NOT EXISTS mac_address VARCHAR(17);

CREATE INDEX IF NOT EXISTS idx_machines_mac_address ON machines(mac_address)
    WHERE mac_address IS NOT NULL AND mac_address <> '';
