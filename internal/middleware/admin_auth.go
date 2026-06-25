package middleware

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

// AdminAuth accepts legacy ADMIN_TOKEN or JWT with admin_global/admin_local/support role.
func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		// ADMIN_TOKEN is validated at startup (config.Validate); never accept a
		// missing/empty expected token, and compare in constant time.
		expectedToken := os.Getenv("ADMIN_TOKEN")
		if expectedToken != "" && subtle.ConstantTimeCompare([]byte(tokenStr), []byte(expectedToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		if tokenStr != "" {
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return jwtKey(), nil
			})
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					role, _ := claims["role"].(string)
					if role == "admin_global" || role == "admin_local" || role == "support" {
						if sub, ok := claims["sub"].(string); ok {
							ctx := context.WithValue(r.Context(), UserEmailKey, sub)
							ctx = context.WithValue(ctx, UserRoleKey, role)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
						next.ServeHTTP(w, r)
						return
					}
					log.Printf("Forbidden: user role '%s' attempted admin access", role)
					http.Error(w, "Forbidden: Insufficient Permissions", http.StatusForbidden)
					return
				}
			}
		}

		http.Error(w, "Unauthorized Admin Access", http.StatusUnauthorized)
	})
}
