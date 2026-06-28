package data

import "testing"

func TestMachineDisplaySerie(t *testing.T) {
	hash := "5e6e0e1ffd940ee5649cf65b1d7a4df8"
	numeric := "15"
	got := machineDisplaySerie(&numeric, hash)
	want := "UNKNOWN-5e6e0e1f"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	unknown := "UNKNOWN-182aa020"
	if got2 := machineDisplaySerie(&unknown, hash); got2 != unknown {
		t.Fatalf("expected preserved %q, got %q", unknown, got2)
	}
}
