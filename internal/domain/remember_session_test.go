package domain

import (
	"testing"
	"time"
)

func TestRememberSessionUsable(t *testing.T) {
	created := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	expires := created.Add(30 * 24 * time.Hour)
	revoked := created.Add(time.Hour)
	cases := []struct {
		name string
		s    *RememberSession
		now  time.Time
		want bool
	}{
		{"day 12", &RememberSession{ExpiresAt: expires}, created.Add(12 * 24 * time.Hour), true},
		{"one minute before the end", &RememberSession{ExpiresAt: expires}, expires.Add(-time.Minute), true},
		{"at the end", &RememberSession{ExpiresAt: expires}, expires, false},
		{"day 30 and one minute", &RememberSession{ExpiresAt: expires}, expires.Add(time.Minute), false},
		{"revoked", &RememberSession{ExpiresAt: expires, RevokedAt: &revoked}, created.Add(2 * time.Hour), false},
		{"nil", nil, created, false},
	}
	for _, c := range cases {
		if got := c.s.Usable(c.now); got != c.want {
			t.Errorf("%s: Usable = %v, want %v", c.name, got, c.want)
		}
	}
}
