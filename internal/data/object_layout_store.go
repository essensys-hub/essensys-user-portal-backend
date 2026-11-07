package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

// Disposition des objets (migration 021, feature affichage-objets-2026-11-006).

type objectLayoutDB struct {
	OrderJSON    string    `db:"order_json"`
	DisabledJSON string    `db:"disabled_json"`
	Version      int       `db:"version"`
	UpdatedAt    time.Time `db:"updated_at"`
	UpdatedBy    string    `db:"updated_by"`
}

func decodeIDs(raw string) []string {
	out := []string{}
	if json.Unmarshal([]byte(raw), &out) != nil || out == nil {
		return []string{}
	}
	return out
}

func encodeIDs(ids []string) string {
	if ids == nil {
		ids = []string{}
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

func (r objectLayoutDB) layout() domain.ObjectLayout {
	return domain.ObjectLayout{
		Order: decodeIDs(r.OrderJSON), Disabled: decodeIDs(r.DisabledJSON),
		Version: r.Version, UpdatedAt: r.UpdatedAt.UTC(), UpdatedBy: r.UpdatedBy,
	}
}

// GetObjectLayout lit la disposition d'un genre pour une armoire. Sans ligne
// elle rend la disposition par défaut et has à false.
func (s *PortalStore) GetObjectLayout(ctx context.Context, machineID int, kind domain.LayoutKind) (domain.ObjectLayout, bool, error) {
	var r objectLayoutDB
	err := s.db.GetContext(ctx, &r, `
		SELECT order_json, disabled_json, version, updated_at, updated_by
		FROM object_layout WHERE machine_id = $1 AND kind = $2`, machineID, string(kind))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultObjectLayout(), false, nil
	}
	if err != nil {
		return domain.ObjectLayout{}, false, err
	}
	return r.layout(), true, nil
}

// SetObjectLayout enregistre une écriture faite dans le cloud. La version est
// incrémentée en base (1 pour un premier document) : deux écritures
// concurrentes ne partagent pas une version.
func (s *PortalStore) SetObjectLayout(ctx context.Context, machineID int, kind domain.LayoutKind, order, disabled []string, by string) (domain.ObjectLayout, error) {
	var r objectLayoutDB
	err := s.db.GetContext(ctx, &r, `
		INSERT INTO object_layout (machine_id, kind, order_json, disabled_json, version, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, 1, $5, $6)
		ON CONFLICT (machine_id, kind) DO UPDATE
		SET order_json = EXCLUDED.order_json, disabled_json = EXCLUDED.disabled_json,
		    version = object_layout.version + 1,
		    updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by
		RETURNING order_json, disabled_json, version, updated_at, updated_by`,
		machineID, string(kind), encodeIDs(order), encodeIDs(disabled), time.Now().UTC().Truncate(time.Millisecond), by)
	if err != nil {
		return domain.ObjectLayout{}, err
	}
	return r.layout(), nil
}

// PutObjectLayoutExact enregistre une valeur fusionnée telle quelle (version et
// horodatage compris) : la fusion de la synchro décide, le magasin ne renumérote pas.
func (s *PortalStore) PutObjectLayoutExact(ctx context.Context, machineID int, kind domain.LayoutKind, l domain.ObjectLayout) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO object_layout (machine_id, kind, order_json, disabled_json, version, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (machine_id, kind) DO UPDATE
		SET order_json = EXCLUDED.order_json, disabled_json = EXCLUDED.disabled_json, version = EXCLUDED.version,
		    updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`,
		machineID, string(kind), encodeIDs(l.Order), encodeIDs(l.Disabled), l.Version, l.UpdatedAt.UTC(), l.UpdatedBy)
	return err
}
