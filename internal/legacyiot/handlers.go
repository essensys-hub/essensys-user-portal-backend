package legacyiot

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
)

type Handlers struct {
	store *data.LegacyIoTStore
}

func NewHandlers(store *data.LegacyIoTStore) *Handlers {
	return &Handlers{store: store}
}

func (h *Handlers) ServerInfos(w http.ResponseWriter, r *http.Request) {
	clientID, _ := r.Context().Value(middleware.LegacyClientIDKey).(string)
	log.Printf("[legacyiot] ServerInfos by %s", clientID)
	writeJSON(w, http.StatusOK, domain.ServerInfosResponse{
		IsConnected: false,
		Infos:       []int{363, 349, 350, 351, 352, 353, 11, 920},
		NewVersion:  "no",
	})
}

func (h *Handlers) MyStatus(w http.ResponseWriter, r *http.Request) {
	clientID, _ := r.Context().Value(middleware.LegacyClientIDKey).(string)
	var payload domain.MyStatusPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[legacyiot] MyStatus bad request from %s: %v", clientID, err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	log.Printf("[legacyiot] MyStatus from %s ver=%s keys=%d", clientID, payload.Version, len(payload.EK))
	if h.store != nil {
		if err := h.store.SaveClientData(clientID, payload.Version, payload.EK); err != nil {
			log.Printf("[legacyiot] save telemetry: %v", err)
		}
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handlers) MyActions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{}"))
}

func (h *Handlers) GatewayInfos(w http.ResponseWriter, r *http.Request) {
	var payload domain.GatewayStatus
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[legacyiot] gateway infos bad request: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	payload.IP = r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		payload.IP = host
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		payload.IP = fwd
	}
	payload.LastSeen = time.Now()
	if h.store != nil {
		if err := h.store.SaveGateway(&payload); err != nil {
			log.Printf("[legacyiot] save gateway: %v", err)
		} else {
			log.Printf("[legacyiot] gateway update %s (%s)", payload.Hostname, payload.IP)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
