package domain

import (
	"bytes"
	"errors"
	"strconv"
)

// LightTimer is one light auto-off delay of the exchange table
// (feature lumieres-temporisees-2026-11-003). The value is an unsigned byte:
// 0..255 minutes, 0 meaning the light is never switched off automatically.
type LightTimer struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Key   int    `json:"key"`
}

const (
	LightTimerMinMinutes = 0
	LightTimerMaxMinutes = 255
)

var lightTimers = []LightTimer{
	{ID: "dressing", Label: "Dressing", Key: 521},
	{ID: "wc1", Label: "WC 1", Key: 555},
	{ID: "wc2", Label: "WC 2", Key: 556},
	{ID: "pieceserv", Label: "Service", Key: 557},
	{ID: "ann1", Label: "Annexe 1", Key: 561},
	{ID: "ann2", Label: "Annexe 2", Key: 562},
}

// LightTimers returns a copy of the catalogue, in display order.
func LightTimers() []LightTimer {
	out := make([]LightTimer, len(lightTimers))
	copy(out, lightTimers)
	return out
}

func LightTimerByID(id string) (LightTimer, bool) {
	for _, lt := range lightTimers {
		if lt.ID == id {
			return lt, true
		}
	}
	return LightTimer{}, false
}

var ErrLightTimerMinutes = errors.New("minutes must be an integer between 0 and 255")

// ParseLightTimerMinutes reads a raw JSON value and accepts only a plain
// integer literal in 0..255 (no string, fraction, exponent, null or sign).
func ParseLightTimerMinutes(raw []byte) (int, error) {
	s := bytes.TrimSpace(raw)
	if len(s) == 0 || len(s) > 3 {
		return 0, ErrLightTimerMinutes
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, ErrLightTimerMinutes
		}
	}
	n, err := strconv.Atoi(string(s))
	if err != nil || n < LightTimerMinMinutes || n > LightTimerMaxMinutes {
		return 0, ErrLightTimerMinutes
	}
	return n, nil
}

// CanWriteLightTimers: only the administrator of the linked cabinet may write.
// admin_global is accepted only when a cabinet is linked.
func CanWriteLightTimers(role string, linkedMachineID *int) bool {
	if linkedMachineID == nil {
		return false
	}
	return role == RoleAdminLocal || role == RoleAdminGlobal
}

// LightTimerMinutesFromCache returns nil when the key was never read (or holds
// an out-of-range value): unknown, never 0.
func LightTimerMinutesFromCache(cache []ExchangeKV, key int) *int {
	for _, kv := range cache {
		if kv.K != key {
			continue
		}
		n, err := ParseLightTimerMinutes([]byte(kv.V))
		if err != nil {
			return nil
		}
		return &n
	}
	return nil
}
