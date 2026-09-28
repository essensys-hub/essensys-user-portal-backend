package domain

import (
	"encoding/json"
	"net/http"
	"time"
)

const ForbiddenRedirectPath = "/maintenance/"
const ChangePasswordRedirectPath = "/change-password"

func IsUserForbidden(u *User) bool {
	return u != nil && u.ForbiddenAt != nil
}

func WriteAccountForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":    "account_forbidden",
		"redirect": ForbiddenRedirectPath,
	})
}

// PasswordChangeRequired reports whether the account is carrying an
// admin-issued temporary password that has not yet been replaced. It stays
// true regardless of TempPasswordExpiresAt: expiry only affects whether the
// temporary password can still be used to log in (see TempPasswordExpired),
// not whether an already-open session is locked out.
func PasswordChangeRequired(u *User) bool {
	return u != nil && u.PasswordChangeRequiredAt != nil
}

// TempPasswordExpired reports whether a still-pending temporary password has
// passed its validity window. A nil TempPasswordExpiresAt (no temporary
// password issued) is never expired.
func TempPasswordExpired(u *User, now time.Time) bool {
	return u != nil && u.TempPasswordExpiresAt != nil && now.After(*u.TempPasswordExpiresAt)
}

func WritePasswordChangeRequired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":    "password_change_required",
		"redirect": ChangePasswordRedirectPath,
	})
}
