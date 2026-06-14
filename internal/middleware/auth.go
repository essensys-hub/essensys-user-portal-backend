package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

type contextKey string

const UserEmailKey contextKey = "user_email"
const UserRoleKey contextKey = "user_role"
const GatewayIDKey contextKey = "gateway_id"

func jwtKey() []byte {
	key := os.Getenv("JWT_SECRET")
	if key == "" {
		key = "default-insecure-jwt-secret-change-me"
	}
	return []byte(key)
}

func UserJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tokenStr == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtKey(), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		email, _ := claims["sub"].(string)
		role, _ := claims["role"].(string)
		if email == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), UserEmailKey, email)
		ctx = context.WithValue(ctx, UserRoleKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AdminJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tokenStr == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return jwtKey(), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims, _ := token.Claims.(jwt.MapClaims)
		role, _ := claims["role"].(string)
		if role != "admin_global" && role != "support" && role != "admin_local" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		email, _ := claims["sub"].(string)
		ctx := context.WithValue(r.Context(), UserEmailKey, email)
		ctx = context.WithValue(ctx, UserRoleKey, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GatewayAuth(store interface {
	ValidateGatewayToken(ctx context.Context, gatewayID, token string) bool
}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gatewayID := r.Header.Get("X-Gateway-ID")
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if gatewayID == "" || token == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if !store.ValidateGatewayToken(r.Context(), gatewayID, token) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), GatewayIDKey, gatewayID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
