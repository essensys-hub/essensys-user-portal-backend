-- Stable inventory id for machines (admin UI + user links).
-- Previously id was row_number() ORDER BY last_seen, which shifted on every poll.

ALTER TABLE machines ADD COLUMN IF NOT EXISTS id INTEGER;

DO $$
DECLARE
    max_id INTEGER;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM machines WHERE id IS NULL LIMIT 1) THEN
        RETURN;
    END IF;

    -- Keep portal links that already used numeric client_id (armoire-seule bind).
    UPDATE machines
    SET id = client_id::INTEGER
    WHERE id IS NULL
      AND client_id ~ '^[0-9]+$'
      AND NOT EXISTS (
          SELECT 1 FROM machines m2
          WHERE m2.id = machines.client_id::INTEGER
      );

    SELECT COALESCE(MAX(id), 0) INTO max_id FROM machines;

    WITH unassigned AS (
        SELECT hashed_pkey, ROW_NUMBER() OVER (ORDER BY hashed_pkey ASC) AS rn
        FROM machines
        WHERE id IS NULL
    )
    UPDATE machines m
    SET id = max_id + u.rn
    FROM unassigned u
    WHERE m.hashed_pkey = u.hashed_pkey;
END $$;

CREATE SEQUENCE IF NOT EXISTS machines_id_seq;
SELECT setval('machines_id_seq', COALESCE((SELECT MAX(id) FROM machines), 0));

ALTER TABLE machines ALTER COLUMN id SET DEFAULT nextval('machines_id_seq');

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM machines WHERE id IS NULL LIMIT 1) THEN
        RAISE EXCEPTION 'machines.id backfill incomplete';
    END IF;
END $$;

ALTER TABLE machines ALTER COLUMN id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_machines_id ON machines(id);

ALTER SEQUENCE machines_id_seq OWNED BY machines.id;
