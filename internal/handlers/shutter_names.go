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

// Custom shutter and blind names (feature noms-volets-2026-11-005, #57). Twin
// of the shutter names: the cloud is the arbiter, the cloud portal writes here,
// the gateway syncs its own copy.

type shutterNameView struct {
	ID        string  `json:"id"`
	Default   string  `json:"default"`
	Name      *string `json:"name"`
	Version   int     `json:"version"`
	UpdatedAt *string `json:"updated_at"`
}

func shutterNameViewOf(e domain.ShutterNameEntry, row data.ShutterNameRow, has bool) shutterNameView {
	v := shutterNameView{ID: e.ID, Default: e.Default}
	if has {
		v.Name = row.Name
		v.Version = row.Version
		ts := row.UpdatedAt.UTC().Format(lightNameTimeLayout)
		v.UpdatedAt = &ts
	}
	return v
}

func shutterNameViews(rows map[string]data.ShutterNameRow) []shutterNameView {
	out := make([]shutterNameView, 0, len(domain.ShutterNameCatalog()))
	for _, e := range domain.ShutterNameCatalog() {
		row, has := rows[e.ID]
		out = append(out, shutterNameViewOf(e, row, has))
	}
	return out
}

// GetShutterNames answers the 16 shutters and blinds of the caller's linked cabinet.
func (h *Handler) GetShutterNames(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || user.LinkedMachineID == nil || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	rows, err := h.store.ListShutterNames(r.Context(), *user.LinkedMachineID)
	if err != nil {
		log.Printf("[portal] shutter names list: %v", err)
		writeLTError(w, http.StatusInternalServerError, "names_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shutters": shutterNameViews(rows)})
}

// PutShutterName renames one shutter or blind of the caller's linked cabinet. Only the
// cabinet administrator may write; the role is read fresh from the database.
func (h *Handler) PutShutterName(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil || !domain.CanWriteShutterNames(user.Role, user.LinkedMachineID) || !h.userPortalAccess(r.Context(), user) {
		writeLTError(w, http.StatusForbidden, "forbidden")
		return
	}
	entry, ok := domain.ShutterNameByID(chi.URLParam(r, "id"))
	if !ok {
		writeLTError(w, http.StatusBadRequest, "unknown_shutter")
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	name, err := parseShutterNameBody(raw)
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
	before, err := h.store.ListShutterNames(r.Context(), machineID)
	if err != nil {
		log.Printf("[portal] shutter name read: %v", err)
		writeLTError(w, http.StatusInternalServerError, "names_unavailable")
		return
	}
	row, err := h.store.SetShutterName(r.Context(), machineID, entry.ID, name, email)
	if err != nil {
		log.Printf("[portal] shutter name write: %v", err)
		writeLTError(w, http.StatusInternalServerError, "names_unavailable")
		return
	}
	_ = h.store.AuditLog(r.Context(), email, "shutter_name", map[string]any{
		"id": entry.ID, "machine_id": machineID, "version": row.Version,
		"old_name": before[entry.ID].Name, "new_name": row.Name,
	})
	resp := map[string]any{"status": "saved"}
	b, _ := json.Marshal(shutterNameViewOf(entry, row, true))
	_ = json.Unmarshal(b, &resp)
	writeJSON(w, http.StatusOK, resp)
}

// parseShutterNameBody reads {"name": "..."} or {"name": null}. A missing key,
// a non-string value or an invalid name is an error; an empty name resets.
func parseShutterNameBody(raw []byte) (*string, error) {
	var body struct {
		Name json.RawMessage `json:"name"`
	}
	if err := json.Unmarshal(raw, &body); err != nil || len(body.Name) == 0 {
		return nil, domain.ErrShutterName
	}
	if string(body.Name) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(body.Name, &s); err != nil {
		return nil, domain.ErrShutterName
	}
	return domain.NormalizeShutterName(s)
}

type shutterNameSyncEntry struct {
	ID        string          `json:"id"`
	Name      json.RawMessage `json:"name"`
	Version   int             `json:"version"`
	UpdatedAt string          `json:"updated_at"`
}

// SyncShutterNames merges the names sent by the authenticated gateway with the
// cloud state of its cabinet, shutter by shutter, and answers the merged state.
// Entries that cannot be read (unknown id, invalid name, no usable timestamp)
// are ignored so that one bad entry never blocks the others.
func (h *Handler) SyncShutterNames(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	machineID, err := h.store.MachineIDForGateway(r.Context(), gatewayID)
	if err != nil {
		http.Error(w, "Unknown gateway", http.StatusUnauthorized)
		return
	}
	var body struct {
		Names []shutterNameSyncEntry `json:"names"`
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if json.Unmarshal(raw, &body) != nil || len(body.Names) > len(domain.ShutterNameCatalog()) {
		writeLTError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	rows, err := h.store.ListShutterNames(r.Context(), machineID)
	if err != nil {
		log.Printf("[gateway] shutter names list: %v", err)
		writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
		return
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, in := range body.Names {
		if _, ok := domain.ShutterNameByID(in.ID); !ok || in.Version <= 0 {
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
			n, nerr := domain.NormalizeShutterName(str)
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
		merged := data.ShutterNameRow{ShutterID: in.ID, Name: gw.Name, Version: gw.Version, UpdatedAt: gw.UpdatedAt, UpdatedBy: "gateway"}
		if err := h.store.PutShutterNameExact(r.Context(), machineID, in.ID, merged); err != nil {
			log.Printf("[gateway] shutter name merge %s: %v", in.ID, err)
			writeLTError(w, http.StatusServiceUnavailable, "names_unavailable")
			return
		}
		_ = h.store.AuditLog(r.Context(), "gateway:"+gatewayID, "shutter_name", map[string]any{
			"id": in.ID, "machine_id": machineID, "version": merged.Version,
			"old_name": cur.Name, "new_name": merged.Name, "source": "gateway_sync",
		})
		rows[in.ID] = merged
	}
	writeJSON(w, http.StatusOK, map[string]any{"shutters": shutterNameViews(rows)})
}
