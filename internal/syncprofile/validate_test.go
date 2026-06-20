package syncprofile

import (
	"testing"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

func TestPullChunkCount_SDB1(t *testing.T) {
	ranges := []domain.IndexRange{{181, 264}}
	if got := PullChunkCount(ranges); got != 3 {
		t.Fatalf("expected 3 chunks, got %d", got)
	}
	if got := ExpectedIndexCount(ranges); got != 84 {
		t.Fatalf("expected 84 indices, got %d", got)
	}
}

func TestValidateIndexRanges_invalid(t *testing.T) {
	err := ValidateIndexRanges([]domain.IndexRange{{10, 5}})
	if err == nil {
		t.Fatal("expected error")
	}
}
