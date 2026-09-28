package domain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPasswordChangeRequired(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)

	cases := []struct {
		name string
		user *User
		want bool
	}{
		{"nil user", nil, false},
		{"no flag set", &User{}, false},
		{"flag set, expiry irrelevant", &User{PasswordChangeRequiredAt: &now, TempPasswordExpiresAt: &future}, true},
		{"flag set, no expiry recorded", &User{PasswordChangeRequiredAt: &now}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PasswordChangeRequired(tc.user); got != tc.want {
				t.Errorf("PasswordChangeRequired() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTempPasswordExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	cases := []struct {
		name string
		user *User
		want bool
	}{
		{"nil user", nil, false},
		{"no temporary password issued", &User{}, false},
		{"not yet expired", &User{TempPasswordExpiresAt: &future}, false},
		{"expired", &User{TempPasswordExpiresAt: &past}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := TempPasswordExpired(tc.user, now); got != tc.want {
				t.Errorf("TempPasswordExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWritePasswordChangeRequired(t *testing.T) {
	w := httptest.NewRecorder()
	WritePasswordChangeRequired(w)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "password_change_required" {
		t.Errorf(`error = %q, want "password_change_required"`, body["error"])
	}
	if body["redirect"] != ChangePasswordRedirectPath {
		t.Errorf("redirect = %q, want %q", body["redirect"], ChangePasswordRedirectPath)
	}
}
