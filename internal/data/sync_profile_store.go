package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/syncprofile"
)

type syncProfileRow struct {
	ID              string         `db:"id"`
	GatewayID       string         `db:"gateway_id"`
	Name            string         `db:"name"`
	IndexRanges     []byte         `db:"index_ranges"`
	IntervalHours   int            `db:"interval_hours"`
	CronExpression  sql.NullString `db:"cron_expression"`
	PullFromArmoire bool           `db:"pull_from_armoire"`
	PushToCloud     bool           `db:"push_to_cloud"`
	Enabled         bool           `db:"enabled"`
	ExcludeIndices  []byte         `db:"exclude_indices"`
	LastRunAt       sql.NullTime   `db:"last_run_at"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
	LastRunStatus   sql.NullString `db:"last_run_status"`
}

func nonNilExclude(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

func decodeProfile(row syncProfileRow) (domain.SyncProfile, error) {
	var ranges []domain.IndexRange
	if err := json.Unmarshal(row.IndexRanges, &ranges); err != nil {
		return domain.SyncProfile{}, err
	}
	var exclude []int
	if len(row.ExcludeIndices) > 0 {
		if err := json.Unmarshal(row.ExcludeIndices, &exclude); err != nil {
			return domain.SyncProfile{}, err
		}
	}
	p := domain.SyncProfile{
		ID:              row.ID,
		GatewayID:       row.GatewayID,
		Name:            row.Name,
		IndexRanges:     ranges,
		IntervalHours:   row.IntervalHours,
		PullFromArmoire: row.PullFromArmoire,
		PushToCloud:     row.PushToCloud,
		Enabled:         row.Enabled,
		ExcludeIndices:  exclude,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if row.CronExpression.Valid && row.CronExpression.String != "" {
		cron := row.CronExpression.String
		p.CronExpression = &cron
	}
	if row.LastRunAt.Valid {
		t := row.LastRunAt.Time
		p.LastRunAt = &t
	}
	if row.LastRunStatus.Valid {
		s := row.LastRunStatus.String
		p.LastRunStatus = &s
	}
	if p.Enabled && p.LastRunAt != nil && p.IntervalHours > 0 {
		next := p.LastRunAt.Add(time.Duration(p.IntervalHours) * time.Hour)
		p.NextRunAt = &next
	}
	return p, nil
}

func (s *PortalStore) ListSyncProfiles(ctx context.Context, gatewayID string) ([]domain.SyncProfile, error) {
	query := `
		SELECT p.id, p.gateway_id, p.name, p.index_ranges, p.interval_hours, p.cron_expression,
		       p.pull_from_armoire, p.push_to_cloud, p.enabled, p.exclude_indices, p.last_run_at,
		       p.created_at, p.updated_at,
		       (SELECT r.status FROM sync_runs r WHERE r.profile_id = p.id
		        ORDER BY r.created_at DESC LIMIT 1) AS last_run_status
		FROM sync_profiles p`
	args := []any{}
	if gatewayID != "" {
		query += ` WHERE p.gateway_id = $1 OR p.gateway_id = ''`
		args = append(args, gatewayID)
	}
	query += ` ORDER BY p.name ASC`

	var rows []syncProfileRow
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	out := make([]domain.SyncProfile, 0, len(rows))
	for _, row := range rows {
		p, err := decodeProfile(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *PortalStore) GetSyncProfile(ctx context.Context, id string) (*domain.SyncProfile, error) {
	var row syncProfileRow
	err := s.db.GetContext(ctx, &row, `
		SELECT p.id, p.gateway_id, p.name, p.index_ranges, p.interval_hours, p.cron_expression,
		       p.pull_from_armoire, p.push_to_cloud, p.enabled, p.exclude_indices, p.last_run_at,
		       p.created_at, p.updated_at,
		       (SELECT r.status FROM sync_runs r WHERE r.profile_id = p.id
		        ORDER BY r.created_at DESC LIMIT 1) AS last_run_status
		FROM sync_profiles p WHERE p.id = $1`, id)
	if err != nil {
		return nil, err
	}
	p, err := decodeProfile(row)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PortalStore) CreateSyncProfile(ctx context.Context, req domain.UpsertSyncProfileRequest) (*domain.SyncProfile, error) {
	if err := syncprofile.ValidateIndexRanges(req.IndexRanges); err != nil {
		return nil, err
	}
	interval := req.IntervalHours
	if interval <= 0 {
		interval = 3
	}
	pull := true
	push := true
	enabled := true
	if req.PullFromArmoire != nil {
		pull = *req.PullFromArmoire
	}
	if req.PushToCloud != nil {
		push = *req.PushToCloud
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	raw, err := json.Marshal(req.IndexRanges)
	if err != nil {
		return nil, err
	}
	excludeRaw, err := json.Marshal(nonNilExclude(req.ExcludeIndices))
	if err != nil {
		return nil, err
	}
	var id string
	err = s.db.GetContext(ctx, &id, `
		INSERT INTO sync_profiles (gateway_id, name, index_ranges, interval_hours, cron_expression,
			pull_from_armoire, push_to_cloud, enabled, exclude_indices, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id`,
		req.GatewayID, req.Name, raw, interval, req.CronExpression, pull, push, enabled, excludeRaw)
	if err != nil {
		return nil, err
	}
	return s.GetSyncProfile(ctx, id)
}

func (s *PortalStore) UpdateSyncProfile(ctx context.Context, id string, req domain.UpsertSyncProfileRequest) (*domain.SyncProfile, error) {
	if err := syncprofile.ValidateIndexRanges(req.IndexRanges); err != nil {
		return nil, err
	}
	interval := req.IntervalHours
	if interval <= 0 {
		interval = 3
	}
	pull := true
	push := true
	enabled := true
	if req.PullFromArmoire != nil {
		pull = *req.PullFromArmoire
	}
	if req.PushToCloud != nil {
		push = *req.PushToCloud
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	raw, err := json.Marshal(req.IndexRanges)
	if err != nil {
		return nil, err
	}
	excludeRaw, err := json.Marshal(nonNilExclude(req.ExcludeIndices))
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE sync_profiles SET gateway_id=$1, name=$2, index_ranges=$3, interval_hours=$4,
			cron_expression=$5, pull_from_armoire=$6, push_to_cloud=$7, enabled=$8,
			exclude_indices=$9, updated_at=NOW()
		WHERE id=$10`,
		req.GatewayID, req.Name, raw, interval, req.CronExpression, pull, push, enabled, excludeRaw, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("profile not found")
	}
	return s.GetSyncProfile(ctx, id)
}

func (s *PortalStore) DeleteSyncProfile(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sync_profiles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("profile not found")
	}
	return nil
}

func (s *PortalStore) ListSyncProfilesForGateway(ctx context.Context, gatewayID string) ([]domain.SyncProfile, error) {
	return s.ListSyncProfiles(ctx, gatewayID)
}

func (s *PortalStore) ListPendingSyncRuns(ctx context.Context, gatewayID string) ([]domain.SyncRun, error) {
	var rows []syncRunRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, profile_id, gateway_id, status, expected_count, received_count,
		       pushed_count, error_message, log_lines, started_at, finished_at, created_at
		FROM sync_runs
		WHERE gateway_id = $1 AND status IN ('pending', 'running')
		ORDER BY created_at ASC`, gatewayID)
	if err != nil {
		return nil, err
	}
	return decodeSyncRuns(rows)
}

type syncRunRow struct {
	ID            string       `db:"id"`
	ProfileID     string       `db:"profile_id"`
	GatewayID     string       `db:"gateway_id"`
	Status        string       `db:"status"`
	ExpectedCount int          `db:"expected_count"`
	ReceivedCount int          `db:"received_count"`
	PushedCount   int          `db:"pushed_count"`
	ErrorMessage  *string      `db:"error_message"`
	LogLines      []byte       `db:"log_lines"`
	StartedAt     sql.NullTime `db:"started_at"`
	FinishedAt    sql.NullTime `db:"finished_at"`
	CreatedAt     time.Time    `db:"created_at"`
}

func decodeSyncRuns(rows []syncRunRow) ([]domain.SyncRun, error) {
	out := make([]domain.SyncRun, 0, len(rows))
	for _, row := range rows {
		run, err := decodeSyncRunRow(row.ID, row.ProfileID, row.GatewayID, row.Status,
			row.ExpectedCount, row.ReceivedCount, row.PushedCount, row.ErrorMessage,
			row.LogLines, row.StartedAt, row.FinishedAt, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func decodeSyncRunRow(id, profileID, gatewayID, status string,
	expected, received, pushed int, errMsg *string, logRaw []byte,
	started, finished sql.NullTime, created time.Time) (domain.SyncRun, error) {
	var logs []domain.SyncLogLine
	if len(logRaw) > 0 {
		if err := json.Unmarshal(logRaw, &logs); err != nil {
			return domain.SyncRun{}, err
		}
	}
	run := domain.SyncRun{
		ID: id, ProfileID: profileID, GatewayID: gatewayID, Status: status,
		ExpectedCount: expected, ReceivedCount: received, PushedCount: pushed,
		ErrorMessage: errMsg, LogLines: logs, CreatedAt: created,
	}
	if started.Valid {
		t := started.Time
		run.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time
		run.FinishedAt = &t
	}
	return run, nil
}

func (s *PortalStore) GetSyncRun(ctx context.Context, runID string) (*domain.SyncRun, error) {
	var row syncRunRow
	err := s.db.GetContext(ctx, &row, `
		SELECT id, profile_id, gateway_id, status, expected_count, received_count,
		       pushed_count, error_message, log_lines, started_at, finished_at, created_at
		FROM sync_runs WHERE id = $1`, runID)
	if err != nil {
		return nil, err
	}
	run, err := decodeSyncRunRow(row.ID, row.ProfileID, row.GatewayID, row.Status,
		row.ExpectedCount, row.ReceivedCount, row.PushedCount, row.ErrorMessage,
		row.LogLines, row.StartedAt, row.FinishedAt, row.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *PortalStore) ListSyncRunsForProfile(ctx context.Context, profileID string, limit int) ([]domain.SyncRun, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []syncRunRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT id, profile_id, gateway_id, status, expected_count, received_count,
		       pushed_count, error_message, log_lines, started_at, finished_at, created_at
		FROM sync_runs WHERE profile_id = $1
		ORDER BY created_at DESC LIMIT $2`, profileID, limit)
	if err != nil {
		return nil, err
	}
	return decodeSyncRuns(rows)
}

func (s *PortalStore) CreateSyncRun(ctx context.Context, profileID, gatewayID string, expected int, initialLog string) (*domain.SyncRun, error) {
	logs := []domain.SyncLogLine{{
		Time: time.Now(), Level: "info", Message: initialLog,
	}}
	raw, _ := json.Marshal(logs)
	var id string
	err := s.db.GetContext(ctx, &id, `
		INSERT INTO sync_runs (profile_id, gateway_id, status, expected_count, log_lines)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		profileID, gatewayID, domain.SyncRunStatusPending, expected, raw)
	if err != nil {
		return nil, err
	}
	return s.GetSyncRun(ctx, id)
}

func (s *PortalStore) StartSyncRun(ctx context.Context, runID, gatewayID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sync_runs SET status = $1, started_at = NOW()
		WHERE id = $2 AND gateway_id = $3 AND status = $4`,
		domain.SyncRunStatusRunning, runID, gatewayID, domain.SyncRunStatusPending)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("run not found or not pending")
	}
	return nil
}

func (s *PortalStore) AppendSyncRunProgress(ctx context.Context, runID, gatewayID string, req domain.SyncRunProgressRequest) error {
	run, err := s.GetSyncRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.GatewayID != gatewayID {
		return fmt.Errorf("forbidden")
	}
	level := "info"
	if req.Phase == "done" {
		level = "success"
	}
	if req.ErrorMessage != "" {
		level = "error"
	}
	msg := req.Message
	if msg == "" && req.ChunkTotal > 0 {
		msg = fmt.Sprintf("%s — %d/%d octets (chunk %d/%d)", req.Phase, req.ReceivedCount, run.ExpectedCount, req.ChunkIndex, req.ChunkTotal)
	}
	if msg != "" {
		run.LogLines = append(run.LogLines, domain.SyncLogLine{
			Time: time.Now(), Level: level, Message: msg,
		})
	}
	status := run.Status
	if req.Status != "" {
		status = req.Status
	} else if run.Status == domain.SyncRunStatusPending {
		status = domain.SyncRunStatusRunning
	}
	received := run.ReceivedCount
	if req.ReceivedCount > 0 {
		received = req.ReceivedCount
	}
	pushed := run.PushedCount
	if req.PushedCount > 0 {
		pushed = req.PushedCount
	}
	var errMsg *string
	if req.ErrorMessage != "" {
		errMsg = &req.ErrorMessage
	}
	finished := false
	if status == domain.SyncRunStatusSuccess || status == domain.SyncRunStatusPartial || status == domain.SyncRunStatusFailed {
		finished = true
	}
	raw, _ := json.Marshal(run.LogLines)
	if finished {
		_, err = s.db.ExecContext(ctx, `
			UPDATE sync_runs SET status=$1, received_count=$2, pushed_count=$3,
				error_message=$4, log_lines=$5, finished_at=NOW(),
				started_at=COALESCE(started_at, NOW())
			WHERE id=$6 AND gateway_id=$7`,
			status, received, pushed, errMsg, raw, runID, gatewayID)
		if err != nil {
			return err
		}
		_, _ = s.db.ExecContext(ctx, `
			UPDATE sync_profiles SET last_run_at = NOW(), updated_at = NOW()
			WHERE id = $1`, run.ProfileID)
		if status == domain.SyncRunStatusFailed || status == domain.SyncRunStatusPartial {
			s.maybeAlertConsecutiveFailures(ctx, run.ProfileID, gatewayID)
		}
		return nil
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE sync_runs SET status=$1, received_count=$2, pushed_count=$3,
			error_message=$4, log_lines=$5,
			started_at=COALESCE(started_at, NOW())
		WHERE id=$6 AND gateway_id=$7`,
		status, received, pushed, errMsg, raw, runID, gatewayID)
	return err
}

func (s *PortalStore) GetSyncProfileByNameForGateway(ctx context.Context, gatewayID, name string) (*domain.SyncProfile, error) {
	var row syncProfileRow
	err := s.db.GetContext(ctx, &row, `
		SELECT p.id, p.gateway_id, p.name, p.index_ranges, p.interval_hours, p.cron_expression,
		       p.pull_from_armoire, p.push_to_cloud, p.enabled, p.exclude_indices, p.last_run_at,
		       p.created_at, p.updated_at,
		       (SELECT r.status FROM sync_runs r WHERE r.profile_id = p.id
		        ORDER BY r.created_at DESC LIMIT 1) AS last_run_status
		FROM sync_profiles p
		WHERE p.name = $1 AND (p.gateway_id = '' OR p.gateway_id = $2)
		ORDER BY CASE WHEN p.gateway_id = $2 THEN 0 ELSE 1 END
		LIMIT 1`, name, gatewayID)
	if err != nil {
		return nil, err
	}
	p, err := decodeProfile(row)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *PortalStore) SetSyncProfileEnabled(ctx context.Context, profileID string, gatewayID string, enabled bool) (*domain.SyncProfile, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sync_profiles SET enabled = $1, updated_at = NOW()
		WHERE id = $2 AND (gateway_id = '' OR gateway_id = $3)`,
		enabled, profileID, gatewayID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("profile not found")
	}
	return s.GetSyncProfile(ctx, profileID)
}
