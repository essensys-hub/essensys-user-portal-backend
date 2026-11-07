package domain

import "testing"

// lumieres-temporisees-2026-11-003 (#53): catalogue and validation of the six
// light auto-off timers.
func TestLightTimerCatalogue_sixCles(t *testing.T) {
	want := map[string]int{"dressing": 521, "wc1": 555, "wc2": 556, "pieceserv": 557, "ann1": 561, "ann2": 562}
	got := LightTimers()
	if len(got) != len(want) {
		t.Fatalf("catalogue has %d lights, want %d", len(got), len(want))
	}
	for _, lt := range got {
		if k, ok := want[lt.ID]; !ok || k != lt.Key {
			t.Fatalf("light %q key %d, want %d", lt.ID, lt.Key, want[lt.ID])
		}
		if lt.Label == "" {
			t.Fatalf("light %q has no label", lt.ID)
		}
		if k, ok := LightTimerByID(lt.ID); !ok || k.Key != lt.Key {
			t.Fatalf("lookup %q failed", lt.ID)
		}
	}
	if _, ok := LightTimerByID("salon"); ok {
		t.Fatal("unknown id must not resolve")
	}
	if _, ok := LightTimerByID(""); ok {
		t.Fatal("empty id must not resolve")
	}
}

func TestLightTimerCatalogue_horsPlageScenario(t *testing.T) {
	for _, lt := range LightTimers() {
		if lt.Key >= IndexLightStart && lt.Key <= IndexSecurityOutlets {
			t.Fatalf("key %d is inside the scenario block 605..623", lt.Key)
		}
	}
}

func TestParseLightTimerMinutes(t *testing.T) {
	ok := map[string]int{"0": 0, "1": 1, "15": 15, "255": 255, " 30 ": 30}
	for raw, want := range ok {
		got, err := ParseLightTimerMinutes([]byte(raw))
		if err != nil || got != want {
			t.Errorf("%q: got %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"256", "-1", "7.5", "30.0", "1e2", `"30"`, `"abc"`, "null", "true", "[]", "{}", "", "0x10", "99999999999999999999"} {
		if _, err := ParseLightTimerMinutes([]byte(raw)); err == nil {
			t.Errorf("%q must be refused", raw)
		}
	}
}

func TestCanWriteLightTimers(t *testing.T) {
	m := 3
	cases := []struct {
		role    string
		machine *int
		want    bool
	}{
		{RoleAdminLocal, &m, true},
		{RoleAdminGlobal, &m, true},
		{RoleAdminLocal, nil, false},
		{RoleAdminGlobal, nil, false},
		{RoleUser, &m, false},
		{RoleGuestLocal, &m, false},
		{RoleSupport, &m, false},
		{"", &m, false},
	}
	for _, c := range cases {
		if got := CanWriteLightTimers(c.role, c.machine); got != c.want {
			t.Errorf("role %q machine %v: got %v want %v", c.role, c.machine, got, c.want)
		}
	}
}

func TestLightTimerMinutesFromCache(t *testing.T) {
	kv := []ExchangeKV{{K: 555, V: "15"}, {K: 556, V: "0"}, {K: 557, V: "abc"}, {K: 561, V: "300"}}
	if m := LightTimerMinutesFromCache(kv, 555); m == nil || *m != 15 {
		t.Fatalf("555: %v", m)
	}
	if m := LightTimerMinutesFromCache(kv, 556); m == nil || *m != 0 {
		t.Fatalf("556 explicit 0 must stay 0: %v", m)
	}
	for _, k := range []int{557, 561, 562, 521} {
		if m := LightTimerMinutesFromCache(kv, k); m != nil {
			t.Fatalf("key %d must be nil (unknown), got %d", k, *m)
		}
	}
}
