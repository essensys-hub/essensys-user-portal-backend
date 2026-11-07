package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// affichage-objets-2026-11-006 (#59): modèle de disposition, validation et
// fonction effective.

func TestObjectLayout_catalogues33Et16IdentifiantsExacts(t *testing.T) {
	lights := strings.Fields(`terrasse entree escalier deg1 deg2 pieceserv ann1 ann2 salon sam cuisine sdb1 sdb2
		wc1 wc2 bureau gdchamb ptchamb1 ptchamb2 ptchamb3 dressing isalonind isalonind2 icuisine isdb1 isdb2
		igdchamb1 igdchamb2 iptchamb1 iptchamb2 iptchamb22 iptchamb3 idressing`)
	shutters := strings.Fields(`volet1salon volet2salon volet3salon volet1salleamanger volet2salleamanger
		volet1cuisine volet2cuisine voletsdb volet1gdchamb volet2gdchamb volet1ptchamb volet2ptchamb
		volet3ptchamb voletbureau voletstore store`)
	if len(lights) != 33 || len(shutters) != 16 {
		t.Fatalf("liste attendue : %d lumières, %d volets", len(lights), len(shutters))
	}
	for kind, want := range map[LayoutKind][]string{LayoutLights: lights, LayoutShutters: shutters} {
		got := LayoutCatalog(kind)
		if len(got) != len(want) {
			t.Fatalf("%s : %d identifiants, attendu %d", kind, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s[%d] = %q, attendu %q", kind, i, got[i], want[i])
			}
		}
		got[0] = "mute"
		if LayoutCatalog(kind)[0] == "mute" {
			t.Fatal("le catalogue doit être rendu en copie")
		}
	}
	if LayoutCatalog("autre") != nil {
		t.Fatal("un genre inconnu n'a pas de catalogue")
	}
}

func TestValidateLayout(t *testing.T) {
	cases := []struct {
		name     string
		kind     LayoutKind
		order    []string
		disabled []string
		want     error
	}{
		{"vide accepté", LayoutLights, nil, nil, nil},
		{"order incomplet accepté", LayoutLights, []string{"salon", "entree"}, []string{"wc2"}, nil},
		{"identifiant inconnu", LayoutLights, []string{"salon", "nope"}, nil, ErrUnknownLight},
		{"identifiant inconnu dans disabled", LayoutLights, nil, []string{"nope"}, ErrUnknownLight},
		{"identifiant de volet refusé pour les lumières", LayoutLights, []string{"volet1salon"}, nil, ErrUnknownLight},
		{"identifiant de lumière refusé pour les volets", LayoutShutters, nil, []string{"salon"}, ErrUnknownShutter},
		{"doublon dans order", LayoutShutters, []string{"store", "store"}, nil, ErrInvalidLayout},
		{"doublon dans disabled", LayoutShutters, nil, []string{"store", "store"}, ErrInvalidLayout},
		{"même id dans order et disabled accepté", LayoutShutters, []string{"store"}, []string{"store"}, nil},
		{"tout désactivé accepté (33)", LayoutLights, nil, LayoutCatalog(LayoutLights), nil},
		{"tout désactivé accepté (16)", LayoutShutters, LayoutCatalog(LayoutShutters), LayoutCatalog(LayoutShutters), nil},
		{"clé d'échange refusée", LayoutLights, []string{"556"}, nil, ErrUnknownLight},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateLayout(c.kind, c.order, c.disabled)
			if !errors.Is(err, c.want) && !(err == nil && c.want == nil) {
				t.Fatalf("erreur %v, attendu %v", err, c.want)
			}
		})
	}
}

func TestEffectiveLayout(t *testing.T) {
	cat := LayoutCatalog(LayoutShutters)
	t.Run("défaut = ordre du catalogue, rien de désactivé", func(t *testing.T) {
		ordered, disabled := EffectiveLayout(LayoutShutters, nil, nil)
		if strings.Join(ordered, " ") != strings.Join(cat, " ") || len(disabled) != 0 {
			t.Fatalf("%v %v", ordered, disabled)
		}
	})
	t.Run("order incomplet : le reste suit dans l'ordre du catalogue", func(t *testing.T) {
		ordered, _ := EffectiveLayout(LayoutShutters, []string{"store", "volet3salon"}, nil)
		if ordered[0] != "store" || ordered[1] != "volet3salon" || ordered[2] != "volet1salon" || ordered[3] != "volet2salon" || len(ordered) != 16 {
			t.Fatalf("%v", ordered)
		}
	})
	t.Run("identifiants inconnus et doublons ignorés", func(t *testing.T) {
		ordered, disabled := EffectiveLayout(LayoutShutters, []string{"zzz", "store", "store", "salon"}, []string{"nope", "voletbureau", "voletbureau"})
		if ordered[0] != "store" || len(ordered) != 16 {
			t.Fatalf("%v", ordered)
		}
		if len(disabled) != 1 || !disabled["voletbureau"] {
			t.Fatalf("%v", disabled)
		}
	})
	t.Run("tout désactivé", func(t *testing.T) {
		_, disabled := EffectiveLayout(LayoutShutters, nil, cat)
		if len(disabled) != 16 {
			t.Fatalf("%d", len(disabled))
		}
	})
}

func TestResetEtDefaut(t *testing.T) {
	d := DefaultObjectLayout()
	if d.Version != 0 || !d.UpdatedAt.IsZero() || len(d.Order) != 0 || len(d.Disabled) != 0 {
		t.Fatalf("%+v", d)
	}
	if d.Order == nil || d.Disabled == nil {
		t.Fatal("les listes du défaut doivent être vides et non nulles (JSON [])")
	}
}

func TestLayoutNewer_memeRegleQueLesNoms(t *testing.T) {
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	base := ObjectLayout{Version: 2, UpdatedAt: at, UpdatedBy: "a"}
	if LayoutNewer(base, base) {
		t.Fatal("une égalité parfaite n'est pas plus récente")
	}
	if !LayoutNewer(ObjectLayout{Version: 1, UpdatedAt: at.Add(time.Second)}, base) {
		t.Fatal("updated_at prime sur la version")
	}
	if !LayoutNewer(ObjectLayout{Version: 3, UpdatedAt: at}, base) {
		t.Fatal("à date égale la version prime")
	}
	if !LayoutNewer(ObjectLayout{Version: 2, UpdatedAt: at, UpdatedBy: "b"}, base) {
		t.Fatal("à version égale updated_by prime")
	}
}

func TestCanWriteLayout_memeRegleQueLesNoms(t *testing.T) {
	m := 3
	if !CanWriteLayout("admin_local", &m) || !CanWriteLayout("admin_global", &m) {
		t.Fatal("l'administrateur avec armoire liée écrit")
	}
	for _, r := range []string{"user", "guest_local", "support"} {
		if CanWriteLayout(r, &m) {
			t.Fatalf("%s ne doit pas écrire", r)
		}
	}
	if CanWriteLayout("admin_local", nil) {
		t.Fatal("sans armoire liée, pas d'écriture")
	}
}
