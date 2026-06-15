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

func TestMergeExchangeKeys_prefersPrimary(t *testing.T) {
	primary := []domain.ExchangeKV{{K: 566, V: "120"}, {K: 605, V: "1"}}
	secondary := []domain.ExchangeKV{{K: 566, V: "99"}, {K: 567, V: "130"}}
	merged := MergeExchangeKeys(primary, secondary, []int{566, 567, 605})
	if len(merged) != 3 {
		t.Fatalf("expected 3 keys, got %v", merged)
	}
	if merged[0].K != 566 || merged[0].V != "120" {
		t.Fatalf("primary should win for 566: %v", merged)
	}
	if merged[1].K != 567 || merged[1].V != "130" {
		t.Fatalf("missing secondary 567: %v", merged)
	}
}

func TestParseKeyListEmpty(t *testing.T) {
	if _, err := ParseKeyList(""); err == nil {
		t.Fatal("expected error for empty keys")
	}
}
