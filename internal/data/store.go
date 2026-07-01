package data

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type PortalStore struct {
	db *sqlx.DB
}

func NewPortalStore(db *sqlx.DB) *PortalStore {
	return &PortalStore{db: db}
}

func (s *PortalStore) RunMigrations(paths ...string) error {
	for _, path := range paths {
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if _, err := s.db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *PortalStore) GetUserByEmail(ctx context.Context, email string) (*domain.UserProfile, error) {
	var u domain.UserProfile
	err := s.db.GetContext(ctx, &u, `
		SELECT id, email, role, first_name, last_name,
		       linked_machine_id, linked_gateway_id, linked_armoire_id
		FROM users WHERE email = $1`, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *PortalStore) CreateLinkRequest(ctx context.Context, userID int, serial, message string) (*domain.LinkRequest, error) {
	var lr domain.LinkRequest
	err := s.db.GetContext(ctx, &lr, `
		INSERT INTO link_requests (user_id, machine_serial, message, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, machine_serial, message, status, reviewed_by, reviewed_at, created_at`,
		userID, serial, nullIfEmpty(message), domain.LinkStatusPending)
	return &lr, err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *PortalStore) GetLatestLinkRequest(ctx context.Context, userID int) (*domain.LinkRequest, error) {
	var lr domain.LinkRequest
	err := s.db.GetContext(ctx, &lr, `
		SELECT id, user_id, machine_serial, message, status, reviewed_by, reviewed_at, created_at
		FROM link_requests WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID)
	return &lr, err
}

func (s *PortalStore) ListLinkRequestsByStatus(ctx context.Context, status string) ([]domain.LinkRequest, error) {
	var rows []domain.LinkRequest
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, user_id, machine_serial, message, status, reviewed_by, reviewed_at, created_at
		FROM link_requests WHERE status = $1 ORDER BY created_at ASC`, status)
	return rows, err
}

// ListLinkRequestsAdmin returns link requests for admin UI.
// filter: "pending" (FIFO), "history" (approved/rejected, newest first), or "all".
func (s *PortalStore) ListLinkRequestsAdmin(ctx context.Context, filter string, limit int) ([]domain.LinkRequestAdminView, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	base := `
		SELECT lr.id, lr.user_id, lr.machine_serial, lr.message, lr.status,
		       lr.reviewed_by, lr.reviewed_at, lr.created_at,
		       u.email AS user_email, u.first_name, u.last_name
		FROM link_requests lr
		JOIN users u ON u.id = lr.user_id`
	var query string
	switch filter {
	case "history":
		query = base + `
		WHERE lr.status IN ($2, $3)
		ORDER BY COALESCE(lr.reviewed_at, lr.created_at) DESC
		LIMIT $1`
		var rows []domain.LinkRequestAdminView
		err := s.db.SelectContext(ctx, &rows, query, limit, domain.LinkStatusApproved, domain.LinkStatusRejected)
		return rows, err
	case "all":
		query = base + `
		ORDER BY lr.created_at DESC
		LIMIT $1`
	default:
		query = base + `
		WHERE lr.status = $2
		ORDER BY lr.created_at ASC
		LIMIT $1`
		var rows []domain.LinkRequestAdminView
		err := s.db.SelectContext(ctx, &rows, query, limit, domain.LinkStatusPending)
		return rows, err
	}
	var rows []domain.LinkRequestAdminView
	err := s.db.SelectContext(ctx, &rows, query, limit)
	return rows, err
}

func (s *PortalStore) UpdateLinkRequestStatus(ctx context.Context, id int, status, reviewer string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE link_requests SET status = $1, reviewed_by = $2, reviewed_at = NOW() WHERE id = $3`,
		status, reviewer, id)
	return err
}

func (s *PortalStore) UserHasApprovedLink(ctx context.Context, userID int) (bool, error) {
	var status string
	err := s.db.GetContext(ctx, &status, `
		SELECT status FROM link_requests
		WHERE user_id = $1 AND status = $2
		ORDER BY created_at DESC LIMIT 1`, userID, domain.LinkStatusApproved)
	if err != nil {
		return false, nil
	}
	return status == domain.LinkStatusApproved, nil
}

func (s *PortalStore) EnqueueCloudAction(ctx context.Context, guid string, userID int, machineID *int, params []domain.ExchangeKV) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO cloud_actions (guid, user_id, machine_id, params, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (guid) DO NOTHING`,
		guid, userID, machineID, raw, domain.ActionStatusPending)
	return err
}

func (s *PortalStore) GetLastCloudActionForUser(ctx context.Context, userID int) (*domain.CloudAction, error) {
	var row struct {
		GUID      string    `db:"guid"`
		UserID    int       `db:"user_id"`
		MachineID *int      `db:"machine_id"`
		Params    []byte    `db:"params"`
		Status    string    `db:"status"`
		CreatedAt time.Time `db:"created_at"`
	}
	err := s.db.GetContext(ctx, &row, `
		SELECT guid, user_id, machine_id, params, status, created_at
		FROM cloud_actions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, userID)
	if err != nil {
		return nil, err
	}
	var params []domain.ExchangeKV
	if err := json.Unmarshal(row.Params, &params); err != nil {
		return nil, err
	}
	return &domain.CloudAction{
		GUID: row.GUID, UserID: row.UserID, MachineID: row.MachineID,
		Params: params, Status: row.Status, CreatedAt: row.CreatedAt,
	}, nil
}

func (s *PortalStore) FetchPendingActionsForMachine(ctx context.Context, machineID int, limit int) ([]domain.CloudAction, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []struct {
		GUID      string    `db:"guid"`
		UserID    int       `db:"user_id"`
		MachineID *int      `db:"machine_id"`
		Params    []byte    `db:"params"`
		Status    string    `db:"status"`
		CreatedAt time.Time `db:"created_at"`
	}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT guid, user_id, machine_id, params, status, created_at
		FROM cloud_actions
		WHERE status = $1 AND machine_id = $3
		ORDER BY created_at ASC
		LIMIT $2`, domain.ActionStatusPending, limit, machineID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	paramChunks := make([][]domain.ExchangeKV, 0, len(rows))
	guids := make([]string, 0, len(rows))
	for _, r := range rows {
		var params []domain.ExchangeKV
		if err := json.Unmarshal(r.Params, &params); err != nil {
			return nil, err
		}
		paramChunks = append(paramChunks, params)
		guids = append(guids, r.GUID)
	}

	last := rows[len(rows)-1]
	deliverParams := paramChunks[len(paramChunks)-1]
	if len(rows) > 1 {
		deliverParams = domain.MergeExchangeParams(paramChunks...)
		log.Printf("[portal] coalesced %d pending cloud_actions for machine %d into guid %s",
			len(rows), machineID, last.GUID)
	}

	for _, guid := range guids {
		_, _ = s.db.ExecContext(ctx, `
			UPDATE cloud_actions SET status = $1, delivered_at = NOW() WHERE guid = $2`,
			domain.ActionStatusDelivered, guid)
	}

	return []domain.CloudAction{{
		GUID: last.GUID, UserID: last.UserID, MachineID: last.MachineID,
		Params: deliverParams, Status: domain.ActionStatusPending, CreatedAt: last.CreatedAt,
	}}, nil
}

func (s *PortalStore) MachineIDFromHashedPkey(ctx context.Context, hashedPkey string) (int, error) {
	var row struct {
		ClientID *string `db:"client_id"`
		ID       int     `db:"id"`
	}
	err := s.db.GetContext(ctx, &row, `
		SELECT client_id, id FROM machines WHERE hashed_pkey = $1`, hashedPkey)
	if err != nil {
		return 0, fmt.Errorf("unknown machine")
	}
	if row.ClientID != nil && *row.ClientID != "" {
		var machineID int
		if _, err := fmt.Sscanf(*row.ClientID, "%d", &machineID); err == nil && machineID > 0 {
			return machineID, nil
		}
	}
	if row.ID > 0 {
		return row.ID, nil
	}
	return 0, fmt.Errorf("non-numeric client_id for machine")
}

func (s *PortalStore) FetchPendingActionsForGateway(ctx context.Context, gatewayID string, limit int) ([]domain.CloudAction, error) {
	sess, err := s.GetGatewaySession(ctx, gatewayID)
	if err != nil || sess == nil {
		return nil, fmt.Errorf("unknown gateway")
	}
	if sess.MachineID == nil {
		return nil, fmt.Errorf("gateway %s has no machine_id — re-register with triplet", gatewayID)
	}
	return s.FetchPendingActionsForMachine(ctx, *sess.MachineID, limit)
}

func (s *PortalStore) MarkActionDone(ctx context.Context, guid string) error {
	var machineID *int
	err := s.db.GetContext(ctx, &machineID, `
		SELECT machine_id FROM cloud_actions WHERE guid = $1`, guid)
	if err != nil {
		return fmt.Errorf("action not found")
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE cloud_actions SET status = $1, done_at = NOW()
		WHERE guid = $2
		   OR (machine_id = $3 AND status = $4)`,
		domain.ActionStatusDone, guid, machineID, domain.ActionStatusDelivered)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("action not found")
	}
	return nil
}

type GatewayRegistration struct {
	GatewayID string
	Token     string
	MachineID int
	Eth0MAC   string
	Eth1MAC   string
}

func (s *PortalStore) RegisterGatewaySession(ctx context.Context, reg GatewayRegistration) error {
	eth0, err := NormalizeMAC(reg.Eth0MAC)
	if err != nil {
		return fmt.Errorf("eth0_mac: %w", err)
	}
	eth1, err := NormalizeMAC(reg.Eth1MAC)
	if err != nil {
		return fmt.Errorf("eth1_mac: %w", err)
	}
	if reg.MachineID <= 0 {
		return fmt.Errorf("machine_id required")
	}
	gatewayID := reg.GatewayID
	if gatewayID == "" {
		gatewayID, err = GatewayIDFromEth0MAC(eth0)
		if err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO gateway_sessions (gateway_id, token_hash, machine_id, eth0_mac, eth1_mac, last_seen)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (gateway_id) DO UPDATE SET
			token_hash = $2, machine_id = $3, eth0_mac = $4, eth1_mac = $5`,
		gatewayID, hashToken(reg.Token), reg.MachineID, eth0, eth1)
	return err
}

func (s *PortalStore) ValidateGatewayRequest(ctx context.Context, gatewayID, token, eth0Raw, eth1Raw string) bool {
	sess, err := s.GetGatewaySession(ctx, gatewayID)
	if err != nil || sess == nil {
		return false
	}
	if sess.TokenHash != hashToken(token) {
		return false
	}
	if sess.Eth0MAC == nil || sess.Eth1MAC == nil {
		// Legacy session without MAC triplet — token only
		return true
	}
	eth0, err := NormalizeMAC(eth0Raw)
	if err != nil {
		return false
	}
	eth1, err := NormalizeMAC(eth1Raw)
	if err != nil {
		return false
	}
	return eth0 == *sess.Eth0MAC && eth1 == *sess.Eth1MAC
}

func (s *PortalStore) ValidateGatewayToken(ctx context.Context, gatewayID, token string) bool {
	return s.ValidateGatewayRequest(ctx, gatewayID, token, "", "")
}

func (s *PortalStore) GetGatewaySession(ctx context.Context, gatewayID string) (*domain.GatewaySession, error) {
	var gs domain.GatewaySession
	err := s.db.GetContext(ctx, &gs, `
		SELECT gateway_id, token_hash, machine_id, eth0_mac, eth1_mac, last_seen
		FROM gateway_sessions WHERE gateway_id = $1`, gatewayID)
	if err != nil {
		return nil, err
	}
	return &gs, nil
}

func (s *PortalStore) ListGatewaySessions(ctx context.Context) ([]domain.GatewaySession, error) {
	var rows []domain.GatewaySession
	err := s.db.SelectContext(ctx, &rows, `
		SELECT gateway_id, machine_id, eth0_mac, eth1_mac, last_seen
		FROM gateway_sessions
		ORDER BY last_seen DESC NULLS LAST`)
	return rows, err
}

func (s *PortalStore) TouchGatewayHeartbeat(ctx context.Context, gatewayID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE gateway_sessions SET last_seen = NOW() WHERE gateway_id = $1`, gatewayID)
	return err
}

func (s *PortalStore) IsGatewayOnline(ctx context.Context, gatewayID string, timeout time.Duration) (bool, error) {
	var lastSeen *time.Time
	err := s.db.GetContext(ctx, &lastSeen, `
		SELECT last_seen FROM gateway_sessions WHERE gateway_id = $1`, gatewayID)
	if err != nil || lastSeen == nil {
		return false, err
	}
	return time.Since(*lastSeen) <= timeout, nil
}

func (s *PortalStore) AuditLog(ctx context.Context, email, action string, details map[string]any) error {
	raw, _ := json.Marshal(details)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO portal_audit_log (user_email, action, details) VALUES ($1, $2, $3)`,
		email, action, raw)
	return err
}
