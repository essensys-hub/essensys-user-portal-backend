package testmode

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

const (
	Header      = "X-Essensys-Test-Mode"
	QueryParam  = "test_mode"
	ValueDryRun = "dry-run"
	StatusOK    = "test_ok"
	StatusFail  = "test_failed"
)

func IsDryRun(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get(Header)), ValueDryRun) {
		return true
	}
	return r.URL.Query().Get(QueryParam) == "dry_run"
}

func WriteOK(w http.ResponseWriter, validated []domain.ExchangeKV, snapshot []domain.ExchangeKV, message string) {
	if message == "" {
		message = "Validation OK — non envoyé à l'armoire"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":            StatusOK,
		"dry_run":           true,
		"validated_params":  validated,
		"exchange_snapshot": snapshot,
		"message":           message,
	})
}

func WriteFailed(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  StatusFail,
		"dry_run": true,
		"message": message,
	})
}
