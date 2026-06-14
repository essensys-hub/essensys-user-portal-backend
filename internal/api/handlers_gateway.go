package api

import (
	"encoding/json"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) PendingActions(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	actions, err := h.store.FetchPendingActionsForGateway(r.Context(), gatewayID, 20)
	if err != nil {
		http.Error(w, "Fetch failed", http.StatusInternalServerError)
		return
	}
	if len(actions) == 0 {
		writeJSON(w, http.StatusOK, []domain.CloudAction{})
		return
	}
	writeJSON(w, http.StatusOK, actions)
}

func (h *Handler) ActionDone(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		http.Error(w, "guid required", http.StatusBadRequest)
		return
	}
	if err := h.store.MarkActionDone(r.Context(), guid); err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "done"})
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	if err := h.store.TouchGatewayHeartbeat(r.Context(), gatewayID); err != nil {
		http.Error(w, "Heartbeat failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerGatewayBody struct {
	GatewayID string `json:"gateway_id"`
	Token     string `json:"token"`
	MachineID *int   `json:"machine_id,omitempty"`
}

func (h *Handler) RegisterGateway(w http.ResponseWriter, r *http.Request) {
	var body registerGatewayBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.GatewayID == "" || body.Token == "" {
		http.Error(w, "gateway_id and token required", http.StatusBadRequest)
		return
	}
	if err := h.store.RegisterGatewaySession(r.Context(), body.GatewayID, body.Token, body.MachineID); err != nil {
		http.Error(w, "Register failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"gateway_id": body.GatewayID})
}
