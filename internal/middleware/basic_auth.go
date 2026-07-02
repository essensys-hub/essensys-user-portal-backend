package middleware

import (
	"context"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

const LegacyClientIDKey contextKey = "clientID"
const LegacyHashedPkeyKey contextKey = "hashedPkey"

type LegacyMachineStore interface {
	GetMachineByHashedPkey(hashedPkey string) (*domain.LegacyMachine, error)
	RegisterUnknownMachine(hashedPkey string) (*domain.LegacyMachine, error)
	UpdateMachineStatus(hashedPkey, ip, rawAuth, rawDecoded string)
}

func BasicAuth(store LegacyMachineStore, strict bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
				if !strict {
					ctx := context.WithValue(r.Context(), LegacyClientIDKey, "anonymous")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				basicUnauthorized(w)
				return
			}

			encodedCredentials := authHeader[6:]
			decodedBytes, err := base64.StdEncoding.DecodeString(encodedCredentials)
			if err != nil {
				if !strict {
					ctx := context.WithValue(r.Context(), LegacyClientIDKey, "anonymous")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				basicUnauthorized(w)
				return
			}

			credentials := string(decodedBytes)
			parts := strings.SplitN(credentials, ":", 2)
			if len(parts) != 2 {
				if !strict {
					ctx := context.WithValue(r.Context(), LegacyClientIDKey, "anonymous")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				basicUnauthorized(w)
				return
			}

			username := parts[0]
			password := parts[1]
			hashedPkey := username + password

			machine, err := store.GetMachineByHashedPkey(hashedPkey)
			if err != nil || machine == nil {
				log.Printf("BasicAuth: unknown machine hash %s..., registering", safePrefix(hashedPkey, 10))
				machine, err = store.RegisterUnknownMachine(hashedPkey)
				if err != nil {
					if !strict {
						ctx := context.WithValue(r.Context(), LegacyClientIDKey, "anonymous")
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					basicUnauthorized(w)
					return
				}
			}

			store.UpdateMachineStatus(hashedPkey, clientIPFromRequest(r), encodedCredentials, credentials)

			if !machine.IsActive {
				if !strict {
					// Inactive armoires still poll serverinfos/mystatus for inventory (MAC, telemetry).
					log.Printf("BasicAuth (Lax): machine %s inactive, telemetry only", machine.NoSerie)
					ctx := context.WithValue(r.Context(), LegacyClientIDKey, machine.NoSerie)
					ctx = context.WithValue(ctx, LegacyHashedPkeyKey, hashedPkey)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				http.Error(w, "Machine inactive", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), LegacyClientIDKey, machine.NoSerie)
			ctx = context.WithValue(ctx, LegacyHashedPkeyKey, hashedPkey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func basicUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Basic")
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func clientIPFromRequest(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return strings.TrimSpace(real)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
