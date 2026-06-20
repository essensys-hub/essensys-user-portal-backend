package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

type ExchangeCache struct {
	MachineID int             `db:"machine_id"`
	Keys      json.RawMessage `db:"keys"`
	UpdatedAt time.Time       `db:"updated_at"`
}

func (s *PortalStore) UpsertGatewayExchange(ctx context.Context, machineID int, keys []domain.ExchangeKV) error {
	merged := keys
	if existing, _, err := s.GetGatewayExchange(ctx, machineID); err == nil && len(existing) > 0 {
		want := make(map[int]struct{}, len(keys)+len(existing))
		for _, kv := range keys {
			want[kv.K] = struct{}{}
		}
		for _, kv := range existing {
			want[kv.K] = struct{}{}
		}
		requested := make([]int, 0, len(want))
		for k := range want {
			requested = append(requested, k)
		}
		merged = MergeExchangeKeys(keys, existing, requested)
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO gateway_exchange_cache (machine_id, keys, updated_at)
		VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (machine_id) DO UPDATE SET keys = EXCLUDED.keys, updated_at = NOW()`,
		machineID, string(raw))
	return err
}

func (s *PortalStore) GetGatewayExchange(ctx context.Context, machineID int) ([]domain.ExchangeKV, time.Time, error) {
	var row ExchangeCache
	err := s.db.GetContext(ctx, &row, `
		SELECT machine_id, keys, updated_at FROM gateway_exchange_cache WHERE machine_id = $1`, machineID)
	if err != nil {
		return nil, time.Time{}, err
	}
	var keys []domain.ExchangeKV
	if err := json.Unmarshal(row.Keys, &keys); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode exchange cache: %w", err)
	}
	return keys, row.UpdatedAt, nil
}

func (s *PortalStore) GetMachineTelemetry(ctx context.Context, clientID string) ([]domain.ExchangeKV, time.Time, error) {
	var ek json.RawMessage
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT ek, updated_at FROM machine_telemetry WHERE client_id = $1`, clientID).Scan(&ek, &updatedAt)
	if err != nil {
		return nil, time.Time{}, err
	}
	var keys []domain.ExchangeKV
	if err := json.Unmarshal(ek, &keys); err != nil {
		return nil, time.Time{}, err
	}
	return keys, updatedAt, nil
}

func (s *PortalStore) MachineIDForGateway(ctx context.Context, gatewayID string) (int, error) {
	var machineID int
	err := s.db.GetContext(ctx, &machineID, `
		SELECT machine_id FROM gateway_sessions WHERE gateway_id = $1`, gatewayID)
	return machineID, err
}

func FilterExchangeKeys(all []domain.ExchangeKV, requested []int) []domain.ExchangeKV {
	if len(requested) == 0 {
		return all
	}
	want := make(map[int]struct{}, len(requested))
	for _, k := range requested {
		want[k] = struct{}{}
	}
	out := make([]domain.ExchangeKV, 0, len(requested))
	for _, kv := range all {
		if _, ok := want[kv.K]; ok {
			out = append(out, kv)
		}
	}
	return out
}

// MergeExchangeKeys returns requested keys, preferring primary over secondary.
func MergeExchangeKeys(primary, secondary []domain.ExchangeKV, requested []int) []domain.ExchangeKV {
	if len(requested) == 0 {
		return FilterExchangeKeys(append(append([]domain.ExchangeKV{}, secondary...), primary...), requested)
	}
	byKey := make(map[int]string, len(primary)+len(secondary))
	for _, kv := range secondary {
		byKey[kv.K] = kv.V
	}
	for _, kv := range primary {
		byKey[kv.K] = kv.V
	}
	out := make([]domain.ExchangeKV, 0, len(requested))
	for _, k := range requested {
		if v, ok := byKey[k]; ok {
			out = append(out, domain.ExchangeKV{K: k, V: v})
		}
	}
	return out
}

func ParseKeyList(keysParam string) ([]int, error) {
	if keysParam == "" {
		return nil, fmt.Errorf("empty keys")
	}
	parts := splitComma(keysParam)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		var k int
		if _, err := fmt.Sscanf(p, "%d", &k); err != nil {
			return nil, fmt.Errorf("invalid key %q", p)
		}
		out = append(out, k)
	}
	return out, nil
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	return parts
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
