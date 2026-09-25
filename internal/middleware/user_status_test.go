package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

type fakeUserStore struct {
	users map[string]*domain.User
	err   error
}

func (f *fakeUserStore) GetUserByEmail(email string) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	u, ok := f.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func newFakeStore(u *domain.User) *fakeUserStore {
	return &fakeUserStore{users: map[string]*domain.User{"user@example.com": u}}
}

// TestEnforceActiveUser_FourCombinations covers D2's priority rule directly:
// forbidden and password-change-required are independent flags, but a
// forbidden account must never leak past the lock into "just needs a
// password change" — being forbidden is the stronger statement.
func TestEnforceActiveUser_FourCombinations(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name       string
		user       *domain.User
		wantOK     bool
		wantStatus int
		wantError  string
	}{
		{
			name:   "no state",
			user:   &domain.User{Email: "user@example.com", Role: "user"},
			wantOK: true,
		},
		{
			name:       "forbidden only",
			user:       &domain.User{Email: "user@example.com", Role: "user", ForbiddenAt: &now},
			wantOK:     false,
			wantStatus: http.StatusForbidden,
			wantError:  "account_forbidden",
		},
		{
			name:       "password change required only",
			user:       &domain.User{Email: "user@example.com", Role: "user", PasswordChangeRequiredAt: &now},
			wantOK:     false,
			wantStatus: http.StatusConflict,
			wantError:  "password_change_required",
		},
		{
			name: "both — forbidden takes priority",
			user: &domain.User{
				Email:                    "user@example.com",
				Role:                     "user",
				ForbiddenAt:              &now,
				PasswordChangeRequiredAt: &now,
			},
			wantOK:     false,
			wantStatus: http.StatusForbidden,
			wantError:  "account_forbidden",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore(tc.user)
			rec := httptest.NewRecorder()

			user, ok := enforceActiveUser(rec, store, "user@example.com")

			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.wantOK {
				if user == nil {
					t.Fatal("expected non-nil user on success")
				}
				return
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Body.String(); !strings.Contains(got, tc.wantError) {
				t.Fatalf("body = %q, want it to contain %q", got, tc.wantError)
			}
		})
	}
}

func TestEnforceActiveUser_StoreError(t *testing.T) {
	store := &fakeUserStore{err: errors.New("boom")}
	rec := httptest.NewRecorder()

	_, ok := enforceActiveUser(rec, store, "user@example.com")
	if ok {
		t.Fatal("expected ok=false on store error")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestEnforceActiveUser_UnknownAccount(t *testing.T) {
	store := &fakeUserStore{users: map[string]*domain.User{}}
	rec := httptest.NewRecorder()

	_, ok := enforceActiveUser(rec, store, "user@example.com")
	if ok {
		t.Fatal("expected ok=false for unknown account")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestUserJWTAllowPasswordChange_ExemptsOnlyThisRoute is the direct check for
// D3: an account marked as needing a password change is blocked by the
// ordinary middleware but let through by the dedicated exemption, and the
// exemption still enforces the forbidden-account check shared via
// resolveActiveUser.
func TestUserJWTAllowPasswordChange_ExemptsOnlyThisRoute(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key")
	token, err := GenerateJWT("user@example.com", "user", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	pendingChange := &domain.User{Email: "user@example.com", Role: "user", PasswordChangeRequiredAt: &now}

	t.Run("UserJWTWithStore blocks it", func(t *testing.T) {
		store := newFakeStore(pendingChange)
		called := false
		h := UserJWTWithStore(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if called {
			t.Fatal("handler should not run when password change is required")
		}
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
		}
	})

	t.Run("UserJWTAllowPasswordChange lets it through", func(t *testing.T) {
		store := newFakeStore(pendingChange)
		called := false
		h := UserJWTAllowPasswordChange(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/api/auth/password/change", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if !called {
			t.Fatal("handler should run under the password-change exemption")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("UserJWTAllowPasswordChange still blocks forbidden accounts", func(t *testing.T) {
		forbidden := &domain.User{Email: "user@example.com", Role: "user", ForbiddenAt: &now}
		store := newFakeStore(forbidden)
		called := false
		h := UserJWTAllowPasswordChange(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/api/auth/password/change", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if called {
			t.Fatal("handler should not run for a forbidden account")
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})
}

// mutableUserStore lets a test change what GetUserByEmail returns between
// requests, simulating an admin issuing a temporary password mid-session.
type mutableUserStore struct {
	user *domain.User
}

func (m *mutableUserStore) GetUserByEmail(email string) (*domain.User, error) {
	return m.user, nil
}

// TestPasswordLock_HasNoDeferredEffect is the direct check for D2's claim
// that the lock is immediate: the JWT is opaque about password state (it was
// signed before any temporary password existed), so the only thing that can
// make the second request fail is enforceActiveUser reloading the account
// fresh — no token reissue, no waiting for expiry.
func TestPasswordLock_HasNoDeferredEffect(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key")
	token, err := GenerateJWT("user@example.com", "user", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	store := &mutableUserStore{user: &domain.User{Email: "user@example.com", Role: "user"}}
	called := 0
	h := UserJWTWithStore(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	doRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := doRequest(); rec.Code != http.StatusOK {
		t.Fatalf("first request (no lock yet): status = %d, want %d", rec.Code, http.StatusOK)
	}

	// An admin issues a temporary password: the account row changes, the
	// token the browser is holding does not.
	now := time.Now()
	store.user = &domain.User{Email: "user@example.com", Role: "user", PasswordChangeRequiredAt: &now}

	rec := doRequest()
	if rec.Code != http.StatusConflict {
		t.Fatalf("second request (same token, lock now set): status = %d, want %d", rec.Code, http.StatusConflict)
	}
	if called != 1 {
		t.Fatalf("handler ran %d times, want exactly 1 (only the first, unlocked request)", called)
	}
}
