package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type AdminInventoryStore struct {
	db *sqlx.DB
}

func NewAdminInventoryStore(db *sqlx.DB) *AdminInventoryStore {
	return &AdminInventoryStore{db: db}
}

func (s *AdminInventoryStore) GetStats() (*domain.AdminStatsResponse, error) {
	var stats domain.AdminStatsResponse
	_ = s.db.Get(&stats.TotalMachines, `SELECT COUNT(*) FROM machines`)
	_ = s.db.Get(&stats.TotalGateways, `SELECT COUNT(*) FROM gateway_push_status`)
	_ = s.db.Get(&stats.ConnectedClients, `SELECT COUNT(*) FROM machine_telemetry`)
	return &stats, nil
}

type machineRow struct {
	ID          int             `db:"id"`
	HashedPkey  string          `db:"hashed_pkey"`
	ClientID    *string         `db:"client_id"`
	MacAddress  *string         `db:"mac_address"`
	Eth1MAC     *string         `db:"eth1_mac"`
	IPAddress   *string         `db:"ip_address"`
	LastSeen    *time.Time      `db:"last_seen"`
	Geo         json.RawMessage `db:"geo"`
	AuthDecoded json.RawMessage `db:"auth_decoded"`
}

func (s *AdminInventoryStore) GetMachines() ([]*domain.MachineDetail, error) {
	var rows []machineRow
	if err := s.db.Select(&rows, `
		SELECT m.id, m.hashed_pkey, m.client_id, m.mac_address, m.ip_address, m.last_seen,
		       COALESCE(m.geo, '{}'::jsonb) AS geo,
		       COALESCE(m.auth_decoded, '{}'::jsonb) AS auth_decoded,
		       gs.eth1_mac
		FROM machines m
		LEFT JOIN LATERAL (
			SELECT eth1_mac FROM gateway_sessions
			WHERE machine_id = m.id AND eth1_mac IS NOT NULL AND eth1_mac <> ''
			ORDER BY last_seen DESC NULLS LAST
			LIMIT 1
		) gs ON true
		ORDER BY m.last_seen DESC NULLS LAST`); err != nil {
		return []*domain.MachineDetail{}, err
	}

	list := make([]*domain.MachineDetail, 0, len(rows))
	for _, row := range rows {
		d := &domain.MachineDetail{
			ID:         row.ID,
			NoSerie:    machineDisplaySerie(row.ClientID, row.HashedPkey),
			MacAddress: resolveMachineMAC(row.MacAddress, row.Eth1MAC),
			IP:         derefString(row.IPAddress, "-"),
		}
		if row.LastSeen != nil {
			d.LastSeen = *row.LastSeen
		}
		if len(row.Geo) > 0 {
			var geo struct {
				Location string  `json:"location"`
				Lat      float64 `json:"lat"`
				Lon      float64 `json:"lon"`
			}
			if json.Unmarshal(row.Geo, &geo) == nil {
				d.GeoLocation = geo.Location
				d.Lat = geo.Lat
				d.Lon = geo.Lon
			}
		}
		if len(row.AuthDecoded) > 0 {
			var auth struct {
				RawAuth    string `json:"raw_auth"`
				RawDecoded string `json:"raw_decoded"`
			}
			if json.Unmarshal(row.AuthDecoded, &auth) == nil {
				d.RawAuth = auth.RawAuth
				d.RawDecoded = auth.RawDecoded
			}
		}
		list = append(list, d)
	}
	s.backfillMissingMACs(list)
	return list, nil
}

// HashedPkeyByInventoryID returns the hashed_pkey for a stable machines.id.
func (s *AdminInventoryStore) HashedPkeyByInventoryID(id int) (string, error) {
	if id <= 0 {
		return "", fmt.Errorf("invalid inventory id %d", id)
	}
	var hashedPkey string
	err := s.db.Get(&hashedPkey, `SELECT hashed_pkey FROM machines WHERE id = $1`, id)
	if err != nil {
		return "", fmt.Errorf("inventory id %d: %w", id, err)
	}
	return hashedPkey, nil
}

// GetMachineByID returns one inventory row by stable machines.id.
func (s *AdminInventoryStore) GetMachineByID(id int) (*domain.MachineDetail, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid machine id %d", id)
	}
	var row machineRow
	err := s.db.Get(&row, `
		SELECT m.id, m.hashed_pkey, m.client_id, m.mac_address, m.ip_address, m.last_seen,
		       COALESCE(m.geo, '{}'::jsonb) AS geo,
		       COALESCE(m.auth_decoded, '{}'::jsonb) AS auth_decoded,
		       gs.eth1_mac
		FROM machines m
		LEFT JOIN LATERAL (
			SELECT eth1_mac FROM gateway_sessions
			WHERE machine_id = m.id AND eth1_mac IS NOT NULL AND eth1_mac <> ''
			ORDER BY last_seen DESC NULLS LAST
			LIMIT 1
		) gs ON true
		WHERE m.id = $1`, id)
	if err != nil {
		return nil, err
	}
	d := &domain.MachineDetail{
		ID:         row.ID,
		NoSerie:    machineDisplaySerie(row.ClientID, row.HashedPkey),
		MacAddress: resolveMachineMAC(row.MacAddress, row.Eth1MAC),
		IP:         derefString(row.IPAddress, "-"),
	}
	if row.LastSeen != nil {
		d.LastSeen = *row.LastSeen
	}
	return d, nil
}

type gatewayRow struct {
	Hostname  string          `db:"hostname"`
	Payload   json.RawMessage `db:"payload"`
	UpdatedAt time.Time       `db:"updated_at"`
}

func (s *AdminInventoryStore) GetGateways() ([]*domain.GatewayStatus, error) {
	var rows []gatewayRow
	if err := s.db.Select(&rows, `
		SELECT hostname, COALESCE(payload, '{}'::jsonb) AS payload, updated_at
		FROM gateway_push_status
		ORDER BY updated_at DESC`); err != nil {
		return []*domain.GatewayStatus{}, err
	}

	list := make([]*domain.GatewayStatus, 0, len(rows))
	for _, row := range rows {
		gw := &domain.GatewayStatus{Hostname: row.Hostname, LastSeen: row.UpdatedAt}
		if len(row.Payload) > 0 {
			_ = json.Unmarshal(row.Payload, gw)
			gw.Hostname = row.Hostname
			if gw.LastSeen.IsZero() {
				gw.LastSeen = row.UpdatedAt
			}
		}
		list = append(list, gw)
	}
	return list, nil
}

func resolveMachineMAC(stored, eth1 *string) string {
	if mac := derefString(stored, ""); mac != "" {
		return mac
	}
	return derefString(eth1, "")
}

func (s *AdminInventoryStore) backfillMissingMACs(list []*domain.MachineDetail) {
	for _, d := range list {
		if d.MacAddress != "" {
			continue
		}
		keys := []string{d.NoSerie, fmt.Sprintf("%d", d.ID)}
		for _, key := range keys {
			if key == "" {
				continue
			}
			var ekJSON json.RawMessage
			if err := s.db.Get(&ekJSON, `SELECT ek FROM machine_telemetry WHERE client_id = $1`, key); err != nil {
				continue
			}
			if mac := ParseMACFromTelemetryJSON(ekJSON); mac != "" {
				d.MacAddress = mac
				_, _ = s.db.Exec(`UPDATE machines SET mac_address = $2 WHERE id = $1`, d.ID, mac)
				break
			}
		}
	}
}

func derefString(p *string, fallback string) string {
	if p != nil && *p != "" {
		return *p
	}
	return fallback
}

// machineDisplaySerie shows UNKNOWN-{hash} in admin UI even when client_id is a portal numeric bind.
func machineDisplaySerie(clientID *string, hashedPkey string) string {
	prefix := hashedPkey
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	label := fmt.Sprintf("UNKNOWN-%s", prefix)
	if clientID == nil || *clientID == "" {
		return label
	}
	cid := strings.TrimSpace(*clientID)
	if cid == "" {
		return label
	}
	var n int
	if _, err := fmt.Sscanf(cid, "%d", &n); err == nil && n > 0 {
		return label
	}
	if strings.HasPrefix(cid, "UNKNOWN-") {
		return cid
	}
	return cid
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
