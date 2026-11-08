package domain

import (
	"testing"
	"time"
)

func TestGatewayState(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	gw := "gw-1"
	legacy := "essensys-server"
	empty := ""
	cases := []struct {
		name     string
		linked   *string
		lastSeen time.Time
		want     string
	}{
		{"aucune gateway", nil, now, GatewayStateNone},
		{"identifiant vide", &empty, now, GatewayStateNone},
		{"gateway non éligible", &legacy, now, GatewayStateNone},
		{"en ligne à 30 s", &gw, now.Add(-30 * time.Second), GatewayStateOnline},
		{"limite de 2 minutes", &gw, now.Add(-OnlineTimeout), GatewayStateOnline},
		{"hors ligne à 3 min", &gw, now.Add(-3 * time.Minute), GatewayStateOffline},
		{"jamais vue", &gw, time.Time{}, GatewayStateOffline},
	}
	for _, c := range cases {
		if got := GatewayState(c.linked, c.lastSeen, now); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
