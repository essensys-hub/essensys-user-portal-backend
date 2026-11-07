package domain

import (
	"strings"
	"testing"
	"time"
)

// noms-lumieres-2026-11-004 (#55): catalogue and name rule.

func TestLightNames_catalogue33IdentifiantsExacts(t *testing.T) {
	want := strings.Fields(`terrasse entree escalier deg1 deg2 pieceserv ann1 ann2 salon sam cuisine
		sdb1 sdb2 wc1 wc2 bureau gdchamb ptchamb1 ptchamb2 ptchamb3 dressing
		isalonind isalonind2 icuisine isdb1 isdb2 igdchamb1 igdchamb2 iptchamb1 iptchamb2 iptchamb22 iptchamb3 idressing`)
	got := LightNameCatalog()
	if len(want) != 33 || len(got) != 33 {
		t.Fatalf("want 33 lights, catalogue has %d (expected list %d)", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("catalogue[%d] = %q, want %q", i, got[i].ID, id)
		}
		if got[i].Default == "" {
			t.Fatalf("%s has no default name", id)
		}
	}
	if l, ok := LightNameByID("dressing"); !ok || l.Default != "Dressing" {
		t.Fatalf("dressing: %+v %v", l, ok)
	}
	if l, _ := LightNameByID("iptchamb2"); l.Default != "Petite Chambre 1 (chevet 2)" {
		t.Fatalf("iptchamb2 default %q", l.Default)
	}
	if _, ok := LightNameByID("salon2"); ok {
		t.Fatal("unknown id must not resolve")
	}
	got[0].ID = "mutated"
	if LightNameCatalog()[0].ID != "terrasse" {
		t.Fatal("the catalogue must be returned as a copy")
	}
}

func TestNormalizeLightName(t *testing.T) {
	ok := []struct{ in, want string }{
		{"WC 3", "WC 3"},
		{"  WC 3  ", "WC 3"},
		{strings.Repeat("a", 40), strings.Repeat("a", 40)},
		{strings.Repeat("é", 40), strings.Repeat("é", 40)}, // 40 runes, 80 bytes
		{"Chambre d'ami \"nord\" & co", "Chambre d'ami \"nord\" & co"},
	}
	for _, c := range ok {
		got, err := NormalizeLightName(c.in)
		if err != nil || got == nil || *got != c.want {
			t.Fatalf("%q: got %v err %v, want %q", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"", "   ", "\t \n"} {
		got, err := NormalizeLightName(in)
		if err != nil || got != nil {
			t.Fatalf("%q must reset (nil, nil), got %v %v", in, got, err)
		}
	}
	bad := []string{
		strings.Repeat("a", 41),
		strings.Repeat("é", 41),
		"<b>x</b>", "a>b", "a<b",
		"a\x00b", "a\x1fb", "a\x7fb", "a\nb", "a\tb",
	}
	for _, in := range bad {
		if _, err := NormalizeLightName(in); err != ErrLightName {
			t.Fatalf("%q must be refused, got %v", in, err)
		}
	}
}

func TestCanWriteLightNames_memeRegleQueTemporisations(t *testing.T) {
	m := 3
	for role, want := range map[string]bool{"admin_local": true, "admin_global": true, "user": false, "guest_local": false, "support": false} {
		if CanWriteLightNames(role, &m) != want {
			t.Fatalf("%s with cabinet: want %v", role, want)
		}
		if CanWriteLightNames(role, nil) {
			t.Fatalf("%s without cabinet must be refused", role)
		}
	}
}

func TestMergeLightName(t *testing.T) {
	base := time.Date(2026, 11, 7, 10, 15, 0, 0, time.UTC)
	s := func(v string) *string { return &v }
	cloud := LightNameState{Name: s("WC 3"), Version: 2, UpdatedAt: base, UpdatedBy: "a@example.org"}
	cases := []struct {
		name     string
		gw       LightNameState
		gwWins   bool
		wantName string
	}{
		{"cloud plus recent", LightNameState{Name: s("Dressing 2"), Version: 5, UpdatedAt: base.Add(-5 * time.Minute)}, false, "WC 3"},
		{"gateway plus recente", LightNameState{Name: s("WC Haut"), Version: 1, UpdatedAt: base.Add(5 * time.Minute)}, true, "WC Haut"},
		{"meme heure, version plus grande", LightNameState{Name: s("V3"), Version: 3, UpdatedAt: base}, true, "V3"},
		{"meme heure, version plus petite", LightNameState{Name: s("V1"), Version: 1, UpdatedAt: base}, false, "WC 3"},
		{"meme heure et version, auteur plus grand", LightNameState{Name: s("Z"), Version: 2, UpdatedAt: base, UpdatedBy: "b@example.org"}, true, "Z"},
		{"egalite parfaite : le cloud reste", LightNameState{Name: s("Autre"), Version: 2, UpdatedAt: base, UpdatedBy: "a@example.org"}, false, "WC 3"},
	}
	for _, c := range cases {
		if got := LightNameNewer(c.gw, cloud); got != c.gwWins {
			t.Fatalf("%s: gateway wins = %v, want %v", c.name, got, c.gwWins)
		}
	}
	// A reset (nil name) is a value like another and wins when it is newer.
	reset := LightNameState{Name: nil, Version: 3, UpdatedAt: base.Add(time.Minute)}
	if !LightNameNewer(reset, cloud) {
		t.Fatal("a newer reset must win")
	}
}

func TestClampLightNameTime(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	if got := ClampLightNameTime(now.Add(5*time.Minute), now); !got.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("exactly 5 minutes ahead is accepted, got %v", got)
	}
	if got := ClampLightNameTime(now.Add(5*time.Minute+time.Second), now); !got.Equal(now) {
		t.Fatalf("more than 5 minutes ahead must be brought back to now, got %v", got)
	}
	past := now.Add(-time.Hour)
	if got := ClampLightNameTime(past, now); !got.Equal(past) {
		t.Fatal("the past is kept")
	}
}
