package data

import (
	"context"
	"database/sql"
	"time"
)

// ShutterNameRow is one stored custom shutter name (migration 020).
type ShutterNameRow struct {
	ShutterID string
	Name      *string
	Version   int
	UpdatedAt time.Time
	UpdatedBy string
}

type shutterNameDB struct {
	ShutterID string         `db:"shutter_id"`
	Name      sql.NullString `db:"name"`
	Version   int            `db:"version"`
	UpdatedAt time.Time      `db:"updated_at"`
	UpdatedBy string         `db:"updated_by"`
}

func (r shutterNameDB) row() ShutterNameRow {
	out := ShutterNameRow{ShutterID: r.ShutterID, Version: r.Version, UpdatedAt: r.UpdatedAt.UTC(), UpdatedBy: r.UpdatedBy}
	if r.Name.Valid {
		n := r.Name.String
		out.Name = &n
	}
	return out
}

// ListShutterNames returns the stored names of one cabinet, by shutter id.
func (s *PortalStore) ListShutterNames(ctx context.Context, machineID int) (map[string]ShutterNameRow, error) {
	var rows []shutterNameDB
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT shutter_id, name, version, updated_at, updated_by
		FROM shutter_names WHERE machine_id = $1`, machineID); err != nil {
		return nil, err
	}
	out := make(map[string]ShutterNameRow, len(rows))
	for _, r := range rows {
		out[r.ShutterID] = r.row()
	}
	return out, nil
}

// SetShutterName stores a name written in the cloud. The version is incremented
// in the database (0 + 1 for a new shutter), so two concurrent writes cannot
// share a version.
func (s *PortalStore) SetShutterName(ctx context.Context, machineID int, shutterID string, name *string, by string) (ShutterNameRow, error) {
	var r shutterNameDB
	err := s.db.GetContext(ctx, &r, `
		INSERT INTO shutter_names (machine_id, shutter_id, name, version, updated_at, updated_by)
		VALUES ($1, $2, $3, 1, $4, $5)
		ON CONFLICT (machine_id, shutter_id) DO UPDATE
		SET name = EXCLUDED.name, version = shutter_names.version + 1,
		    updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by
		RETURNING shutter_id, name, version, updated_at, updated_by`,
		machineID, shutterID, nullName(name), time.Now().UTC().Truncate(time.Millisecond), by)
	if err != nil {
		return ShutterNameRow{}, err
	}
	return r.row(), nil
}

// PutShutterNameExact stores a merged value as is (version and timestamp
// included): the gateway sync merge decides, the store does not renumber.
func (s *PortalStore) PutShutterNameExact(ctx context.Context, machineID int, shutterID string, row ShutterNameRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO shutter_names (machine_id, shutter_id, name, version, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (machine_id, shutter_id) DO UPDATE
		SET name = EXCLUDED.name, version = EXCLUDED.version,
		    updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`,
		machineID, shutterID, nullName(row.Name), row.Version, row.UpdatedAt.UTC(), row.UpdatedBy)
	return err
}
