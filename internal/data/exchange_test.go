package data

import (
	"testing"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

func TestParseKeyList(t *testing.T) {
	keys, err := ParseKeyList("605,606,613")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || keys[0] != 605 || keys[2] != 613 {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

func TestFilterExchangeKeys(t *testing.T) {
	all := []domain.ExchangeKV{{K: 605, V: "1"}, {K: 606, V: "0"}, {K: 613, V: "64"}}
	filtered := FilterExchangeKeys(all, []int{605, 613})
	if len(filtered) != 2 || filtered[0].K != 605 || filtered[1].K != 613 {
		t.Fatalf("unexpected filter: %v", filtered)
	}
}

func TestParseKeyListEmpty(t *testing.T) {
	if _, err := ParseKeyList(""); err == nil {
		t.Fatal("expected error for empty keys")
	}
}
