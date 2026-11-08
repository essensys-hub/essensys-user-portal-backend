package data

import (
	"context"
	"database/sql"
	"errors"
)

// DoorbellRingSeen reports whether this ring was already recorded.
func (s *PortalStore) DoorbellRingSeen(ctx context.Context, gatewayID, ringID string) (bool, error) {
	var one int
	err := s.db.GetContext(ctx, &one, `
		SELECT 1 FROM doorbell_rings WHERE gateway_id = $1 AND ring_id = $2`, gatewayID, ringID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// RecordDoorbellRing stores a ring; false when it was already there (a
// concurrent duplicate). Rings older than a day are dropped on the way.
func (s *PortalStore) RecordDoorbellRing(ctx context.Context, gatewayID, ringID string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO doorbell_rings (gateway_id, ring_id) VALUES ($1, $2)
		ON CONFLICT (gateway_id, ring_id) DO NOTHING`, gatewayID, ringID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	_, _ = s.db.ExecContext(ctx, `DELETE FROM doorbell_rings WHERE received_at < NOW() - INTERVAL '1 day'`)
	return n > 0, nil
}

// DoorbellRecipients returns the devices to notify for a gateway: users linked
// to it with doorbell_notify enabled.
func (s *PortalStore) DoorbellRecipients(ctx context.Context, gatewayID string) ([]PushDevice, error) {
	var out []PushDevice
	err := s.db.SelectContext(ctx, &out, `
		SELECT d.id, d.user_id, d.platform, d.token, d.app_version, d.doorbell_notify, d.last_seen
		FROM push_devices d JOIN users u ON u.id = d.user_id
		WHERE u.linked_gateway_id = $1 AND d.doorbell_notify = TRUE
		ORDER BY d.id`, gatewayID)
	return out, err
}
