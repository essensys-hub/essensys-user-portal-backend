package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
)

type scenariosSyncEnabledBody struct {
	Enabled bool `json:"enabled"`
}

// PatchScenariosSyncProfile toggles the default Scénarios sync profile (LAN Settings).
func (h *Handler) PatchScenariosSyncProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	var body scenariosSyncEnabledBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	profile, err := h.store.GetSyncProfileByNameForGateway(r.Context(), gatewayID, "Scénarios")
	if err != nil || profile == nil {
		http.Error(w, "Scénarios profile not found", http.StatusNotFound)
		return
	}
	updated, err := h.store.SetSyncProfileEnabled(r.Context(), profile.ID, gatewayID, body.Enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": updated})
}
