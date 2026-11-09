package data

import (
	"reflect"
	"testing"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

// NR-user-portal-backend-63 : la rotation serverinfos alterne des blocs d'indices ; chaque MyStatus ne contient
// que le bloc demandé. Sans fusion, le bloc suivant effaçait 440 (état des prises) de la télémétrie.
func TestMergeTelemetryEKKeepsKeysFromOtherChunks(t *testing.T) {
	existing := []domain.ExchangeKeyValue{{K: 440, V: "1"}, {K: 590, V: "0"}}
	incoming := []domain.ExchangeKeyValue{{K: 0, V: "7"}, {K: 590, V: "1"}}
	got := MergeTelemetryEK(existing, incoming)
	want := []domain.ExchangeKeyValue{{K: 0, V: "7"}, {K: 440, V: "1"}, {K: 590, V: "1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeTelemetryEKWithoutExisting(t *testing.T) {
	incoming := []domain.ExchangeKeyValue{{K: 9, V: "a"}, {K: 1, V: "b"}}
	got := MergeTelemetryEK(nil, incoming)
	want := []domain.ExchangeKeyValue{{K: 1, V: "b"}, {K: 9, V: "a"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
