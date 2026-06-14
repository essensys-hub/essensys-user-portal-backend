package data

import (
	"fmt"
	"strings"
)

// NormalizeMAC lowercases and validates aa:bb:cc:dd:ee:ff (also accepts aa-bb-... or aabbccddeeff).
func NormalizeMAC(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", fmt.Errorf("empty MAC")
	}
	s = strings.ReplaceAll(s, "-", ":")
	if !strings.Contains(s, ":") && len(s) == 12 {
		var parts []string
		for i := 0; i < 12; i += 2 {
			parts = append(parts, s[i:i+2])
		}
		s = strings.Join(parts, ":")
	}
	parts := strings.Split(s, ":")
	if len(parts) != 6 {
		return "", fmt.Errorf("invalid MAC %q", raw)
	}
	for _, p := range parts {
		if len(p) != 2 {
			return "", fmt.Errorf("invalid MAC %q", raw)
		}
		for _, c := range p {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return "", fmt.Errorf("invalid MAC %q", raw)
			}
		}
	}
	return strings.Join(parts, ":"), nil
}

// GatewayIDFromEth0MAC returns a stable gateway_id from eth0 (gw-88a29e342761).
func GatewayIDFromEth0MAC(eth0 string) (string, error) {
	norm, err := NormalizeMAC(eth0)
	if err != nil {
		return "", err
	}
	return "gw-" + strings.ReplaceAll(norm, ":", ""), nil
}
