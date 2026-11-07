package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/testmode"
	"github.com/go-chi/chi/v5"
)

// Light auto-off timers (feature lumieres-temporisees-2026-11-003, #53).

type lightTimerView struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Key     int    `json:"key"`
	Minutes *int   `json:"minutes"`
}

func writeLTError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// GetLightTimers answers the six delays from the gateway cache; a key never
// read is null, never 0.
func (h *Handler) GetLightTimers(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || user.LinkedMachineID == nil || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	cache, _, cerr := h.store.GetGatewayExchange(r.Context(), *user.LinkedMachineID)
	if cerr != nil {
		cache = nil
	}
	out := make([]lightTimerView, 0, 6)
	for _, lt := range domain.LightTimers() {
		out = append(out, lightTimerView{ID: lt.ID, Label: lt.Label, Key: lt.Key, Minutes: domain.LightTimerMinutesFromCache(cache, lt.Key)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"timers": out})
}

// PutLightTimer queues one delay for the caller's linked cabinet. Only the
// cabinet administrator may write; the role is read fresh from the database.
func (h *Handler) PutLightTimer(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || !domain.CanWriteLightTimers(user.Role, user.LinkedMachineID) || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	lt, ok := domain.LightTimerByID(chi.URLParam(r, "id"))
	if !ok {
		writeLTError(w, http.StatusBadRequest, "unknown_light")
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	var body struct {
		Minutes json.RawMessage `json:"minutes"`
	}
	if json.Unmarshal(raw, &body) != nil {
		writeLTError(w, http.StatusBadRequest, "invalid_minutes")
		return
	}
	minutes, err := domain.ParseLightTimerMinutes(body.Minutes)
	if err != nil {
		writeLTError(w, http.StatusBadRequest, "invalid_minutes")
		return
	}
	params := []domain.ExchangeKV{{K: lt.Key, V: strconv.Itoa(minutes)}}
	resp := map[string]any{"id": lt.ID, "minutes": minutes, "status": "queued"}
	if testmode.IsDryRun(r) {
		resp["status"] = "dry_run"
		resp["dry_run"] = true
		resp["validated_params"] = params
		writeJSON(w, http.StatusOK, resp)
		return
	}
	guid := newGUID()
	if err := h.store.EnqueueCloudAction(r.Context(), guid, user.ID, user.LinkedMachineID, params); err != nil {
		log.Printf("[portal] light timer enqueue: %v", err)
		writeLTError(w, http.StatusInternalServerError, "enqueue_failed")
		return
	}
	if err := h.store.UpsertGatewayExchange(r.Context(), *user.LinkedMachineID, params); err != nil {
		log.Printf("[portal] light timer cache update: %v", err)
	}
	_ = h.store.AuditLog(r.Context(), email, "portal_light_timer", map[string]any{"guid": guid, "id": lt.ID, "key": lt.Key, "minutes": minutes})
	writeJSON(w, http.StatusOK, resp)
}
