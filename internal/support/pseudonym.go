package support

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"strconv"
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// UserPseudonym is the stable public reference of an account (D5): the same
// account always gets the same U-xxxxxxxx, and nobody can map it back to the
// account without the server-side key.
func UserPseudonym(key []byte, userID int) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(strconv.Itoa(userID)))
	return "U-" + b32.EncodeToString(mac.Sum(nil))[:8]
}

// NewReportRef returns a random public reference for one report.
func NewReportRef() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "R-" + b32.EncodeToString(buf), nil
}
