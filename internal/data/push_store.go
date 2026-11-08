package data

import (
	"context"
	"time"
)

// PushDevice is one registered mobile device (migration 022). The token is
// only ever read by the notification dispatcher, never by an HTTP response.
type PushDevice struct {
	ID             int       `db:"id"`
	UserID         int       `db:"user_id"`
	Platform       string    `db:"platform"`
	Token          string    `db:"token"`
	AppVersion     string    `db:"app_version"`
	DoorbellNotify bool      `db:"doorbell_notify"`
	LastSeen       time.Time `db:"last_seen"`
}

// UpsertPushDevice registers a token for a user. The (platform, token) key is
// unique: a token already known moves to the current user.
func (s *PortalStore) UpsertPushDevice(ctx context.Context, userID int, platform, token, appVersion string, notify bool) (PushDevice, error) {
	var d PushDevice
	err := s.db.GetContext(ctx, &d, `
		INSERT INTO push_devices (user_id, platform, token, app_version, doorbell_notify)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (platform, token) DO UPDATE
		SET user_id = EXCLUDED.user_id, app_version = EXCLUDED.app_version,
		    doorbell_notify = EXCLUDED.doorbell_notify,
		    updated_at = NOW(), last_seen = NOW()
		RETURNING id, user_id, platform, token, app_version, doorbell_notify, last_seen`,
		userID, platform, token, appVersion, notify)
	return d, err
}

// DeletePushDevice removes a device owned by userID; false when it is unknown
// or belongs to someone else.
func (s *PortalStore) DeletePushDevice(ctx context.Context, userID, id int) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM push_devices WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeletePushDeviceByID removes a device whose token the provider refused.
func (s *PortalStore) DeletePushDeviceByID(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM push_devices WHERE id = $1`, id)
	return err
}

// ListPushDevices returns the devices of one user (tokens included: callers
// must not expose them).
func (s *PortalStore) ListPushDevices(ctx context.Context, userID int) ([]PushDevice, error) {
	var out []PushDevice
	err := s.db.SelectContext(ctx, &out, `
		SELECT id, user_id, platform, token, app_version, doorbell_notify, last_seen
		FROM push_devices WHERE user_id = $1 ORDER BY id`, userID)
	return out, err
}
