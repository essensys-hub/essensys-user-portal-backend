package legacyiot

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	store  *data.LegacyIoTStore
	portal *data.PortalStore
}

func NewHandlers(store *data.LegacyIoTStore, portal *data.PortalStore) *Handlers {
	return &Handlers{store: store, portal: portal}
}

// serverInfoIndices matches essensys-server-backend GetServerInfos (mystatus poll list).
func serverInfoIndices() []int {
	indices := []int{613, 607, 615, 590, 349, 350, 351, 352, 363, 425, 426, 920,
		566, 567, 568, 569, 570, 571, 572,
		574, 575, 576, 577, 578,
		582, 583, 584, 585}
	// Planning chauffage (13–348) : NE PAS lister ici. Le firmware BP_MQX_ETH (099-37)
	// accepte au maximum 30 indices dans serverinfos (Json.c → ERREUR_INFOS_NB_VALEURS_MAX).
	// Au-delà, le cycle Ethernet s'arrête après GET serverinfos : pas de mystatus ni myactions.
	// Écriture planning : POST /api/portal/inject/batch (≤30 params/action). Lecture UI : exchange.
	return indices
}

func (h *Handlers) ServerInfos(w http.ResponseWriter, r *http.Request) {
	clientID, _ := r.Context().Value(middleware.LegacyClientIDKey).(string)
	log.Printf("[legacyiot] ServerInfos by %s", clientID)
	writeJSON(w, http.StatusOK, domain.ServerInfosResponse{
		IsConnected: true,
		Infos:       serverInfoIndices(),
		NewVersion:  "no",
	})
}

func (h *Handlers) MyStatus(w http.ResponseWriter, r *http.Request) {
	clientID, _ := r.Context().Value(middleware.LegacyClientIDKey).(string)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	normalized, err := NormalizeJSON(body)
	if err != nil {
		log.Printf("[legacyiot] MyStatus bad request from %s: %v", clientID, err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	var payload domain.MyStatusPayload
	if err := json.Unmarshal(normalized, &payload); err != nil {
		log.Printf("[legacyiot] MyStatus parse from %s: %v", clientID, err)
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
	resp := domain.LegacyActionsResponse{Actions: []domain.LegacyAction{}}
	if h.portal != nil {
		hashedPkey, _ := r.Context().Value(middleware.LegacyHashedPkeyKey).(string)
		if hashedPkey != "" {
			var machineID int
			var err error
			if h.store != nil {
				machineID, err = h.store.PortalMachineIDFromHashedPkey(hashedPkey)
			} else {
				machineID, err = h.portal.MachineIDFromHashedPkey(r.Context(), hashedPkey)
			}
			if err == nil {
				actions, err := h.portal.FetchPendingActionsForMachine(r.Context(), machineID, 20)
				if err != nil {
					log.Printf("[legacyiot] myactions machine %d: %v", machineID, err)
				} else if len(actions) > 0 {
					resp.Actions = make([]domain.LegacyAction, 0, len(actions))
					for _, act := range actions {
						resp.Actions = append(resp.Actions, domain.LegacyAction{
							GUID:   act.GUID,
							Params: act.Params,
						})
					}
					log.Printf("[legacyiot] myactions delivered %d action(s) to machine %d", len(actions), machineID)
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json ;charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) Done(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	if guid == "" {
		http.Error(w, "guid required", http.StatusBadRequest)
		return
	}
	if h.portal == nil {
		http.Error(w, "Not configured", http.StatusServiceUnavailable)
		return
	}
	if err := h.portal.MarkActionDone(r.Context(), guid); err != nil {
		http.Error(w, "Action not found", http.StatusNotFound)
		return
	}
	clientID, _ := r.Context().Value(middleware.LegacyClientIDKey).(string)
	log.Printf("[legacyiot] done %s from %s", guid, clientID)
	w.Header().Set("Content-Type", "application/json ;charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}"))
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
