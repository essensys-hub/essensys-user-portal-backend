package middleware

import (
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func GenerateJWT(email, role string, expirationTime time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":  email,
		"role": role,
		"exp":  expirationTime.Unix(),
		"iss":  "essensys-backend",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey())
}
