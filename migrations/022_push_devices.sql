-- Appareils mobiles pour les notifications push (OpenSpec
-- sonnette-plages-push-2026-11-008, #61).
-- push_devices : un jeton FCM (android) ou APNs (ios) par ligne ; l'unicité
-- (platform, token) fait passer un jeton re-enregistré à l'utilisateur
-- courant. doorbell_notify vaut faux par défaut (opt-in par appareil).
-- doorbell_rings : mémoire des appuis reçus, pour l'idempotence sur
-- (gateway_id, ring_id) ; aucun jeton, aucune donnée personnelle.
-- Le binaire rejoue toutes les migrations à chaque démarrage : uniquement des
-- créations gardées par IF NOT EXISTS, aucune instruction qui modifie ou
-- supprime des données.

CREATE TABLE IF NOT EXISTS push_devices (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    platform TEXT NOT NULL,
    token TEXT NOT NULL,
    app_version TEXT NOT NULL DEFAULT '',
    doorbell_notify BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT push_devices_platform_token_key UNIQUE (platform, token)
);

CREATE INDEX IF NOT EXISTS idx_push_devices_user_id ON push_devices(user_id);

CREATE TABLE IF NOT EXISTS doorbell_rings (
    gateway_id TEXT NOT NULL,
    ring_id TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (gateway_id, ring_id)
);
