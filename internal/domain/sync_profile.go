package domain

import "time"

const (
	SyncRunStatusPending = "pending"
	SyncRunStatusRunning = "running"
	SyncRunStatusSuccess = "success"
	SyncRunStatusPartial = "partial"
	SyncRunStatusFailed  = "failed"
)

// IndexRange is an inclusive [start, end] pair in the exchange table.
type IndexRange [2]int

type SyncProfile struct {
	ID              string       `json:"id" db:"id"`
	GatewayID       string       `json:"gateway_id" db:"gateway_id"`
	Name            string       `json:"name" db:"name"`
	IndexRanges     []IndexRange `json:"index_ranges" db:"index_ranges"`
	IntervalHours   int          `json:"interval_hours" db:"interval_hours"`
	CronExpression  *string      `json:"cron_expression,omitempty" db:"cron_expression"`
	PullFromArmoire bool         `json:"pull_from_armoire" db:"pull_from_armoire"`
	PushToCloud     bool         `json:"push_to_cloud" db:"push_to_cloud"`
	Enabled         bool         `json:"enabled" db:"enabled"`
	ExcludeIndices  []int        `json:"exclude_indices" db:"exclude_indices"`
	LastRunAt       *time.Time   `json:"last_run_at,omitempty" db:"last_run_at"`
	CreatedAt       time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at" db:"updated_at"`
	NextRunAt       *time.Time   `json:"next_run_at,omitempty" db:"-"`
	LastRunStatus   *string      `json:"last_run_status,omitempty" db:"-"`
}

type SyncLogLine struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type SyncRun struct {
	ID            string        `json:"id" db:"id"`
	ProfileID     string        `json:"profile_id" db:"profile_id"`
	GatewayID     string        `json:"gateway_id" db:"gateway_id"`
	Status        string        `json:"status" db:"status"`
	ExpectedCount int           `json:"expected_count" db:"expected_count"`
	ReceivedCount int           `json:"received_count" db:"received_count"`
	PushedCount   int           `json:"pushed_count" db:"pushed_count"`
	ErrorMessage  *string       `json:"error_message,omitempty" db:"error_message"`
	LogLines      []SyncLogLine `json:"log_lines" db:"log_lines"`
	StartedAt     *time.Time    `json:"started_at,omitempty" db:"started_at"`
	FinishedAt    *time.Time    `json:"finished_at,omitempty" db:"finished_at"`
	CreatedAt     time.Time     `json:"created_at" db:"created_at"`
}

type SyncConfigResponse struct {
	Profiles     []SyncProfile `json:"profiles"`
	PendingRuns  []SyncRun     `json:"pending_runs"`
}

type UpsertSyncProfileRequest struct {
	GatewayID       string       `json:"gateway_id"`
	Name            string       `json:"name"`
	IndexRanges     []IndexRange `json:"index_ranges"`
	IntervalHours   int          `json:"interval_hours"`
	CronExpression  *string      `json:"cron_expression"`
	PullFromArmoire *bool        `json:"pull_from_armoire"`
	PushToCloud     *bool        `json:"push_to_cloud"`
	Enabled         *bool        `json:"enabled"`
	ExcludeIndices  []int        `json:"exclude_indices"`
}

type SyncRunProgressRequest struct {
	ReceivedCount int    `json:"received_count"`
	PushedCount   int    `json:"pushed_count"`
	ChunkIndex    int    `json:"chunk_index"`
	ChunkTotal    int    `json:"chunk_total"`
	Phase         string `json:"phase"`
	Message       string `json:"message"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"error_message"`
}
