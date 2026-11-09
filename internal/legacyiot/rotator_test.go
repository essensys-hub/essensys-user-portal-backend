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

// NR-user-portal-backend-63 : sans gateway, l'armoire ne remontait jamais l'état des prises de sécurité (440),
// l'app affichait « État inconnu ». 440 doit être demandé dans un cycle complet de la rotation.
func TestInfoRotatorAsksSecurityOutletsState(t *testing.T) {
	r := NewInfoRotator()
	for i := 0; i < 6; i++ {
		indices := r.Next()
		if len(indices) > 30 {
			t.Fatalf("poll %d: %d indices exceeds firmware limit 30", i, len(indices))
		}
		for _, k := range indices {
			if k == IndexSecurityOutletsState {
				return
			}
		}
	}
	t.Fatalf("index %d (Securite_PriseCoupe) never requested in 6 polls", IndexSecurityOutletsState)
}
