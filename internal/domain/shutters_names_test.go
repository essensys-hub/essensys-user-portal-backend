package domain

import (
	"strings"
	"testing"
)

// noms-volets-2026-11-005 (#57): catalogue and name rule.

func TestShutterNames_catalogue16IdentifiantsExacts(t *testing.T) {
	want := strings.Fields(`volet1salon volet2salon volet3salon volet1salleamanger volet2salleamanger
		volet1cuisine volet2cuisine voletsdb volet1gdchamb volet2gdchamb volet1ptchamb volet2ptchamb
		volet3ptchamb voletbureau voletstore store`)
	got := ShutterNameCatalog()
	if len(want) != 16 || len(got) != 16 {
		t.Fatalf("want 16 shutters, catalogue has %d (expected list %d)", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("catalogue[%d] = %q, want %q", i, got[i].ID, id)
		}
		if got[i].Default == "" {
			t.Fatalf("%s has no default name", id)
		}
	}
	if s, ok := ShutterNameByID("voletstore"); !ok || s.Default != `Volet "Store"` {
		t.Fatalf("voletstore must keep its straight quotes: %+v %v", s, ok)
	}
	if s, _ := ShutterNameByID("store"); s.Default != "Store (banne)" {
		t.Fatalf("store default %q", s.Default)
	}
	if s, _ := ShutterNameByID("voletsdb"); s.Default != "Volet Salle de Bain 1" {
		t.Fatalf("voletsdb default %q", s.Default)
	}
	for _, id := range []string{"volet99", "salon", "dressing", "556"} {
		if _, ok := ShutterNameByID(id); ok {
			t.Fatalf("%s must not resolve (light or unknown id)", id)
		}
	}
	got[0].ID = "mutated"
	if ShutterNameCatalog()[0].ID != "volet1salon" {
		t.Fatal("the catalogue must be returned as a copy")
	}
}

func TestNormalizeShutterName(t *testing.T) {
	ok := []struct{ in, want string }{
		{"Volet baie vitrée", "Volet baie vitrée"},
		{"  Volet baie vitrée  ", "Volet baie vitrée"},
		{strings.Repeat("a", 40), strings.Repeat("a", 40)},
		{strings.Repeat("é", 40), strings.Repeat("é", 40)},
		{`Store "nord" & co`, `Store "nord" & co`},
	}
	for _, c := range ok {
		got, err := NormalizeShutterName(c.in)
		if err != nil || got == nil || *got != c.want {
			t.Fatalf("%q: got %v err %v, want %q", c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"", "   ", "\t \n"} {
		got, err := NormalizeShutterName(in)
		if err != nil || got != nil {
			t.Fatalf("%q must reset (nil, nil), got %v %v", in, got, err)
		}
	}
	for _, in := range []string{strings.Repeat("a", 41), "<b>x</b>", "a>b", "a<b", "a\x00b", "a\x7fb", "a\nb"} {
		if _, err := NormalizeShutterName(in); err != ErrShutterName {
			t.Fatalf("%q must be refused, got %v", in, err)
		}
	}
}

func TestCanWriteShutterNames_memeRegleQueLumieres(t *testing.T) {
	m := 3
	for role, want := range map[string]bool{"admin_local": true, "admin_global": true, "user": false, "guest_local": false, "support": false} {
		if CanWriteShutterNames(role, &m) != want {
			t.Fatalf("%s with cabinet: want %v", role, want)
		}
		if CanWriteShutterNames(role, nil) {
			t.Fatalf("%s without cabinet must be refused", role)
		}
	}
}
