package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/push"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/testmode"
)

// Doorbell ring received from a gateway (feature sonnette-plages-push-2026-11-008,
// #61). The gateway decides whether a ring counts (schedule); the cloud relays
// it to the devices of the users linked to that gateway.

const (
	// doorbellRingInterval : at most one ring per gateway in this window.
	doorbellRingInterval = 10 * time.Second
	// doorbellMaxFuture : a later `at` is brought back to the cloud clock.
	doorbellMaxFuture = 5 * time.Minute
	doorbellMaxRingID = 128
	// doorbellSendTimeout bounds the whole fan-out so the gateway gets its answer.
	doorbellSendTimeout = 8 * time.Second
)

// ringLimiter lets one ring through per gateway and interval.
type ringLimiter struct {
	interval time.Duration
	mu       sync.Mutex
	last     map[string]time.Time
}

func newRingLimiter(interval time.Duration) *ringLimiter {
	return &ringLimiter{interval: interval, last: map[string]time.Time{}}
}

// allow records the ring and answers true, or answers false if the previous
// accepted ring of this gateway is less than one interval old.
func (l *ringLimiter) allow(gatewayID string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, t := range l.last { // forget idle gateways
		if now.Sub(t) > 10*l.interval {
			delete(l.last, id)
		}
	}
	if t, ok := l.last[gatewayID]; ok && now.Sub(t) < l.interval {
		return false
	}
	l.last[gatewayID] = now
	return true
}

type doorbellRingBody struct {
	RingID string `json:"ring_id"`
	At     string `json:"at"`
}

// DoorbellRing receives a ring from an authenticated gateway.
func (h *Handler) DoorbellRing(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.Context().Value(middleware.GatewayIDKey).(string)
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
	var body doorbellRingBody
	if json.Unmarshal(raw, &body) != nil {
		writeLTError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	ringID := strings.TrimSpace(body.RingID)
	if ringID == "" || len(ringID) > doorbellMaxRingID {
		writeLTError(w, http.StatusBadRequest, "invalid_ring_id")
		return
	}
	now := time.Now().UTC()
	at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(body.At))
	if err != nil {
		writeLTError(w, http.StatusBadRequest, "invalid_at")
		return
	}
	at = at.UTC()
	if at.After(now.Add(doorbellMaxFuture)) {
		at = now
	}

	if testmode.IsDryRun(r) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "dry_run", "recipients": 0, "dry_run": true})
		return
	}

	seen, err := h.store.DoorbellRingSeen(r.Context(), gatewayID, ringID)
	if err != nil {
		log.Printf("[gateway] doorbell ring lookup: %v", err)
		writeLTError(w, http.StatusServiceUnavailable, "ring_unavailable")
		return
	}
	if seen {
		writeJSON(w, http.StatusOK, map[string]any{"status": "accepted", "recipients": 0})
		return
	}
	if !h.rings.allow(gatewayID, now) {
		w.Header().Set("Retry-After", "10")
		writeLTError(w, http.StatusTooManyRequests, "too_many_rings")
		return
	}
	devices, err := h.store.DoorbellRecipients(r.Context(), gatewayID)
	if err != nil {
		log.Printf("[gateway] doorbell recipients: %v", err)
		writeLTError(w, http.StatusServiceUnavailable, "ring_unavailable")
		return
	}
	fresh, err := h.store.RecordDoorbellRing(r.Context(), gatewayID, ringID)
	if err != nil {
		log.Printf("[gateway] doorbell ring record: %v", err)
		writeLTError(w, http.StatusServiceUnavailable, "ring_unavailable")
		return
	}
	if !fresh { // a concurrent duplicate won the race
		writeJSON(w, http.StatusOK, map[string]any{"status": "accepted", "recipients": 0})
		return
	}
	sent := h.notifyDevices(r.Context(), devices, push.DoorbellMessage(ringID, at.Format(time.RFC3339Nano)))
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted", "recipients": sent})
}

// notifyDevices sends to every device, one goroutine per platform so that a
// slow or failing provider never holds back the other. It returns the number
// of devices the provider accepted. Tokens never reach a log line.
func (h *Handler) notifyDevices(ctx context.Context, devices []data.PushDevice, msg push.Message) int {
	byPlatform := map[string][]data.PushDevice{}
	for _, d := range devices {
		byPlatform[d.Platform] = append(byPlatform[d.Platform], d)
	}
	ctx, cancel := context.WithTimeout(ctx, doorbellSendTimeout)
	defer cancel()
	var sent atomic.Int32
	var wg sync.WaitGroup
	for platform, list := range byPlatform {
		sender := h.push.For(platform)
		if sender == nil {
			continue
		}
		wg.Add(1)
		go func(platform string, sender push.PushSender, list []data.PushDevice) {
			defer wg.Done()
			for _, d := range list {
				err := sender.Send(ctx, d.Token, msg)
				switch {
				case err == nil:
					sent.Add(1)
				case isUnregistered(err):
					dctx, dcancel := context.WithTimeout(context.Background(), 3*time.Second)
					if derr := h.store.DeletePushDeviceByID(dctx, d.ID); derr != nil {
						log.Printf("[push] stale device %d not removed: %v", d.ID, derr)
					}
					dcancel()
				default:
					log.Printf("[push] %s send failed for device %d: %v", platform, d.ID, err)
				}
			}
		}(platform, sender, list)
	}
	wg.Wait()
	return int(sent.Load())
}

func isUnregistered(err error) bool { return errors.Is(err, push.ErrUnregistered) }
