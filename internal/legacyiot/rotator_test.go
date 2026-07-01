package legacyiot

import "testing"

func TestInfoRotatorRespectsFirmwareLimit(t *testing.T) {
	r := NewInfoRotator()
	for i := 0; i < 4; i++ {
		indices := r.Next()
		if len(indices) > 30 {
			t.Fatalf("poll %d: %d indices exceeds firmware limit 30", i, len(indices))
		}
		if len(indices) == 0 {
			t.Fatalf("poll %d: empty indices", i)
		}
	}
}

func TestInfoRotatorAlternatesIdentity(t *testing.T) {
	r := NewInfoRotator()
	first := r.Next()
	second := r.Next()
	if first[0] != DefaultCommandIndices[0] {
		t.Fatalf("expected default chunk first, got %v", first)
	}
	if second[0] != IdentityIndices[0] {
		t.Fatalf("expected identity chunk second, got %v", second)
	}
}
