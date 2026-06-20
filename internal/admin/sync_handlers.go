package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/syncprofile"
	"github.com/go-chi/chi/v5"
)

func (h *Handlers) ListSyncProfiles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	if h.portal == nil {
		writeJSON(w, http.StatusOK, []domain.SyncProfile{})
		return
	}
	gatewayID := r.URL.Query().Get("gateway_id")
	list, err := h.portal.ListSyncProfiles(r.Context(), gatewayID)
	if err != nil {
		http.Error(w, "List failed", http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []domain.SyncProfile{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) CreateSyncProfile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	var req domain.UpsertSyncProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.Name == "" || len(req.IndexRanges) == 0 {
		http.Error(w, "name and index_ranges required", http.StatusBadRequest)
		return
	}
	if req.IntervalHours <= 0 {
		req.IntervalHours = 3
	}
	p, err := h.portal.CreateSyncProfile(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handlers) UpdateSyncProfile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req domain.UpsertSyncProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.IntervalHours <= 0 {
		req.IntervalHours = 3
	}
	p, err := h.portal.UpdateSyncProfile(r.Context(), id, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handlers) DeleteSyncProfile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.portal.DeleteSyncProfile(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handlers) RunSyncProfile(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	profile, err := h.portal.GetSyncProfile(r.Context(), id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	gatewayID := profile.GatewayID
	if gw := r.URL.Query().Get("gateway_id"); gw != "" {
		gatewayID = gw
	}
	if gatewayID == "" {
		http.Error(w, "gateway_id required (set on profile or ?gateway_id=)", http.StatusBadRequest)
		return
	}
	expected := syncprofile.ExpectedIndexCount(profile.IndexRanges)
	chunks := syncprofile.PullChunkCount(profile.IndexRanges)
	msg := "Sync demandée depuis admin — " + profile.Name
	if profile.PullFromArmoire {
		msg += " · " + strconv.Itoa(expected) + " octets · " + strconv.Itoa(chunks) + " cycle(s) firmware"
	}
	run, err := h.portal.CreateSyncRun(r.Context(), id, gatewayID, expected, msg)
	if err != nil {
		http.Error(w, "Create run failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "started",
		"run":    run,
	})
}

func (h *Handlers) ListSyncProfileRuns(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := h.portal.ListSyncRunsForProfile(r.Context(), id, limit)
	if err != nil {
		http.Error(w, "List failed", http.StatusInternalServerError)
		return
	}
	if runs == nil {
		runs = []domain.SyncRun{}
	}
	writeJSON(w, http.StatusOK, runs)
}

func (h *Handlers) GetSyncRun(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminGlobal(w, r); !ok {
		return
	}
	runID := chi.URLParam(r, "runId")
	run, err := h.portal.GetSyncRun(r.Context(), runID)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, run)
}
