package middleware

import (
	"context"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

type ActiveUserStore interface {
	GetUserByEmail(email string) (*domain.User, error)
}

// resolveActiveUser loads the account and applies the checks that hold
// regardless of the caller's purpose: unknown account, forbidden account.
// It intentionally does not test PasswordChangeRequired — that check has
// exactly one exemption (the change-password route itself), so it lives only
// in enforceActiveUser, not here where a future caller could reach for it by
// accident.
func resolveActiveUser(w http.ResponseWriter, users ActiveUserStore, email string) (*domain.User, bool) {
	if users == nil || email == "" {
		return nil, true
	}
	user, err := users.GetUserByEmail(email)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil, false
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if domain.IsUserForbidden(user) {
		domain.WriteAccountForbidden(w)
		return nil, false
	}
	return user, true
}

// enforceActiveUser is resolveActiveUser plus the password-change-required
// lock: an account carrying an admin-issued temporary password is refused on
// every authenticated route that goes through it, which is every route
// except the one mounted with enforceActiveUserAllowPasswordChange. Because
// the account is reloaded from the database on every request, this takes
// effect immediately — no re-issued token, no wait for expiry.
func enforceActiveUser(w http.ResponseWriter, users ActiveUserStore, email string) (*domain.User, bool) {
	user, ok := resolveActiveUser(w, users, email)
	if !ok {
		return nil, false
	}
	if domain.PasswordChangeRequired(user) {
		domain.WritePasswordChangeRequired(w)
		return nil, false
	}
	return user, true
}

// enforceActiveUserAllowPasswordChange is the one deliberate exemption from
// the lock above, reserved for POST /auth/password/change. A named function
// rather than a boolean threaded through enforceActiveUser, so the exemption
// stays visible at its single point of use instead of becoming an option any
// future caller could reach for.
func enforceActiveUserAllowPasswordChange(w http.ResponseWriter, users ActiveUserStore, email string) (*domain.User, bool) {
	return resolveActiveUser(w, users, email)
}

func withUserContext(r *http.Request, email, role string) context.Context {
	ctx := context.WithValue(r.Context(), UserEmailKey, email)
	return context.WithValue(ctx, UserRoleKey, role)
}

func UserJWTWithStore(users ActiveUserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return UserJWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email, _ := r.Context().Value(UserEmailKey).(string)
			user, ok := enforceActiveUser(w, users, email)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(withUserContext(r, email, user.Role)))
		}))
	}
}

func AdminJWTWithStore(users ActiveUserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return AdminJWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email, _ := r.Context().Value(UserEmailKey).(string)
			user, ok := enforceActiveUser(w, users, email)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(withUserContext(r, email, user.Role)))
		}))
	}
}

// UserJWTAllowPasswordChange is UserJWTWithStore without the
// password-change-required lock (D3). Mount it on exactly one route:
// POST /auth/password/change — the only place an account that must change
// its password is allowed to act before doing so.
func UserJWTAllowPasswordChange(users ActiveUserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return UserJWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email, _ := r.Context().Value(UserEmailKey).(string)
			user, ok := enforceActiveUserAllowPasswordChange(w, users, email)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(withUserContext(r, email, user.Role)))
		}))
	}
}

func AdminAuthWithStore(users ActiveUserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email, _ := r.Context().Value(UserEmailKey).(string)
			if email == "" {
				next.ServeHTTP(w, r)
				return
			}
			user, ok := enforceActiveUser(w, users, email)
			if !ok {
				return
			}
			next.ServeHTTP(w, r.WithContext(withUserContext(r, email, user.Role)))
		}))
	}
}
