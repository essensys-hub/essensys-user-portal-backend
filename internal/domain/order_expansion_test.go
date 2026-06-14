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
