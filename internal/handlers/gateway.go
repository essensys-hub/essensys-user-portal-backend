package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
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

type pushExchangeBody struct {
	Keys []domain.ExchangeKV `json:"keys"`
}

func (h *Handler) PushExchange(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	machineID, err := h.store.MachineIDForGateway(r.Context(), gatewayID)
	if err != nil {
		http.Error(w, "Unknown gateway", http.StatusUnauthorized)
		return
	}

	var body pushExchangeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Keys) == 0 {
		http.Error(w, "keys required", http.StatusBadRequest)
		return
	}

	if err := h.store.UpsertGatewayExchange(r.Context(), machineID, body.Keys); err != nil {
		http.Error(w, "Store failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type registerGatewayBody struct {
	GatewayID string `json:"gateway_id,omitempty"`
	Token     string `json:"token"`
	MachineID int    `json:"machine_id"`
	Eth0MAC   string `json:"eth0_mac"`
	Eth1MAC   string `json:"eth1_mac"`
}

func (h *Handler) RegisterGateway(w http.ResponseWriter, r *http.Request) {
	var body registerGatewayBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		http.Error(w, "token, machine_id, eth0_mac, eth1_mac required", http.StatusBadRequest)
		return
	}
	if body.MachineID <= 0 || body.Eth0MAC == "" || body.Eth1MAC == "" {
		http.Error(w, "token, machine_id, eth0_mac, eth1_mac required", http.StatusBadRequest)
		return
	}
	reg := data.GatewayRegistration{
		GatewayID: body.GatewayID,
		Token:     body.Token,
		MachineID: body.MachineID,
		Eth0MAC:   body.Eth0MAC,
		Eth1MAC:   body.Eth1MAC,
	}
	if err := h.store.RegisterGatewaySession(r.Context(), reg); err != nil {
		http.Error(w, "Register failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	gatewayID := body.GatewayID
	if gatewayID == "" {
		gatewayID, _ = data.GatewayIDFromEth0MAC(body.Eth0MAC)
	}
	eth0, _ := data.NormalizeMAC(body.Eth0MAC)
	eth1, _ := data.NormalizeMAC(body.Eth1MAC)
	writeJSON(w, http.StatusCreated, map[string]any{
		"gateway_id": gatewayID,
		"machine_id": body.MachineID,
		"eth0_mac":   eth0,
		"eth1_mac":   eth1,
	})
}

func (h *Handler) ListGatewaySessions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListGatewaySessions(r.Context())
	if err != nil {
		http.Error(w, "List failed", http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []domain.GatewaySession{}
	}
	writeJSON(w, http.StatusOK, rows)
}
