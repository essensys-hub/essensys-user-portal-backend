package domain

import (
	"encoding/json"
	"testing"
	"time"
)

var sessionNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func armoireSeen(ago time.Duration) *MachineDetail {
	return &MachineDetail{ID: 7, NoSerie: "UNKNOWN-734254c2", IP: "192.0.2.10", LastSeen: sessionNow.Add(-ago)}
}

func gatewayInfo(online bool) *PortalGatewayInfo {
	return &PortalGatewayInfo{ID: "gw-demo", Hostname: "gw-demo", Online: online}
}

func sessionFor(gateway *PortalGatewayInfo, armoire *MachineDetail) PortalSessionResponse {
	return NewPortalSessionResponse(PortalUserInfo{ID: 1, Email: "demo@essensys.fr"}, true, gateway, armoire, sessionNow)
}

func TestIsOnline_seuil_de_deux_minutes(t *testing.T) {
	cases := []struct {
		name string
		last time.Time
		want bool
	}{
		{"jamais vu", time.Time{}, false},
		{"il y a 10 s", sessionNow.Add(-10 * time.Second), true},
		{"exactement 2 min", sessionNow.Add(-OnlineTimeout), true},
		{"2 min et 1 s", sessionNow.Add(-OnlineTimeout - time.Second), false},
		{"il y a 5 min", sessionNow.Add(-5 * time.Minute), false},
	}
	for _, c := range cases {
		if got := IsOnline(c.last, sessionNow, OnlineTimeout); got != c.want {
			t.Errorf("%s : got %v, want %v", c.name, got, c.want)
		}
	}
}

// NR: NR-backend-13 essensys-hub/essensys-user-portal-backend#45
// Cas A : avec gateway, en ligne (comportement d'origine).
func Test_NR_backend_13_avec_gateway_en_ligne(t *testing.T) {
	r := sessionFor(gatewayInfo(true), armoireSeen(10*time.Second))
	if r.Gateway == nil || !r.Gateway.Online {
		t.Fatalf("gateway.online attendu vrai : %+v", r.Gateway)
	}
	if r.Armoire == nil || !r.Armoire.Online {
		t.Fatalf("armoire.online attendu vrai : %+v", r.Armoire)
	}
}

// Cas B : gateway hors ligne, armoire vue il y a moins de 2 min.
func Test_NR_backend_13_gateway_hors_ligne_armoire_vue(t *testing.T) {
	r := sessionFor(gatewayInfo(false), armoireSeen(30*time.Second))
	if r.Gateway == nil || r.Gateway.Online {
		t.Fatalf("gateway.online attendu faux : %+v", r.Gateway)
	}
	if r.Armoire == nil || !r.Armoire.Online {
		t.Fatalf("armoire.online attendu vrai : %+v", r.Armoire)
	}
}

// Cas C : avec gateway, tout hors ligne.
func Test_NR_backend_13_gateway_et_armoire_hors_ligne(t *testing.T) {
	r := sessionFor(gatewayInfo(false), armoireSeen(10*time.Minute))
	if r.Gateway == nil || r.Gateway.Online || r.Armoire == nil || r.Armoire.Online {
		t.Fatalf("tout doit être hors ligne : gateway=%+v armoire=%+v", r.Gateway, r.Armoire)
	}
}

// Cas D : sans gateway, armoire vue. C'est le signalement du 2026-10-26 (essensys-feature-lifecycle#37) :
// pas de bloc gateway, et l'armoire doit pourtant être en ligne.
func Test_NR_backend_13_sans_gateway_armoire_vue_en_ligne(t *testing.T) {
	r := sessionFor(nil, armoireSeen(10*time.Second))
	if r.Gateway != nil {
		t.Fatalf("pas de bloc gateway attendu : %+v", r.Gateway)
	}
	if r.Armoire == nil || !r.Armoire.Online {
		t.Fatalf("armoire.online attendu vrai sans gateway : %+v", r.Armoire)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["gateway"]; ok {
		t.Fatalf("le JSON ne doit pas contenir de bloc gateway : %s", raw)
	}
	armoire, _ := wire["armoire"].(map[string]any)
	if online, ok := armoire["online"].(bool); !ok || !online {
		t.Fatalf("le JSON doit porter armoire.online = true : %s", raw)
	}
}

// Cas E : sans gateway, armoire non vue depuis plus de 2 min.
func Test_NR_backend_13_sans_gateway_armoire_ancienne_hors_ligne(t *testing.T) {
	r := sessionFor(nil, armoireSeen(5*time.Minute))
	if r.Gateway != nil || r.Armoire == nil || r.Armoire.Online {
		t.Fatalf("armoire.online attendu faux : gateway=%+v armoire=%+v", r.Gateway, r.Armoire)
	}
}

// Cas F : ni gateway ni armoire, ou armoire jamais vue : rien n'est en ligne.
func Test_NR_backend_13_sans_gateway_ni_armoire_hors_ligne(t *testing.T) {
	r := sessionFor(nil, nil)
	if r.Gateway != nil || r.Armoire != nil {
		t.Fatalf("aucun bloc attendu : gateway=%+v armoire=%+v", r.Gateway, r.Armoire)
	}
	never := sessionFor(nil, &MachineDetail{ID: 7})
	if never.Armoire == nil || never.Armoire.Online {
		t.Fatalf("armoire jamais vue : online attendu faux : %+v", never.Armoire)
	}
	noAccess := NewPortalSessionResponse(PortalUserInfo{ID: 1}, false, nil, armoireSeen(time.Second), sessionNow)
	if noAccess.Armoire != nil {
		t.Fatalf("sans accès au portail, pas de bloc armoire : %+v", noAccess.Armoire)
	}
}
