package domain

import (
	"testing"
)

func TestExpandLegacyScenarioBlock_adds590And605to622(t *testing.T) {
	in := []ExchangeKV{{K: 613, V: "64"}}
	out := ExpandLegacyScenarioBlock(in)

	seen := map[int]string{}
	for _, p := range out {
		seen[p.K] = p.V
	}
	if seen[590] != "1" {
		t.Fatalf("expected 590=1, got %v", seen[590])
	}
	for i := 605; i <= 622; i++ {
		if _, ok := seen[i]; !ok {
			t.Fatalf("missing index %d", i)
		}
	}
	if seen[613] != "64" {
		t.Fatalf("expected 613=64, got %v", seen[613])
	}
}

func TestExpandLegacyScenarioBlock_noOpOutsideRange(t *testing.T) {
	in := []ExchangeKV{{K: 100, V: "1"}}
	out := ExpandLegacyScenarioBlock(in)
	if len(out) != 1 || out[0].K != 100 {
		t.Fatalf("unexpected expansion: %+v", out)
	}
}

// NR: NR-backend-10 essensys-hub/essensys-user-portal-backend#42
// security-outlets-2026-10-011: a lone security outlet
// command is completed into a full scenario 1 block.
func Test_NR_backend_10_coupure_seule_complete_590_et_605_a_622(t *testing.T) {
	for _, v := range []string{"1", "2"} {
		seen := map[int]string{}
		for _, p := range ExpandLegacyScenarioBlock([]ExchangeKV{{K: IndexSecurityOutlets, V: v}}) {
			seen[p.K] = p.V
		}
		if seen[IndexScenario] != "1" {
			t.Fatalf("623=%s: expected 590=1, got %q", v, seen[IndexScenario])
		}
		for i := IndexLightStart; i <= IndexLightEnd; i++ {
			if seen[i] != "0" {
				t.Fatalf("623=%s: expected %d=0, got %q", v, i, seen[i])
			}
		}
		if seen[IndexSecurityOutlets] != v {
			t.Fatalf("expected 623=%s, got %q", v, seen[IndexSecurityOutlets])
		}
	}
}

// NR: NR-backend-11 essensys-hub/essensys-user-portal-backend#42
// security-outlets-2026-10-011: a light command carries
// 623=0 and cannot replay a cut; a mixed batch keeps both values.
func Test_NR_backend_11_eclairage_ajoute_623_a_0(t *testing.T) {
	seen := map[int]string{}
	for _, p := range ExpandLegacyScenarioBlock([]ExchangeKV{{K: 612, V: "128"}}) {
		seen[p.K] = p.V
	}
	if seen[IndexSecurityOutlets] != "0" || seen[612] != "128" {
		t.Fatalf("light command: expected 612=128 and 623=0, got 612=%q 623=%q", seen[612], seen[IndexSecurityOutlets])
	}
	mixed := map[int]string{}
	for _, p := range ExpandLegacyScenarioBlock([]ExchangeKV{{K: 612, V: "128"}, {K: IndexSecurityOutlets, V: "1"}}) {
		mixed[p.K] = p.V
	}
	if mixed[612] != "128" || mixed[IndexSecurityOutlets] != "1" || mixed[IndexScenario] != "1" {
		t.Fatalf("mixed batch: got 590=%q 612=%q 623=%q", mixed[IndexScenario], mixed[612], mixed[IndexSecurityOutlets])
	}
}
