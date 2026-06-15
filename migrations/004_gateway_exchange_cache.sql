-- Exchange snapshot pushed by gateway cloudsync for portal UI

CREATE TABLE IF NOT EXISTS gateway_exchange_cache (
    machine_id INT PRIMARY KEY,
    keys JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gateway_exchange_cache_updated ON gateway_exchange_cache(updated_at DESC);
