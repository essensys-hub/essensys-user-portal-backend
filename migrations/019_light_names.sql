-- Noms personnalisés des lumières (OpenSpec noms-lumieres-2026-11-004)
-- Une ligne par armoire et par lumière. name NULL = nom d'origine (la remise à
-- zéro est une valeur à part entière : sa version est incrémentée pour qu'elle
-- se propage à la gateway). La fusion cloud <-> gateway compare
-- (updated_at, version, updated_by). Le binaire rejoue toutes les migrations à
-- chaque démarrage : uniquement des créations gardées par IF NOT EXISTS, aucune
-- instruction qui modifie ou supprime des données.

CREATE TABLE IF NOT EXISTS light_names (
    machine_id INT NOT NULL,
    light_id TEXT NOT NULL,
    name TEXT NULL,
    version INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (machine_id, light_id)
);
