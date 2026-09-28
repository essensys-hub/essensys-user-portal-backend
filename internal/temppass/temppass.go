// Package temppass generates admin-issued temporary passwords: a secret
// meant to be read once off a screen and dictated over the phone, not typed
// from a saved copy. It owns no storage and no HTTP concern, the way pwreset
// owns token generation for the self-service reset flow.
package temppass

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Alphabet excludes O, 0, I, l, 1: characters that are easy to mishear or
// misread when a password is dictated aloud rather than copy-pasted. 57
// symbols over 12 characters is close to 70 bits of entropy (D4) — a
// generous margin for a secret that lives at most 72 hours and is expected
// to survive a single login.
const Alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// Length is the number of characters generated.
const Length = 12

// Generate returns a random password drawn from Alphabet, using rejection
// sampling via crypto/rand.Int so that a 57-symbol alphabet (which does not
// evenly divide any byte-aligned range) introduces no modulo bias.
func Generate() (string, error) {
	alphabetLen := big.NewInt(int64(len(Alphabet)))
	buf := make([]byte, Length)
	for i := range buf {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", fmt.Errorf("generate temporary password: %w", err)
		}
		buf[i] = Alphabet[n.Int64()]
	}
	return string(buf), nil
}
