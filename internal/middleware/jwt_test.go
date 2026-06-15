package middleware

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestGenerateJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key")
	tokenStr, err := GenerateJWT("user@example.com", "user", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return jwtKey(), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("invalid token: %v", err)
	}
	claims := token.Claims.(jwt.MapClaims)
	if claims["sub"] != "user@example.com" || claims["role"] != "user" {
		t.Fatalf("unexpected claims: %v", claims)
	}
}
