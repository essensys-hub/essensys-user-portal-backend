package domain

import (
	"strconv"
	"testing"
)

// portalLight mirrors one entry of the lighting table of
// essensys-user-portal-frontend src/pages/LightingPage.tsx.
type portalLight struct {
	name    string
	mask    int
	on, off int
}

// portalMainLights is the "Éclairages principaux" table of the portal: several
// lamps share an index and only differ by their bit.
var portalMainLights = []portalLight{
	{"Terrasse", 4, 616, 610}, {"Entrée", 1, 611, 605}, {"Escalier", 1, 613, 607},
	{"Dégagement 1", 1, 616, 610}, {"Dégagement 2", 2, 616, 610}, {"Pièce de service", 128, 615, 609},
	{"Annexe 1", 8, 616, 610}, {"Annexe 2", 16, 616, 610}, {"Salon", 128, 612, 606},
	{"Salle à Manger", 64, 612, 606}, {"Cuisine", 1, 615, 609}, {"Salle de Bain 1", 128, 616, 610},
	{"Salle de Bain 2", 8, 615, 609}, {"WC 1", 32, 615, 609}, {"WC 2", 64, 615, 609},
	{"Bureau", 32, 612, 606}, {"Grande Chambre", 128, 614, 608}, {"Petite Chambre 1", 64, 614, 608},
	{"Petite Chambre 2", 32, 614, 608}, {"Petite Chambre 3", 16, 614, 608}, {"Dressing", 8, 611, 605},
}

// inject builds the action stored by POST /api/portal/inject for one command.
func inject(k, mask int) []ExchangeKV {
	return ExpandLegacyScenarioBlock([]ExchangeKV{{K: k, V: itoa(mask)}})
}

func itoa(n int) string { return strconv.Itoa(n) }

func values(params []ExchangeKV) map[int]string {
	seen := make(map[int]string, len(params))
	for _, p := range params {
		seen[p.K] = p.V
	}
	return seen
}

// allOnActions returns the actions the portal enqueues for "Tout allumer".
func allOnActions(lights []portalLight) [][]ExchangeKV {
	out := make([][]ExchangeKV, 0, len(lights))
	for _, l := range lights {
		out = append(out, inject(l.on, l.mask))
	}
	return out
}

func assertLit(t *testing.T, merged []ExchangeKV, lights []portalLight) {
	t.Helper()
	got := values(merged)
	for _, l := range lights {
		on, err := strconv.Atoi(got[l.on])
		if err != nil {
			t.Fatalf("%s: index %d = %q", l.name, l.on, got[l.on])
		}
		off, _ := strconv.Atoi(got[l.off])
		if on&l.mask == 0 {
			t.Errorf("%s: bit %d missing on index %d (got %q)", l.name, l.mask, l.on, got[l.on])
		}
		if off&l.mask != 0 {
			t.Errorf("%s: bit %d also set on off index %d", l.name, l.mask, l.off)
		}
	}
}

func TestMerge_tout_allumer_allume_les_21_lampes(t *testing.T) {
	merged := MergeBlockActions(allOnActions(portalMainLights)...)
	assertLit(t, merged, portalMainLights)
}

func TestMerge_allumer_puis_eteindre_eteint(t *testing.T) {
	merged := values(MergeBlockActions(inject(613, 64), inject(607, 64)))
	if merged[613] != "0" {
		t.Fatalf("613 should be 0 after OFF, got %q", merged[613])
	}
	if merged[607] != "64" {
		t.Fatalf("607 should be 64 after OFF, got %q", merged[607])
	}

	again := values(MergeBlockActions(inject(607, 64), inject(613, 64)))
	if again[613] != "64" || again[607] != "0" {
		t.Fatalf("off then on should be on, got 613=%q 607=%q", again[613], again[607])
	}
}

func TestMerge_deux_bits_sur_un_index_se_cumulent(t *testing.T) {
	merged := values(MergeBlockActions(inject(616, 1), inject(616, 4)))
	if merged[616] != "5" {
		t.Fatalf("616 should be 1|4 = 5, got %q", merged[616])
	}
}

func TestMerge_eteindre_une_lampe_garde_les_autres_allumees(t *testing.T) {
	merged := values(MergeBlockActions(inject(616, 4), inject(616, 1), inject(610, 4)))
	if merged[616] != "1" || merged[610] != "4" {
		t.Fatalf("want 616=1 610=4, got 616=%q 610=%q", merged[616], merged[610])
	}
}

func TestMerge_ouvrir_puis_fermer_un_volet_ferme(t *testing.T) {
	merged := values(MergeBlockActions(inject(619, 2), inject(622, 2)))
	if merged[619] != "0" || merged[622] != "2" {
		t.Fatalf("want 619=0 622=2, got 619=%q 622=%q", merged[619], merged[622])
	}
}

func TestMerge_bloc_complet_et_590_a_1(t *testing.T) {
	merged := MergeBlockActions(inject(613, 64), []ExchangeKV{{K: IndexScenario, V: "3"}, {K: 616, V: "2"}})
	got := values(merged)
	if got[IndexScenario] != "1" {
		t.Fatalf("590 must stay 1, got %q", got[IndexScenario])
	}
	for k := IndexLightStart; k <= IndexLightEnd; k++ {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing index %d", k)
		}
	}
	if got[IndexSecurityOutlets] != "0" {
		t.Fatalf("623 must be 0 after a merge, got %q", got[IndexSecurityOutlets])
	}
	if len(merged) != IndexSecurityOutlets-IndexLightStart+2 {
		t.Fatalf("block must have 20 params, got %d", len(merged))
	}
	if len(merged) > MaxFirmwareParamsPerAction {
		t.Fatalf("block exceeds firmware limit")
	}
}

func TestMerge_valeur_non_numerique_la_plus_recente(t *testing.T) {
	merged := values(MergeBlockActions(inject(616, 1), []ExchangeKV{{K: 616, V: "x"}}))
	if merged[616] != "x" {
		t.Fatalf("non-numeric value should win, got %q", merged[616])
	}
}

func TestOppositeBlockIndex_chaque_index_a_un_oppose(t *testing.T) {
	for k := IndexLightStart; k <= IndexLightEnd; k++ {
		o, ok := OppositeBlockIndex(k)
		if !ok {
			t.Fatalf("index %d has no opposite", k)
		}
		if back, _ := OppositeBlockIndex(o); back != k {
			t.Fatalf("pairing not symmetric: %d -> %d -> %d", k, o, back)
		}
		if o == k {
			t.Fatalf("index %d is its own opposite", k)
		}
	}
}

func TestIsPureBlockAction(t *testing.T) {
	if !IsPureBlockAction(inject(613, 64)) {
		t.Fatal("expanded light command is a pure block action")
	}
	if IsPureBlockAction([]ExchangeKV{{K: IndexScenario, V: "3"}}) {
		t.Fatal("scenario launch is not a block action")
	}
	if IsPureBlockAction(append(inject(613, 64), ExchangeKV{K: 349, V: "1"})) {
		t.Fatal("mixed action is not pure")
	}
	if IsPureBlockAction(inject(IndexSecurityOutlets, 1)) {
		t.Fatal("security outlet cut is not a pure block action")
	}
}
