-- Disposition des objets (OpenSpec affichage-objets-2026-11-006)
-- Un document par armoire et par genre : kind vaut 'lights' ou 'shutters'
-- (borné par le code). order_json et disabled_json sont des listes JSON
-- d'identifiants du catalogue ; version 0 = disposition par défaut. La fusion
-- cloud <-> gateway compare (updated_at, version, updated_by) sur le document
-- entier. Le binaire rejoue toutes les migrations à chaque démarrage :
-- uniquement des créations gardées par IF NOT EXISTS, aucune instruction qui
-- modifie ou supprime des données. Les tables de noms ne sont pas touchées.

CREATE TABLE IF NOT EXISTS object_layout (
    machine_id INT NOT NULL,
    kind TEXT NOT NULL,
    order_json TEXT NOT NULL DEFAULT '[]',
    disabled_json TEXT NOT NULL DEFAULT '[]',
    version INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (machine_id, kind)
);
