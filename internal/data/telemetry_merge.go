package data

import (
	"sort"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

// MergeTelemetryEK combines the keys already stored for a machine with those of its latest MyStatus.
// The serverinfos rotation asks for a different chunk of indices on each poll, so a MyStatus only carries
// that chunk: replacing the stored keys would erase the others (essensys-user-portal-backend#63).
// Incoming values win; the result is sorted by key.
func MergeTelemetryEK(existing, incoming []domain.ExchangeKeyValue) []domain.ExchangeKeyValue {
	byKey := make(map[int]string, len(existing)+len(incoming))
	for _, kv := range existing {
		byKey[kv.K] = kv.V
	}
	for _, kv := range incoming {
		byKey[kv.K] = kv.V
	}
	out := make([]domain.ExchangeKeyValue, 0, len(byKey))
	for k, v := range byKey {
		out = append(out, domain.ExchangeKeyValue{K: k, V: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].K < out[j].K })
	return out
}
