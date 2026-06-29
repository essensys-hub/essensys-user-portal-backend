package domain

import "testing"

func TestMergeExchangeParams_lastWinsToggle(t *testing.T) {
	on := ExpandLegacyScenarioBlock([]ExchangeKV{{K: 613, V: "64"}})
	off := ExpandLegacyScenarioBlock([]ExchangeKV{{K: 607, V: "64"}})
	merged := MergeExchangeParams(on, off)

	seen := map[int]string{}
	for _, p := range merged {
		seen[p.K] = p.V
	}
	if seen[613] != "0" {
		t.Fatalf("613 should be 0 after OFF, got %q", seen[613])
	}
	if seen[607] != "64" {
		t.Fatalf("607 should be 64 after OFF, got %q", seen[607])
	}
}

func TestMergeExchangeParams_singleChunk(t *testing.T) {
	in := []ExchangeKV{{K: 100, V: "1"}}
	out := MergeExchangeParams(in)
	if len(out) != 1 || out[0].K != 100 {
		t.Fatalf("unexpected: %+v", out)
	}
}
