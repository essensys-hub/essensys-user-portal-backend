package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/syncprofile"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) SyncConfig(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	profiles, err := h.store.ListSyncProfilesForGateway(r.Context(), gatewayID)
	if err != nil {
		http.Error(w, "List profiles failed", http.StatusInternalServerError)
		return
	}
	if profiles == nil {
		profiles = []domain.SyncProfile{}
	}
	pending, err := h.store.ListPendingSyncRuns(r.Context(), gatewayID)
	if err != nil {
		http.Error(w, "List runs failed", http.StatusInternalServerError)
		return
	}
	if pending == nil {
		pending = []domain.SyncRun{}
	}
	writeJSON(w, http.StatusOK, domain.SyncConfigResponse{
		Profiles:    profiles,
		PendingRuns: pending,
	})
}

func (h *Handler) SyncRunStart(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	runID := chi.URLParam(r, "id")
	if err := h.store.StartSyncRun(r.Context(), runID, gatewayID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (h *Handler) SyncRunProgress(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	runID := chi.URLParam(r, "id")
	var req domain.SyncRunProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := h.store.AppendSyncRunProgress(r.Context(), runID, gatewayID, req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	run, _ := h.store.GetSyncRun(r.Context(), runID)
	writeJSON(w, http.StatusOK, run)
}

type gatewayCreateSyncRunRequest struct {
	ProfileID string `json:"profile_id"`
	Source    string `json:"source"`
}

// SyncRunCreate lets the gateway enqueue a scheduled sync run (source=scheduled).
func (h *Handler) SyncRunCreate(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	var req gatewayCreateSyncRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.ProfileID == "" {
		http.Error(w, "profile_id required", http.StatusBadRequest)
		return
	}
	profile, err := h.store.GetSyncProfile(r.Context(), req.ProfileID)
	if err != nil || profile == nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}
	if profile.GatewayID != "" && profile.GatewayID != gatewayID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	expected := syncprofile.ExpectedIndexCount(profile.IndexRanges)
	msg := "Sync planifiée — " + profile.Name
	if req.Source != "" {
		msg += " (" + req.Source + ")"
	}
	run, err := h.store.CreateSyncRun(r.Context(), req.ProfileID, gatewayID, expected, msg)
	if err != nil {
		http.Error(w, "Create run failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"run": run})
}
