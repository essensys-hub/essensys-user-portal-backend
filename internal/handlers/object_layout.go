package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/testmode"
)

// Disposition des objets (feature affichage-objets-2026-11-006, #59) : activation
// et ordre d'affichage des lumières et des volets. Un document par armoire et
// par genre ; le cloud est l'arbitre. Les routes des deux genres sont jumelles
// et partagent le même code, seul le genre change.

const (
	layoutWriteMaxBytes = 16 * 1024
	layoutSyncMaxBytes  = 64 * 1024
	layoutAuthorMax     = 200
)

// layoutView est le document rendu aux clients : jamais d'updated_by (le cloud y
// stocke l'adresse e-mail de l'administrateur).
type layoutView struct {
	Order     []string `json:"order"`
	Disabled  []string `json:"disabled"`
	Version   int      `json:"version"`
	UpdatedAt *string  `json:"updated_at"`
	Status    string   `json:"status,omitempty"`
	DryRun    bool     `json:"dry_run,omitempty"`
}

// layoutSyncView est l'état fusionné rendu à la gateway, avec updated_by.
type layoutSyncView struct {
	Order     []string `json:"order"`
	Disabled  []string `json:"disabled"`
	Version   int      `json:"version"`
	UpdatedAt *string  `json:"updated_at"`
	UpdatedBy string   `json:"updated_by"`
}

func layoutTime(l domain.ObjectLayout) *string {
	if l.UpdatedAt.IsZero() {
		return nil
	}
	ts := l.UpdatedAt.UTC().Format(lightNameTimeLayout)
	return &ts
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func layoutViewOf(l domain.ObjectLayout) layoutView {
	return layoutView{Order: nonNil(l.Order), Disabled: nonNil(l.Disabled), Version: l.Version, UpdatedAt: layoutTime(l)}
}

func layoutSyncViewOf(l domain.ObjectLayout) layoutSyncView {
	return layoutSyncView{Order: nonNil(l.Order), Disabled: nonNil(l.Disabled), Version: l.Version, UpdatedAt: layoutTime(l), UpdatedBy: l.UpdatedBy}
}

func layoutAudit(l domain.ObjectLayout) map[string]any {
	return map[string]any{"order": nonNil(l.Order), "disabled": nonNil(l.Disabled), "version": l.Version}
}

func layoutErrorCode(err error) string {
	switch err {
	case domain.ErrUnknownLight:
		return "unknown_light"
	case domain.ErrUnknownShutter:
		return "unknown_shutter"
	}
	return "invalid_layout"
}

func (h *Handler) getLayout(kind domain.LayoutKind, w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || user.LinkedMachineID == nil || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	l, _, err := h.store.GetObjectLayout(r.Context(), *user.LinkedMachineID, kind)
	if err != nil {
		log.Printf("[portal] object layout %s read: %v", kind, err)
		writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, layoutViewOf(l))
}

// stringList lit une valeur JSON qui doit être une liste de chaînes (null refusé).
func stringList(raw json.RawMessage) ([]string, bool) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, false
	}
	var out []string
	if json.Unmarshal(raw, &out) != nil {
		return nil, false
	}
	return nonNil(out), true
}

// parseLayoutWrite lit {"order":[…],"disabled":[…]} ou {"reset":true}. Les deux
// clés sont obligatoires hors reset. Le serveur valide, il ne complète pas.
func parseLayoutWrite(raw []byte) (order, disabled []string, ok bool) {
	if len(raw) > layoutWriteMaxBytes {
		return nil, nil, false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return nil, nil, false
	}
	if rs, has := m["reset"]; has {
		var reset bool
		if json.Unmarshal(rs, &reset) == nil && reset {
			return []string{}, []string{}, true
		}
	}
	order, ok1 := stringList(m["order"])
	disabled, ok2 := stringList(m["disabled"])
	if !ok1 || !ok2 {
		return nil, nil, false
	}
	return order, disabled, true
}

func (h *Handler) putLayout(kind domain.LayoutKind, w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || !domain.CanWriteLayout(user.Role, user.LinkedMachineID) || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, layoutWriteMaxBytes+1))
	order, disabled, ok := parseLayoutWrite(raw)
	if !ok {
		writeLTError(w, http.StatusBadRequest, "invalid_layout")
		return
	}
	if err := domain.ValidateLayout(kind, order, disabled); err != nil {
		writeLTError(w, http.StatusBadRequest, layoutErrorCode(err))
		return
	}
	machineID := *user.LinkedMachineID
	before, _, err := h.store.GetObjectLayout(r.Context(), machineID, kind)
	if err != nil {
		log.Printf("[portal] object layout %s read: %v", kind, err)
		writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
		return
	}
	if testmode.IsDryRun(r) {
		would := before
		would.Order, would.Disabled = order, disabled
		v := layoutViewOf(would)
		v.Status, v.DryRun = "dry_run", true
		writeJSON(w, http.StatusOK, v)
		return
	}
	row, err := h.store.SetObjectLayout(r.Context(), machineID, kind, order, disabled, email)
	if err != nil {
		log.Printf("[portal] object layout %s write: %v", kind, err)
		writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
		return
	}
	_ = h.store.AuditLog(r.Context(), email, "object_layout", map[string]any{
		"kind": string(kind), "machine_id": machineID, "version": row.Version,
		"old": layoutAudit(before), "new": layoutAudit(row),
	})
	v := layoutViewOf(row)
	v.Status = "saved"
	writeJSON(w, http.StatusOK, v)
}

// syncDocument lit le document envoyé par la gateway. Un document de version 0,
// illisible, avec un identifiant inconnu ou en double est ignoré (ok à false)
// sans bloquer la réponse.
func syncDocument(kind domain.LayoutKind, m map[string]json.RawMessage, now time.Time) (domain.ObjectLayout, bool) {
	var l domain.ObjectLayout
	var ok bool
	if json.Unmarshal(m["version"], &l.Version) != nil || l.Version <= 0 {
		return l, false
	}
	if l.Order, ok = stringList(m["order"]); !ok {
		return l, false
	}
	if l.Disabled, ok = stringList(m["disabled"]); !ok {
		return l, false
	}
	if domain.ValidateLayout(kind, l.Order, l.Disabled) != nil {
		return l, false
	}
	var at string
	if json.Unmarshal(m["updated_at"], &at) != nil {
		return l, false
	}
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(at))
	if err != nil {
		return l, false
	}
	l.UpdatedAt = domain.ClampLightNameTime(t.UTC().Truncate(time.Millisecond), now)
	var by string
	_ = json.Unmarshal(m["updated_by"], &by)
	by = strings.TrimSpace(by)
	if by == "" || !utf8.ValidString(by) {
		by = "gateway"
	}
	if utf8.RuneCountInString(by) > layoutAuthorMax {
		by = string([]rune(by)[:layoutAuthorMax])
	}
	l.UpdatedBy = by
	return l, true
}

// syncLayout fusionne le document de la gateway avec celui du cloud, document
// entier : le plus grand (updated_at, version, updated_by) gagne, une égalité
// parfaite garde la valeur du cloud. Répond l'état fusionné complet.
func (h *Handler) syncLayout(kind domain.LayoutKind, w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	machineID, err := h.store.MachineIDForGateway(r.Context(), gatewayID)
	if err != nil {
		http.Error(w, "Unknown gateway", http.StatusUnauthorized)
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, layoutSyncMaxBytes+1))
	var m map[string]json.RawMessage
	if len(raw) > layoutSyncMaxBytes || json.Unmarshal(raw, &m) != nil || m == nil {
		writeLTError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	cur, _, err := h.store.GetObjectLayout(r.Context(), machineID, kind)
	if err != nil {
		log.Printf("[gateway] object layout %s read: %v", kind, err)
		writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
		return
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	if gw, ok := syncDocument(kind, m, now); ok && domain.LayoutNewer(gw, cur) {
		if err := h.store.PutObjectLayoutExact(r.Context(), machineID, kind, gw); err != nil {
			log.Printf("[gateway] object layout %s merge: %v", kind, err)
			writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
			return
		}
		_ = h.store.AuditLog(r.Context(), "gateway:"+gatewayID, "object_layout", map[string]any{
			"kind": string(kind), "machine_id": machineID, "version": gw.Version,
			"old": layoutAudit(cur), "new": layoutAudit(gw), "source": "gateway_sync",
		})
		cur = gw
	}
	writeJSON(w, http.StatusOK, layoutSyncViewOf(cur))
}

func (h *Handler) GetLightsLayout(w http.ResponseWriter, r *http.Request) {
	h.getLayout(domain.LayoutLights, w, r)
}
func (h *Handler) GetShuttersLayout(w http.ResponseWriter, r *http.Request) {
	h.getLayout(domain.LayoutShutters, w, r)
}
func (h *Handler) PutLightsLayout(w http.ResponseWriter, r *http.Request) {
	h.putLayout(domain.LayoutLights, w, r)
}
func (h *Handler) PutShuttersLayout(w http.ResponseWriter, r *http.Request) {
	h.putLayout(domain.LayoutShutters, w, r)
}
func (h *Handler) SyncLightsLayout(w http.ResponseWriter, r *http.Request) {
	h.syncLayout(domain.LayoutLights, w, r)
}
func (h *Handler) SyncShuttersLayout(w http.ResponseWriter, r *http.Request) {
	h.syncLayout(domain.LayoutShutters, w, r)
}
