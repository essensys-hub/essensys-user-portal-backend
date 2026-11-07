package data

import (
	"context"
	"database/sql"
	"time"
)

// LightNameRow is one stored custom light name (migration 019).
type LightNameRow struct {
	LightID   string
	Name      *string
	Version   int
	UpdatedAt time.Time
	UpdatedBy string
}

type lightNameDB struct {
	LightID   string         `db:"light_id"`
	Name      sql.NullString `db:"name"`
	Version   int            `db:"version"`
	UpdatedAt time.Time      `db:"updated_at"`
	UpdatedBy string         `db:"updated_by"`
}

func (r lightNameDB) row() LightNameRow {
	out := LightNameRow{LightID: r.LightID, Version: r.Version, UpdatedAt: r.UpdatedAt.UTC(), UpdatedBy: r.UpdatedBy}
	if r.Name.Valid {
		n := r.Name.String
		out.Name = &n
	}
	return out
}

func nullName(n *string) sql.NullString {
	if n == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *n, Valid: true}
}

// ListLightNames returns the stored names of one cabinet, by light id.
func (s *PortalStore) ListLightNames(ctx context.Context, machineID int) (map[string]LightNameRow, error) {
	var rows []lightNameDB
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT light_id, name, version, updated_at, updated_by
		FROM light_names WHERE machine_id = $1`, machineID); err != nil {
		return nil, err
	}
	out := make(map[string]LightNameRow, len(rows))
	for _, r := range rows {
		out[r.LightID] = r.row()
	}
	return out, nil
}

// SetLightName stores a name written in the cloud. The version is incremented
// in the database (0 + 1 for a new light), so two concurrent writes cannot
// share a version.
func (s *PortalStore) SetLightName(ctx context.Context, machineID int, lightID string, name *string, by string) (LightNameRow, error) {
	var r lightNameDB
	err := s.db.GetContext(ctx, &r, `
		INSERT INTO light_names (machine_id, light_id, name, version, updated_at, updated_by)
		VALUES ($1, $2, $3, 1, $4, $5)
		ON CONFLICT (machine_id, light_id) DO UPDATE
		SET name = EXCLUDED.name, version = light_names.version + 1,
		    updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by
		RETURNING light_id, name, version, updated_at, updated_by`,
		machineID, lightID, nullName(name), time.Now().UTC().Truncate(time.Millisecond), by)
	if err != nil {
		return LightNameRow{}, err
	}
	return r.row(), nil
}

// PutLightNameExact stores a merged value as is (version and timestamp
// included): the gateway sync merge decides, the store does not renumber.
func (s *PortalStore) PutLightNameExact(ctx context.Context, machineID int, lightID string, row LightNameRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO light_names (machine_id, light_id, name, version, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (machine_id, light_id) DO UPDATE
		SET name = EXCLUDED.name, version = EXCLUDED.version,
		    updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`,
		machineID, lightID, nullName(row.Name), row.Version, row.UpdatedAt.UTC(), row.UpdatedBy)
	return err
}
