package data

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

// macExchangeIndices are k=947..952 (6 bytes of armoire Ethernet MAC).
var macExchangeIndices = []int{947, 948, 949, 950, 951, 952}

// ParseMACFromEK builds aa:bb:cc:dd:ee:ff from exchange table MAC indices.
func ParseMACFromEK(ek []domain.ExchangeKeyValue) string {
	if len(ek) == 0 {
		return ""
	}
	vals := make(map[int]string, len(ek))
	for _, item := range ek {
		vals[item.K] = item.V
	}
	parts := make([]string, 0, 6)
	for _, k := range macExchangeIndices {
		b, ok := parseExchangeByte(vals[k])
		if !ok {
			return ""
		}
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	if len(parts) != 6 {
		return ""
	}
	return strings.ToLower(strings.Join(parts, ":"))
}

func parseExchangeByte(v string) (byte, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0, false
	}
	if n < 0 || n > 255 {
		return 0, false
	}
	return byte(n), true
}

func unknownHashPrefix(clientID string) string {
	const prefix = "UNKNOWN-"
	if !strings.HasPrefix(clientID, prefix) {
		return ""
	}
	return strings.TrimPrefix(clientID, prefix)
}

// ParseMACFromTelemetryJSON extracts MAC from stored machine_telemetry.ek JSON.
func ParseMACFromTelemetryJSON(ekJSON []byte) string {
	if len(ekJSON) == 0 {
		return ""
	}
	var ek []domain.ExchangeKeyValue
	if err := json.Unmarshal(ekJSON, &ek); err != nil {
		return ""
	}
	return ParseMACFromEK(ek)
}
