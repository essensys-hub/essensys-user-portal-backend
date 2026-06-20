package data

import (
	"context"
	"log"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

const syncConsecutiveFailureThreshold = 3

// maybeAlertConsecutiveFailures logs when a profile has partial/failed runs N times in a row.
func (s *PortalStore) maybeAlertConsecutiveFailures(ctx context.Context, profileID, gatewayID string) {
	runs, err := s.ListSyncRunsForProfile(ctx, profileID, syncConsecutiveFailureThreshold)
	if err != nil || len(runs) < syncConsecutiveFailureThreshold {
		return
	}
	for _, run := range runs {
		if run.Status != domain.SyncRunStatusFailed && run.Status != domain.SyncRunStatusPartial {
			return
		}
	}
	profile, _ := s.GetSyncProfile(ctx, profileID)
	name := profileID
	if profile != nil {
		name = profile.Name
	}
	log.Printf("[sync-alert] CRITICAL profile %q gateway %s: %d consecutive partial/failed sync runs",
		name, gatewayID, syncConsecutiveFailureThreshold)
}
