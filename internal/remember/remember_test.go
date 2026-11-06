package remember

import (
	"encoding/base64"
	"testing"
)

func TestGenerateYields256BitsAndMatchingDigest(t *testing.T) {
	plain, digest, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(plain)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token must be 32 random bytes in base64 URL, got %d bytes, err=%v", len(raw), err)
	}
	if digest != Hash(plain) || len(digest) != 64 {
		t.Fatalf("digest must be the 64-char SHA-256 of the token, got %q", digest)
	}
	if plain == digest {
		t.Fatal("the stored digest must differ from the clear token")
	}
}

func TestGenerateIsRandom(t *testing.T) {
	a, _, _ := Generate()
	b, _, _ := Generate()
	if a == b {
		t.Fatal("two tokens must differ")
	}
}
