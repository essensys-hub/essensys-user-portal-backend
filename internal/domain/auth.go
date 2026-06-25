package domain

import (
	"encoding/json"
	"net/http"
)

const ForbiddenRedirectPath = "/maintenance/"

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
