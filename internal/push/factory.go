package push

import (
	"context"
	"log"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
)

// NewDispatcher builds the senders from the configuration. A provider whose
// settings are missing or invalid is left nil (disabled); the log names the
// provider only, never a value.
func NewDispatcher(ctx context.Context, cfg config.Config) *Dispatcher {
	d := &Dispatcher{}
	if cfg.FCMServiceAccountJSON != "" {
		if s, err := NewFCMSender(ctx, cfg.FCMServiceAccountJSON, cfg.FCMProjectID); err != nil {
			log.Printf("[push] fcm disabled: %v", err)
		} else {
			d.Android = s
		}
	}
	if cfg.APNsKeyP8 != "" || cfg.APNsKeyID != "" || cfg.APNsTeamID != "" {
		if s, err := NewAPNsSender(APNsConfig{
			KeyP8: cfg.APNsKeyP8, KeyID: cfg.APNsKeyID, TeamID: cfg.APNsTeamID,
			BundleID: cfg.APNsBundleID, Environment: cfg.APNsEnvironment,
		}); err != nil {
			log.Printf("[push] apns disabled: %v", err)
		} else {
			d.IOS = s
		}
	}
	return d
}
