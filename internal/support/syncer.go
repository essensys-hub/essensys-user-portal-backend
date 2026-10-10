package support

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
)

// SyncInterval and RefreshAfter guarantee that a closed issue shows in
// « Mes signalements » within 15 minutes (D8).
const (
	SyncInterval = 5 * time.Minute
	RefreshAfter = 10 * time.Minute
)

// SyncOnce sends pending reports and refreshes the state of open ones. Only
// one backend instance does it at a time.
func (s *Service) SyncOnce(ctx context.Context) error {
	if s.GH == nil {
		return nil
	}
	err := s.Store.WithSyncLock(ctx, func() error {
		pending, err := s.Store.ListPending(ctx, 50)
		if err != nil {
			return err
		}
		for i := range pending {
			if err := s.send(ctx, &pending[i]); err != nil {
				log.Printf("support: retry %s: %v", pending[i].PublicRef, err)
			}
		}
		open, err := s.Store.ListOpenToSync(ctx, s.Now().Add(-RefreshAfter), 100)
		if err != nil {
			return err
		}
		for _, r := range open {
			if !r.Repo.Valid || !r.IssueNumber.Valid {
				continue
			}
			iss, err := s.GH.GetIssue(ctx, r.Repo.String, int(r.IssueNumber.Int64))
			if err != nil {
				log.Printf("support: refresh %s: %v", r.PublicRef, err)
				continue
			}
			if err := s.Store.UpdateStatus(ctx, r.ID, StatusFromIssue(iss)); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, data.ErrSyncLockBusy) {
		return nil
	}
	return err
}

// Run calls SyncOnce every interval until ctx is cancelled.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := s.SyncOnce(ctx); err != nil {
			log.Printf("support: sync: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
