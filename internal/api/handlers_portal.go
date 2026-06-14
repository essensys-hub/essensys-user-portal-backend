package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	store *data.PortalStore
}

func NewHandler(store *data.PortalStore) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type linkRequestBody struct {
	MachineSerial string `json:"machine_serial"`
	Message       string `json:"message"`
}

func (h *Handler) CreateLinkRequest(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if !domain.IsRemoteEligibleGateway(user.LinkedGatewayID) {
		http.Error(w, "Gateway not eligible for remote portal", http.StatusForbidden)
		return
	}

	var body linkRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MachineSerial == "" {
		http.Error(w, "machine_serial required", http.StatusBadRequest)
		return
	}

	lr, err := h.store.CreateLinkRequest(r.Context(), user.ID, body.MachineSerial, body.Message)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, lr)
}

func (h *Handler) LinkRequestStatus(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	lr, err := h.store.GetLatestLinkRequest(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":              "none",
			"linked_machine_id":   user.LinkedMachineID,
			"linked_gateway_id":   user.LinkedGatewayID,
			"portal_access":       false,
		})
		return
	}
	portalAccess := lr.Status == domain.LinkStatusApproved &&
		user.LinkedMachineID != nil && user.LinkedGatewayID != nil &&
		domain.IsRemoteEligibleGateway(user.LinkedGatewayID)
	writeJSON(w, http.StatusOK, map[string]any{
		"link_request":        lr,
		"linked_machine_id":   user.LinkedMachineID,
		"linked_gateway_id":   user.LinkedGatewayID,
		"portal_access":       portalAccess,
	})
}

func (h *Handler) GatewayStatus(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || user.LinkedGatewayID == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"online": false})
		return
	}
	online, _ := h.store.IsGatewayOnline(r.Context(), *user.LinkedGatewayID, 2*time.Minute)
	writeJSON(w, http.StatusOK, map[string]bool{"online": online})
}

type injectBody struct {
	K    int    `json:"k"`
	V    string `json:"v"`
	GUID string `json:"guid,omitempty"`
}

func (h *Handler) Inject(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	approved, _ := h.store.UserHasApprovedLink(r.Context(), user.ID)
	if !approved || user.LinkedMachineID == nil {
		http.Error(w, "Portal access not approved", http.StatusForbidden)
		return
	}
	if !domain.IsRemoteEligibleGateway(user.LinkedGatewayID) {
		http.Error(w, "Gateway not eligible for remote portal", http.StatusForbidden)
		return
	}

	var body injectBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	guid := body.GUID
	if guid == "" {
		guid = newGUID()
	}

	params := domain.ExpandLegacyScenarioBlock([]domain.ExchangeKV{{K: body.K, V: body.V}})
	if err := h.store.EnqueueCloudAction(r.Context(), guid, user.ID, user.LinkedMachineID, params); err != nil {
		http.Error(w, "Enqueue failed", http.StatusInternalServerError)
		return
	}
	_ = h.store.AuditLog(r.Context(), email, "portal_inject", map[string]any{"guid": guid, "k": body.K, "v": body.V})
	writeJSON(w, http.StatusOK, map[string]any{"guid": guid, "params": params})
}

func (h *Handler) ListPendingLinkRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListLinkRequestsByStatus(r.Context(), domain.LinkStatusPending)
	if err != nil {
		http.Error(w, "List failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *Handler) ReviewLinkRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	reviewer := r.Context().Value(middleware.UserEmailKey).(string)
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	if body.Status != domain.LinkStatusApproved && body.Status != domain.LinkStatusRejected {
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}
	var reqID int
	if _, err := fmt.Sscanf(id, "%d", &reqID); err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateLinkRequestStatus(r.Context(), reqID, body.Status, reviewer); err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": body.Status})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func newGUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" + hex.EncodeToString(b[10:])
}
