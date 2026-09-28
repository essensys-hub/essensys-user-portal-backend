package temppass

import (
	"math"
	"strings"
	"testing"
)

func TestGenerate_LengthAndAlphabet(t *testing.T) {
	pw, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(pw) != Length {
		t.Fatalf("len(pw) = %d, want %d", len(pw), Length)
	}
	for _, c := range pw {
		if !strings.ContainsRune(Alphabet, c) {
			t.Fatalf("character %q not in Alphabet", c)
		}
	}
}

func TestGenerate_ExcludesAmbiguousCharacters(t *testing.T) {
	excluded := "O0Il1"
	for _, c := range excluded {
		if strings.ContainsRune(Alphabet, c) {
			t.Fatalf("Alphabet must not contain %q", c)
		}
	}

	const n = 1000
	for i := 0; i < n; i++ {
		pw, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		for _, c := range excluded {
			if strings.ContainsRune(pw, c) {
				t.Fatalf("generated password %q contains excluded character %q", pw, c)
			}
		}
	}
}

func TestGenerate_NoCollisionsAcrossManyDraws(t *testing.T) {
	const n = 1000
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		pw, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if seen[pw] {
			t.Fatalf("collision on draw %d: %q generated twice in %d draws", i, pw, n)
		}
		seen[pw] = true
	}
}

// TestGenerate_MinimumEntropy pins design decision D4: Alphabet and Length
// together must not silently drop below ~64 bits, the point at which 72h of
// exposure and a single expected use would no longer be a comfortable
// margin.
func TestGenerate_MinimumEntropy(t *testing.T) {
	bits := float64(Length) * math.Log2(float64(len(Alphabet)))
	const minBits = 64.0
	if bits < minBits {
		t.Fatalf("entropy = %.1f bits (alphabet=%d, length=%d), want >= %.1f",
			bits, len(Alphabet), Length, minBits)
	}
}
