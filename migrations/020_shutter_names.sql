-- Noms personnalisés des volets et stores (OpenSpec noms-volets-2026-11-005)
-- Une ligne par armoire et par volet. name NULL = nom d'origine (la remise à
-- zéro est une valeur à part entière : sa version est incrémentée pour qu'elle
-- se propage à la gateway). La fusion cloud <-> gateway compare
-- (updated_at, version, updated_by). Le binaire rejoue toutes les migrations à
-- chaque démarrage : uniquement des créations gardées par IF NOT EXISTS, aucune
-- instruction qui modifie ou supprime des données. La table light_names n'est
-- pas touchée.

CREATE TABLE IF NOT EXISTS shutter_names (
    machine_id INT NOT NULL,
    shutter_id TEXT NOT NULL,
    name TEXT NULL,
    version INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (machine_id, shutter_id)
);
