package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain/scenario"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/testmode"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListScenarios(w http.ResponseWriter, r *http.Request) {
	if !h.requirePortalAccess(w, r) {
		return
	}
	user, err := h.portalUser(r)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	cache, _, err := h.store.GetGatewayExchange(r.Context(), *user.LinkedMachineID)
	if err != nil {
		cache = nil
	}
	byK := kvMap(cache)
	last := parseOptionalInt(byK[scenario.IndexLastLaunched])

	slots := make([]scenarioSlotSummary, 0, scenario.SlotCount)
	for slot := 1; slot <= scenario.SlotCount; slot++ {
		base, _ := scenario.SlotBaseIndex(slot)
		sum := scenarioSlotSummary{
			SlotNumber: slot,
			Label:      scenario.DefaultSlotLabels[slot],
			BaseIndex:  base,
			EndIndex:   base + scenario.ParamCount - 1,
			Editable:   slot >= 2,
		}
		if last != nil && *last == slot {
			sum.LastLaunched = last
		}
		slots = append(slots, sum)
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

func (h *Handler) GetScenario(w http.ResponseWriter, r *http.Request) {
	if !h.requirePortalAccess(w, r) {
		return
	}
	slot, err := scenarioSlotParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := h.portalUser(r)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	detail, err := h.scenarioDetail(r.Context(), *user.LinkedMachineID, slot)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) PutScenario(w http.ResponseWriter, r *http.Request) {
	if !h.requirePortalAccess(w, r) {
		return
	}
	slot, err := scenarioSlotParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := h.portalUser(r)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var body struct {
		Params map[int]string `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Params) == 0 {
		http.Error(w, "params required", http.StatusBadRequest)
		return
	}

	chunks, err := scenario.WriteDefinitionChunks(slot, body.Params)
	if err != nil {
		if err == scenario.ErrSlot1ServerReserved {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if testmode.IsDryRun(r) {
			testmode.WriteFailed(w, err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if testmode.IsDryRun(r) {
		testmode.WriteOK(w, flattenChunks(chunks), nil, "")
		return
	}

	guids := make([]string, 0, len(chunks))
	email := r.Context().Value(middleware.UserEmailKey).(string)
	for _, chunk := range chunks {
		guid := newGUID()
		if err := h.store.EnqueueCloudAction(r.Context(), guid, user.ID, user.LinkedMachineID, chunk); err != nil {
			http.Error(w, "Enqueue failed", http.StatusInternalServerError)
			return
		}
		guids = append(guids, guid)
	}
	flat := flattenChunks(chunks)
	_ = h.store.UpsertGatewayExchange(r.Context(), *user.LinkedMachineID, flat)
	_ = h.store.AuditLog(r.Context(), email, "portal_scenario_put", map[string]any{"slot": slot, "guids": guids})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "guids": guids})
}

func (h *Handler) LaunchScenario(w http.ResponseWriter, r *http.Request) {
	if !h.requirePortalAccess(w, r) {
		return
	}
	slot, err := scenarioSlotParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := h.portalUser(r)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	params, err := scenario.LaunchParams(slot)
	if err != nil {
		if testmode.IsDryRun(r) {
			testmode.WriteFailed(w, err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if testmode.IsDryRun(r) {
		testmode.WriteOK(w, params, nil, "")
		return
	}
	guid := newGUID()
	email := r.Context().Value(middleware.UserEmailKey).(string)
	if err := h.store.EnqueueCloudAction(r.Context(), guid, user.ID, user.LinkedMachineID, params); err != nil {
		http.Error(w, "Enqueue failed", http.StatusInternalServerError)
		return
	}
	_ = h.store.AuditLog(r.Context(), email, "portal_scenario_launch", map[string]any{"slot": slot, "guid": guid})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "guid": guid, "slot": slot})
}

func (h *Handler) RestoreScenario(w http.ResponseWriter, r *http.Request) {
	if !h.requirePortalAccess(w, r) {
		return
	}
	slot, err := scenarioSlotParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := h.portalUser(r)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	params, err := scenario.RestorePresetParams(slot)
	if err != nil {
		if testmode.IsDryRun(r) {
			testmode.WriteFailed(w, err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if testmode.IsDryRun(r) {
		testmode.WriteOK(w, params, nil, "")
		return
	}
	guid := newGUID()
	email := r.Context().Value(middleware.UserEmailKey).(string)
	if err := h.store.EnqueueCloudAction(r.Context(), guid, user.ID, user.LinkedMachineID, params); err != nil {
		http.Error(w, "Enqueue failed", http.StatusInternalServerError)
		return
	}
	_ = h.store.AuditLog(r.Context(), email, "portal_scenario_restore", map[string]any{"slot": slot, "guid": guid})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "guid": guid, "slot": slot})
}

func (h *Handler) GetScenarioBitmasks(w http.ResponseWriter, r *http.Request) {
	if !h.requirePortalAccess(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": scenario.BitmaskMeta()})
}

type scenarioSlotSummary struct {
	SlotNumber   int    `json:"slot_number"`
	Label        string `json:"label"`
	BaseIndex    int    `json:"base_index"`
	EndIndex     int    `json:"end_index"`
	Editable     bool   `json:"editable"`
	LastLaunched *int   `json:"last_launched,omitempty"`
}

type scenarioSlotDetail struct {
	scenarioSlotSummary
	Params []domain.ExchangeKV `json:"params"`
}

func (h *Handler) scenarioDetail(ctx context.Context, machineID, slot int) (*scenarioSlotDetail, error) {
	if slot < 1 || slot > scenario.SlotCount {
		return nil, fmt.Errorf("invalid slot")
	}
	start, end, err := scenario.SlotRange(slot)
	if err != nil {
		return nil, err
	}
	cache, _, err := h.store.GetGatewayExchange(ctx, machineID)
	if err != nil {
		cache = nil
	}
	byK := kvMap(cache)
	params := make([]domain.ExchangeKV, 0, scenario.ParamCount)
	for i := start; i <= end; i++ {
		if v, ok := byK[i]; ok {
			params = append(params, domain.ExchangeKV{K: i, V: v})
		}
	}
	sort.Slice(params, func(i, j int) bool { return params[i].K < params[j].K })

	base, _ := scenario.SlotBaseIndex(slot)
	last := parseOptionalInt(byK[scenario.IndexLastLaunched])
	sum := scenarioSlotSummary{
		SlotNumber: slot,
		Label:      scenario.DefaultSlotLabels[slot],
		BaseIndex:  base,
		EndIndex:   base + scenario.ParamCount - 1,
		Editable:   slot >= 2,
	}
	if last != nil && *last == slot {
		sum.LastLaunched = last
	}
	return &scenarioSlotDetail{scenarioSlotSummary: sum, Params: params}, nil
}

func (h *Handler) portalUser(r *http.Request) (*domain.UserProfile, error) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	return h.store.GetUserByEmail(r.Context(), email)
}

func scenarioSlotParam(r *http.Request) (int, error) {
	slot, err := strconv.Atoi(chi.URLParam(r, "slot"))
	if err != nil || slot < 1 || slot > scenario.SlotCount {
		return 0, fmt.Errorf("invalid slot")
	}
	return slot, nil
}

func kvMap(kvs []domain.ExchangeKV) map[int]string {
	m := make(map[int]string, len(kvs))
	for _, kv := range kvs {
		m[kv.K] = kv.V
	}
	return m
}

func parseOptionalInt(v string) *int {
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

func flattenChunks(chunks [][]domain.ExchangeKV) []domain.ExchangeKV {
	out := make([]domain.ExchangeKV, 0)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return out
}
