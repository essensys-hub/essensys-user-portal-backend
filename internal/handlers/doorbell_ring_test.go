package handlers

import (
	"testing"
	"time"
)

// sonnette-plages-push-2026-11-008 (#61): one ring per gateway and 10 s.
func TestRingLimiter_unParDixSecondesEtParGateway(t *testing.T) {
	l := newRingLimiter(10 * time.Second)
	t0 := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	if !l.allow("gw-1", t0) {
		t.Fatal("first ring must pass")
	}
	if l.allow("gw-1", t0.Add(9*time.Second)) {
		t.Fatal("a second ring within 10 s must be refused")
	}
	if !l.allow("gw-2", t0.Add(time.Second)) {
		t.Fatal("another gateway has its own window")
	}
	if !l.allow("gw-1", t0.Add(10*time.Second)) {
		t.Fatal("a ring 10 s after the last accepted one must pass")
	}
	if l.allow("gw-1", t0.Add(11*time.Second)) {
		t.Fatal("the window restarts at the last accepted ring, not at the refused one")
	}
}

func TestRingLimiter_oublieLesGatewaysInactives(t *testing.T) {
	l := newRingLimiter(time.Second)
	t0 := time.Now()
	l.allow("old", t0)
	l.allow("new", t0.Add(time.Minute))
	if _, ok := l.last["old"]; ok {
		t.Fatal("idle gateway must be forgotten")
	}
}
