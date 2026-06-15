package data

import (
	"encoding/json"
	"log"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/geo"
)

type machineGeoRow struct {
	IP  *string         `db:"ip_address"`
	Geo json.RawMessage `db:"geo"`
}

func (s *LegacyIoTStore) BackfillMissingMachineGeo() {
	var rows []struct {
		HashedPkey string  `db:"hashed_pkey"`
		IPAddress  *string `db:"ip_address"`
	}
	if err := s.db.Select(&rows, `
		SELECT hashed_pkey, ip_address FROM machines
		WHERE ip_address IS NOT NULL AND ip_address NOT IN ('', '-', '127.0.0.1')
		  AND (
		    geo IS NULL OR geo = '{}'::jsonb
		    OR COALESCE(geo->>'lat', '') IN ('', '0')
		    OR COALESCE(geo->>'lon', '') IN ('', '0')
		  )`); err != nil {
		log.Printf("[legacyiot] geo backfill list: %v", err)
		return
	}
	for i, row := range rows {
		ip := derefString(row.IPAddress, "")
		if !geo.IsLookupable(ip) {
			continue
		}
		hashedPkey := row.HashedPkey
		go func(key, addr string, delay time.Duration) {
			time.Sleep(delay)
			s.resolveAndSaveMachineGeo(key, addr)
		}(hashedPkey, ip, time.Duration(i)*time.Second)
	}
	if len(rows) > 0 {
		log.Printf("[legacyiot] geo backfill queued for %d machine(s)", len(rows))
	}
}

func (s *LegacyIoTStore) resolveAndSaveMachineGeo(hashedPkey, ip string) {
	time.Sleep(time.Second)
	result, err := geo.LookupIP(ip)
	if err != nil {
		log.Printf("[legacyiot] machine geo %s: %v", safeKeyPrefix(hashedPkey), err)
		return
	}
	raw, err := json.Marshal(map[string]any{
		"location": result.Location,
		"lat":      result.Lat,
		"lon":      result.Lon,
	})
	if err != nil {
		return
	}
	if _, err := s.db.Exec(`
		UPDATE machines SET geo = $2::jsonb WHERE hashed_pkey = $1`, hashedPkey, string(raw)); err != nil {
		log.Printf("[legacyiot] machine geo save %s: %v", safeKeyPrefix(hashedPkey), err)
		return
	}
	log.Printf("[legacyiot] machine geo %s (%s): %s", safeKeyPrefix(hashedPkey), ip, result.Location)
}

func (s *LegacyIoTStore) resolveAndSaveGatewayGeo(hostname, ip string) {
	time.Sleep(time.Second)
	result, err := geo.LookupIP(ip)
	if err != nil {
		log.Printf("[legacyiot] gateway geo %s: %v", hostname, err)
		return
	}

	var payload json.RawMessage
	if err := s.db.Get(&payload, `SELECT payload FROM gateway_push_status WHERE hostname = $1`, hostname); err != nil {
		log.Printf("[legacyiot] gateway geo load %s: %v", hostname, err)
		return
	}

	gw := map[string]any{}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &gw)
	}
	gw["geo_location"] = result.Location
	gw["lat"] = result.Lat
	gw["lon"] = result.Lon
	gw["hostname"] = hostname

	raw, err := json.Marshal(gw)
	if err != nil {
		return
	}
	if _, err := s.db.Exec(`
		UPDATE gateway_push_status SET payload = $2::jsonb, updated_at = NOW() WHERE hostname = $1`,
		hostname, string(raw)); err != nil {
		log.Printf("[legacyiot] gateway geo save %s: %v", hostname, err)
		return
	}
	log.Printf("[legacyiot] gateway geo %s (%s): %s", hostname, ip, result.Location)
}

func machineGeoEmpty(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var geo struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	if json.Unmarshal(raw, &geo) != nil {
		return true
	}
	return geo.Lat == 0 && geo.Lon == 0
}

func safeKeyPrefix(key string) string {
	if len(key) <= 10 {
		return key
	}
	return key[:10] + "..."
}
