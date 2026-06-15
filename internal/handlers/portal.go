package handlers

import (
	"context"
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
	"github.com/newrelic/go-agent/v3/newrelic"
)

type Handler struct {
	store           *data.PortalStore
	exchangeStaleTTL time.Duration
}

func NewHandler(store *data.PortalStore, exchangeStaleTTL time.Duration) *Handler {
	if exchangeStaleTTL <= 0 {
		exchangeStaleTTL = 120 * time.Second
	}
	return &Handler{store: store, exchangeStaleTTL: exchangeStaleTTL}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if txn := newrelic.FromContext(r.Context()); txn != nil {
		txn.Ignore()
	}
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
	approved, _ := h.store.UserHasApprovedLink(r.Context(), user.ID)
	portalAccess := approved &&
		user.LinkedMachineID != nil && user.LinkedGatewayID != nil &&
		domain.IsRemoteEligibleGateway(user.LinkedGatewayID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":              "none",
			"linked_machine_id":   user.LinkedMachineID,
			"linked_gateway_id":   user.LinkedGatewayID,
			"portal_access":       portalAccess,
		})
		return
	}
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
	if rows == nil {
		rows = []domain.LinkRequest{}
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

func (h *Handler) GetExchange(w http.ResponseWriter, r *http.Request) {
	if !h.requirePortalAccess(w, r) {
		return
	}
	keysParam := r.URL.Query().Get("keys")
	requested, err := data.ParseKeyList(keysParam)
	if err != nil {
		http.Error(w, "Missing or invalid keys", http.StatusBadRequest)
		return
	}

	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || user.LinkedMachineID == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	machineID := *user.LinkedMachineID
	clientID := fmt.Sprintf("%d", machineID)
	now := time.Now()

	var cached, tel []domain.ExchangeKV
	var cacheUpdated, telUpdated time.Time
	cacheOK := false
	telOK := false

	if c, updatedAt, err := h.store.GetGatewayExchange(r.Context(), machineID); err == nil {
		cached = c
		cacheUpdated = updatedAt
		cacheOK = true
	}
	if t, updatedAt, err := h.store.GetMachineTelemetry(r.Context(), clientID); err == nil {
		tel = t
		telUpdated = updatedAt
		telOK = true
	}

	values := data.MergeExchangeKeys(cached, tel, requested)

	source := "none"
	stale := true
	var updatedAt time.Time
	switch {
	case cacheOK && telOK:
		source = "gateway_cache+mystatus"
		updatedAt = cacheUpdated
		if telUpdated.After(updatedAt) {
			updatedAt = telUpdated
		}
		stale = now.Sub(cacheUpdated) > h.exchangeStaleTTL
	case cacheOK:
		source = "gateway_cache"
		updatedAt = cacheUpdated
		stale = now.Sub(cacheUpdated) > h.exchangeStaleTTL
	case telOK:
		source = "mystatus"
		updatedAt = telUpdated
		stale = true
	}

	resp := map[string]any{
		"values": values,
		"stale":  stale,
		"source": source,
	}
	if !updatedAt.IsZero() {
		resp["updated_at"] = updatedAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetHistoryLatest(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	action, err := h.store.GetLastCloudActionForUser(r.Context(), user.ID)
	if err != nil || action == nil {
		writeJSON(w, http.StatusOK, map[string]any{"lastAction": nil, "message": "No actions yet"})
		return
	}
	info := fmt.Sprintf("Cloud action (%d params)", len(action.Params))
	if len(action.Params) > 0 {
		info = fmt.Sprintf("k=%d", action.Params[0].K)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"lastAction": map[string]any{
			"guid":       action.GUID,
			"actionType": "CLOUD",
			"actionInfo": info,
			"isDone":     action.Status == domain.ActionStatusDone,
			"timestamp":  action.CreatedAt.UTC().Format(time.RFC3339),
		},
	})
}

func (h *Handler) PostWebActions(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if !h.userPortalAccess(r.Context(), user) {
		http.Error(w, "Portal access not approved", http.StatusForbidden)
		return
	}

	var req struct {
		Alarme     string `json:"alarme"`
		CodeAlarme string `json:"codealarme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}
	if req.Alarme == "" || len(req.CodeAlarme) != 4 {
		http.Error(w, "alarme and 4-digit codealarme required", http.StatusBadRequest)
		return
	}
	cmd := "0"
	if req.Alarme == "on" {
		cmd = "1"
	}
	params := []domain.ExchangeKV{
		{K: 409, V: cmd},
		{K: 410, V: req.CodeAlarme[0:2]},
		{K: 411, V: req.CodeAlarme[2:4]},
		{K: 307, V: "0"},
	}
	guid := newGUID()
	if err := h.store.EnqueueCloudAction(r.Context(), guid, user.ID, user.LinkedMachineID, params); err != nil {
		http.Error(w, "Enqueue failed", http.StatusInternalServerError)
		return
	}
	_ = h.store.AuditLog(r.Context(), email, "portal_alarm", map[string]any{"guid": guid, "alarme": req.Alarme})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Action sent to queue"})
}

func (h *Handler) requirePortalAccess(w http.ResponseWriter, r *http.Request) bool {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || !h.userPortalAccess(r.Context(), user) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func (h *Handler) userPortalAccess(ctx context.Context, user *domain.UserProfile) bool {
	if user == nil || user.LinkedMachineID == nil || !domain.IsRemoteEligibleGateway(user.LinkedGatewayID) {
		return false
	}
	ok, _ := h.store.UserHasApprovedLink(ctx, user.ID)
	return ok
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
