package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/testmode"
	"github.com/go-chi/chi/v5"
)

// Custom light names (feature noms-lumieres-2026-11-004, #55). The cloud is
// the arbiter: the cloud portal writes here, the gateway syncs its own copy.

const lightNameTimeLayout = "2006-01-02T15:04:05.000Z"

type lightNameView struct {
	ID        string  `json:"id"`
	Default   string  `json:"default"`
	Name      *string `json:"name"`
	Version   int     `json:"version"`
	UpdatedAt *string `json:"updated_at"`
}

func lightNameViewOf(e domain.LightNameEntry, row data.LightNameRow, has bool) lightNameView {
	v := lightNameView{ID: e.ID, Default: e.Default}
	if has {
		v.Name = row.Name
		v.Version = row.Version
		ts := row.UpdatedAt.UTC().Format(lightNameTimeLayout)
		v.UpdatedAt = &ts
	}
	return v
}

func lightNameViews(rows map[string]data.LightNameRow) []lightNameView {
	out := make([]lightNameView, 0, 33)
	for _, e := range domain.LightNameCatalog() {
		row, has := rows[e.ID]
		out = append(out, lightNameViewOf(e, row, has))
	}
	return out
}

// GetLightNames answers the 33 lights of the caller's linked cabinet.
func (h *Handler) GetLightNames(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || user.LinkedMachineID == nil || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	rows, err := h.store.ListLightNames(r.Context(), *user.LinkedMachineID)
	if err != nil {
		log.Printf("[portal] light names list: %v", err)
		writeLTError(w, http.StatusInternalServerError, "names_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lights": lightNameViews(rows)})
}

// PutLightName renames one light of the caller's linked cabinet. Only the
// cabinet administrator may write; the role is read fresh from the database.
func (h *Handler) PutLightName(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || !domain.CanWriteLightNames(user.Role, user.LinkedMachineID) || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	entry, ok := domain.LightNameByID(chi.URLParam(r, "id"))
	if !ok {
		writeLTError(w, http.StatusBadRequest, "unknown_light")
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	name, err := parseLightNameBody(raw)
	if err != nil {
		writeLTError(w, http.StatusBadRequest, "invalid_name")
		return
	}
	if testmode.IsDryRun(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"id": entry.ID, "default": entry.Default, "name": name,
			"status": "dry_run", "dry_run": true,
		})
		return
	}
	machineID := *user.LinkedMachineID
	before, err := h.store.ListLightNames(r.Context(), machineID)
	if err != nil {
		log.Printf("[portal] light name read: %v", err)
		writeLTError(w, http.StatusInternalServerError, "names_unavailable")
		return
	}
	row, err := h.store.SetLightName(r.Context(), machineID, entry.ID, name, email)
	if err != nil {
		log.Printf("[portal] light name write: %v", err)
		writeLTError(w, http.StatusInternalServerError, "names_unavailable")
		return
	}
	_ = h.store.AuditLog(r.Context(), email, "light_name", map[string]any{
		"id": entry.ID, "machine_id": machineID, "version": row.Version,
		"old_name": before[entry.ID].Name, "new_name": row.Name,
	})
	resp := map[string]any{"status": "saved"}
	b, _ := json.Marshal(lightNameViewOf(entry, row, true))
	_ = json.Unmarshal(b, &resp)
	writeJSON(w, http.StatusOK, resp)
}

// parseLightNameBody reads {"name": "..."} or {"name": null}. A missing key,
// a non-string value or an invalid name is an error; an empty name resets.
func parseLightNameBody(raw []byte) (*string, error) {
	var body struct {
		Name json.RawMessage `json:"name"`
	}
	if err := json.Unmarshal(raw, &body); err != nil || len(body.Name) == 0 {
		return nil, domain.ErrLightName
	}
	if string(body.Name) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(body.Name, &s); err != nil {
		return nil, domain.ErrLightName
	}
	return domain.NormalizeLightName(s)
}

type lightNameSyncEntry struct {
	ID        string          `json:"id"`
	Name      json.RawMessage `json:"name"`
	Version   int             `json:"version"`
	UpdatedAt string          `json:"updated_at"`
}

// SyncLightNames merges the names sent by the authenticated gateway with the
// cloud state of its cabinet, light by light, and answers the merged state.
// Entries that cannot be read (unknown id, invalid name, no usable timestamp)
// are ignored so that one bad entry never blocks the others.
func (h *Handler) SyncLightNames(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	machineID, err := h.store.MachineIDForGateway(r.Context(), gatewayID)
	if err != nil {
		http.Error(w, "Unknown gateway", http.StatusUnauthorized)
		return
	}
	var body struct {
		Names []lightNameSyncEntry `json:"names"`
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if json.Unmarshal(raw, &body) != nil || len(body.Names) > len(domain.LightNameCatalog()) {
		writeLTError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	rows, err := h.store.ListLightNames(r.Context(), machineID)
	if err != nil {
		log.Printf("[gateway] light names list: %v", err)
		writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
		return
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, in := range body.Names {
		if _, ok := domain.LightNameByID(in.ID); !ok || in.Version <= 0 {
			continue
		}
		at, perr := time.Parse(time.RFC3339Nano, strings.TrimSpace(in.UpdatedAt))
		if perr != nil {
			continue
		}
		gw := domain.LightNameState{Version: in.Version,
			UpdatedAt: domain.ClampLightNameTime(at.UTC().Truncate(time.Millisecond), now)}
		if s := string(in.Name); len(in.Name) > 0 && s != "null" {
			var str string
			if json.Unmarshal(in.Name, &str) != nil {
				continue
			}
			n, nerr := domain.NormalizeLightName(str)
			if nerr != nil {
				continue
			}
			gw.Name = n
		}
		cur, has := rows[in.ID]
		// The request carries no author: on equal updated_at and version the
		// stored cloud value is kept.
		gw.UpdatedBy = cur.UpdatedBy
		if has && !domain.LightNameNewer(gw, domain.LightNameState{Name: cur.Name, Version: cur.Version, UpdatedAt: cur.UpdatedAt, UpdatedBy: cur.UpdatedBy}) {
			continue
		}
		merged := data.LightNameRow{LightID: in.ID, Name: gw.Name, Version: gw.Version, UpdatedAt: gw.UpdatedAt, UpdatedBy: "gateway"}
		if err := h.store.PutLightNameExact(r.Context(), machineID, in.ID, merged); err != nil {
			log.Printf("[gateway] light name merge %s: %v", in.ID, err)
			writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
			return
		}
		_ = h.store.AuditLog(r.Context(), "gateway:"+gatewayID, "light_name", map[string]any{
			"id": in.ID, "machine_id": machineID, "version": merged.Version,
			"old_name": cur.Name, "new_name": merged.Name, "source": "gateway_sync",
		})
		rows[in.ID] = merged
	}
	writeJSON(w, http.StatusOK, map[string]any{"lights": lightNameViews(rows)})
}
