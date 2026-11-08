package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Push devices and doorbell preferences (feature sonnette-plages-push-2026-11-008,
// #61). A device token is never returned and never logged.

const (
	pushPlatformAndroid = "android"
	pushPlatformIOS     = "ios"
	pushMaxTokenLen     = 4096
	pushMaxVersionLen   = 64
)

type pushDeviceView struct {
	ID             int    `json:"id"`
	Platform       string `json:"platform"`
	DoorbellNotify bool   `json:"doorbell_notify"`
}

type pushDeviceBody struct {
	Platform       string `json:"platform"`
	Token          string `json:"token"`
	AppVersion     string `json:"app_version"`
	DoorbellNotify bool   `json:"doorbell_notify"`
}

// PutPushDevice registers (or updates) a mobile device for the caller.
func (h *Handler) PutPushDevice(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		writeLTError(w, http.StatusNotFound, "user_not_found")
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	var body pushDeviceBody
	if err := json.Unmarshal(raw, &body); err != nil {
		writeLTError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	body.Platform = strings.ToLower(strings.TrimSpace(body.Platform))
	if body.Platform != pushPlatformAndroid && body.Platform != pushPlatformIOS {
		writeLTError(w, http.StatusBadRequest, "invalid_platform")
		return
	}
	body.Token = strings.TrimSpace(body.Token)
	if body.Token == "" || len(body.Token) > pushMaxTokenLen {
		writeLTError(w, http.StatusBadRequest, "invalid_token")
		return
	}
	version := strings.TrimSpace(body.AppVersion)
	if len(version) > pushMaxVersionLen {
		version = version[:pushMaxVersionLen]
	}
	d, err := h.store.UpsertPushDevice(r.Context(), user.ID, body.Platform, body.Token, version, body.DoorbellNotify)
	if err != nil {
		log.Printf("[portal] push device upsert: %v", err)
		writeLTError(w, http.StatusInternalServerError, "push_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, pushDeviceView{ID: d.ID, Platform: d.Platform, DoorbellNotify: d.DoorbellNotify})
}

// DeletePushDevice removes a device of the caller; anything else is a 404.
func (h *Handler) DeletePushDevice(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		writeLTError(w, http.StatusNotFound, "not_found")
		return
	}
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeLTError(w, http.StatusNotFound, "not_found")
		return
	}
	ok, err := h.store.DeletePushDevice(r.Context(), user.ID, id)
	if err != nil {
		log.Printf("[portal] push device delete: %v", err)
		writeLTError(w, http.StatusInternalServerError, "push_unavailable")
		return
	}
	if !ok {
		writeLTError(w, http.StatusNotFound, "not_found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetDoorbellPreferences lists the caller's devices (without tokens).
func (h *Handler) GetDoorbellPreferences(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(middleware.UserEmailKey).(string)
	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		writeLTError(w, http.StatusNotFound, "user_not_found")
		return
	}
	devices, err := h.store.ListPushDevices(r.Context(), user.ID)
	if err != nil {
		log.Printf("[portal] push devices list: %v", err)
		writeLTError(w, http.StatusInternalServerError, "push_unavailable")
		return
	}
	views := make([]pushDeviceView, 0, len(devices))
	for _, d := range devices {
		views = append(views, pushDeviceView{ID: d.ID, Platform: d.Platform, DoorbellNotify: d.DoorbellNotify})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available": h.gatewayState(r.Context(), user) != domain.GatewayStateNone,
		"devices":   views,
	})
}
