package data

import (
	"testing"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

func TestParseMACFromEK(t *testing.T) {
	ek := []domain.ExchangeKeyValue{
		{K: 947, V: "0"}, {K: 948, V: "224"}, {K: 949, V: "76"}, {K: 950, V: "104"}, {K: 951, V: "1"}, {K: 952, V: "190"},
	}
	got := ParseMACFromEK(ek)
	want := "00:e0:4c:68:01:be"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if ParseMACFromEK(nil) != "" {
		t.Fatal("expected empty for nil ek")
	}
}
