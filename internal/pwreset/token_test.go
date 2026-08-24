package pwreset

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateProducesDistinctURLSafeTokens(t *testing.T) {
	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		plain, digest, err := Generate()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if seen[plain] {
			t.Fatal("generated a duplicate token")
		}
		seen[plain] = true

		if len(plain) != 43 {
			t.Fatalf("expected 43 chars for 256 bits, got %d", len(plain))
		}
		if strings.ContainsAny(plain, "+/=") {
			t.Fatalf("token must be URL-safe, got %q", plain)
		}
		if len(digest) != 64 {
			t.Fatalf("expected 64 hex chars, got %d", len(digest))
		}
		if strings.Contains(digest, plain) {
			t.Fatal("digest must not embed the clear-text token")
		}
	}
}

func TestHashIsStableAndNotReversible(t *testing.T) {
	if Hash("abc") != Hash("abc") {
		t.Fatal("hash must be deterministic")
	}
	if Hash("abc") == Hash("abd") {
		t.Fatal("distinct inputs must not collide here")
	}
	if strings.Contains(Hash("supersecret"), "supersecret") {
		t.Fatal("digest leaked the input")
	}
}

func TestBuildResetURL(t *testing.T) {
	cases := []struct{ base, token, want string }{
		{"https://mon.essensys.fr", "abc", "https://mon.essensys.fr/reset-password?token=abc"},
		{"https://mon.essensys.fr/", "abc", "https://mon.essensys.fr/reset-password?token=abc"},
		{"", "abc", "https://mon.essensys.fr/reset-password?token=abc"},
	}
	for _, c := range cases {
		if got := BuildResetURL(c.base, c.token); got != c.want {
			t.Fatalf("BuildResetURL(%q,%q) = %q, want %q", c.base, c.token, got, c.want)
		}
	}
}

func TestBuildResetURLEscapesToken(t *testing.T) {
	// RawURLEncoding never emits these, but the link must survive it anyway.
	got := BuildResetURL("https://x.test", "a b&c=d")
	if strings.Contains(got, " ") || strings.Contains(got, "&c=") {
		t.Fatalf("token was not escaped: %q", got)
	}
}

func TestPortalBaseURLFallsBackAndTrims(t *testing.T) {
	t.Setenv("FRONTEND_URL", "")
	if got := PortalBaseURL(); got != "https://mon.essensys.fr" {
		t.Fatalf("fallback: got %q", got)
	}
	t.Setenv("FRONTEND_URL", "https://staging.essensys.fr/")
	if got := PortalBaseURL(); got != "https://staging.essensys.fr" {
		t.Fatalf("trim: got %q", got)
	}
}

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"emilienbieber67260@gmail.com": "e***@gmail.com",
		"a@b.c":                        "*@b.c",
		"nicolas@rineau.eu":            "n***@rineau.eu",
		"garbage":                      "***",
	}
	for in, want := range cases {
		if got := MaskEmail(in); got != want {
			t.Fatalf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskEmailHidesLocalPartLength(t *testing.T) {
	short := MaskEmail("ab@x.test")
	long := MaskEmail("abcdefghijklmnop@x.test")
	if short != long {
		t.Fatalf("mask leaks local-part length: %q vs %q", short, long)
	}
}

func TestExpiresInMinutes(t *testing.T) {
	now := time.Now()
	if got := ExpiresInMinutes(now.Add(60*time.Minute), now); got != 59 && got != 60 {
		t.Fatalf("expected ~60, got %d", got)
	}
	if got := ExpiresInMinutes(now.Add(-time.Minute), now); got != 0 {
		t.Fatalf("expired token must report 0, got %d", got)
	}
}
