package data

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/geo"
	"github.com/jmoiron/sqlx"
)

type LegacyIoTStore struct {
	db *sqlx.DB
}

func NewLegacyIoTStore(db *sqlx.DB) *LegacyIoTStore {
	return &LegacyIoTStore{db: db}
}

func (s *LegacyIoTStore) GetMachineByHashedPkey(hashedPkey string) (*domain.LegacyMachine, error) {
	var row struct {
		HashedPkey string  `db:"hashed_pkey"`
		ClientID   *string `db:"client_id"`
		IsActive   bool    `db:"is_active"`
	}
	err := s.db.Get(&row, `SELECT hashed_pkey, client_id, is_active FROM machines WHERE hashed_pkey = $1`, hashedPkey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &domain.LegacyMachine{
		HashedPkey: row.HashedPkey,
		NoSerie:    derefString(row.ClientID, row.HashedPkey),
		IsActive:   row.IsActive,
	}, nil
}

func (s *LegacyIoTStore) RegisterUnknownMachine(hashedPkey string) (*domain.LegacyMachine, error) {
	prefix := hashedPkey
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	noSerie := fmt.Sprintf("UNKNOWN-%s", prefix)
	_, err := s.db.Exec(`
		INSERT INTO machines (hashed_pkey, client_id, is_active, last_seen)
		VALUES ($1, $2, false, NOW())
		ON CONFLICT (hashed_pkey) DO NOTHING`, hashedPkey, noSerie)
	if err != nil {
		return nil, err
	}
	m, err := s.GetMachineByHashedPkey(hashedPkey)
	if err != nil {
		return &domain.LegacyMachine{HashedPkey: hashedPkey, NoSerie: noSerie, IsActive: false}, nil
	}
	log.Printf("[legacyiot] registered unknown machine %s", noSerie)
	return m, nil
}

func (s *LegacyIoTStore) UpdateMachineStatus(hashedPkey, ip, rawAuth, rawDecoded string) {
	var row machineGeoRow
	_ = s.db.Get(&row, `SELECT ip_address, COALESCE(geo, '{}'::jsonb) AS geo FROM machines WHERE hashed_pkey = $1`, hashedPkey)
	oldIP := derefString(row.IP, "")
	triggerGeo := (oldIP != ip && geo.IsLookupable(ip)) || (machineGeoEmpty(row.Geo) && geo.IsLookupable(ip))

	authJSON, _ := json.Marshal(map[string]string{
		"raw_auth":    rawAuth,
		"raw_decoded": rawDecoded,
	})
	_, err := s.db.Exec(`
		UPDATE machines SET ip_address = $2, last_seen = NOW(), auth_decoded = $3::jsonb
		WHERE hashed_pkey = $1`, hashedPkey, ip, string(authJSON))
	if err != nil {
		log.Printf("[legacyiot] update machine status: %v", err)
		return
	}
	if triggerGeo {
		go s.resolveAndSaveMachineGeo(hashedPkey, ip)
	}
}

func (s *LegacyIoTStore) SaveClientData(clientID, version string, ek []domain.ExchangeKeyValue) error {
	ekJSON, err := json.Marshal(ek)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO machine_telemetry (client_id, version, ek, updated_at)
		VALUES ($1, $2, $3::jsonb, NOW())
		ON CONFLICT (client_id) DO UPDATE SET
			version = EXCLUDED.version,
			ek = EXCLUDED.ek,
			updated_at = NOW()`, clientID, version, string(ekJSON))
	return err
}

func (s *LegacyIoTStore) SaveGateway(gw *domain.GatewayStatus) error {
	triggerGeo := false
	var existing domain.GatewayStatus
	var existingPayload json.RawMessage
	err := s.db.Get(&existingPayload, `SELECT payload FROM gateway_push_status WHERE hostname = $1`, gw.Hostname)
	if err == nil && len(existingPayload) > 0 {
		_ = json.Unmarshal(existingPayload, &existing)
		if gw.IP != existing.IP && geo.IsLookupable(gw.IP) {
			triggerGeo = true
		} else {
			gw.GeoLocation = existing.GeoLocation
			gw.Lat = existing.Lat
			gw.Lon = existing.Lon
		}
		if gw.GeoLocation == "" && (gw.Lat == 0 && gw.Lon == 0) && geo.IsLookupable(gw.IP) {
			triggerGeo = true
		}
	} else if geo.IsLookupable(gw.IP) {
		triggerGeo = true
	}

	payload, err := json.Marshal(gw)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO gateway_push_status (hostname, payload, updated_at)
		VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (hostname) DO UPDATE SET
			payload = EXCLUDED.payload,
			updated_at = NOW()`, gw.Hostname, string(payload))
	if err != nil {
		return err
	}
	if triggerGeo {
		go s.resolveAndSaveGatewayGeo(gw.Hostname, gw.IP)
	}
	return nil
}

func (s *LegacyIoTStore) ImportMachine(hashedPkey, clientID, ip string, isActive bool, lastSeen time.Time, authDecoded json.RawMessage) error {
	_, err := s.db.Exec(`
		INSERT INTO machines (hashed_pkey, client_id, ip_address, is_active, last_seen, auth_decoded)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (hashed_pkey) DO UPDATE SET
			client_id = EXCLUDED.client_id,
			ip_address = COALESCE(EXCLUDED.ip_address, machines.ip_address),
			is_active = EXCLUDED.is_active,
			last_seen = COALESCE(EXCLUDED.last_seen, machines.last_seen),
			auth_decoded = COALESCE(EXCLUDED.auth_decoded, machines.auth_decoded)`,
		hashedPkey, clientID, nullString(ip), isActive, lastSeen, authDecoded)
	return err
}

func nullString(s string) interface{} {
	if s == "" || s == "-" {
		return nil
	}
	return s
}
