package middleware

import (
	"context"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

type ActiveUserStore interface {
	GetUserByEmail(email string) (*domain.User, error)
}

func enforceActiveUser(w http.ResponseWriter, users ActiveUserStore, email string) (*domain.User, bool) {
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
