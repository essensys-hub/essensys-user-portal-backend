package domain

import "time"

type AuditLog struct {
	ID           int       `db:"id" json:"id"`
	UserID       int       `db:"user_id" json:"user_id"`
	Username     string    `db:"username" json:"username"`
	Action       string    `db:"action" json:"action"`
	ResourceType string    `db:"resource_type" json:"resource_type"`
	ResourceID   string    `db:"resource_id" json:"resource_id"`
	IPAddress    string    `db:"ip_address" json:"ip_address"`
	Details      string    `db:"details" json:"details"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

type AuditFilter struct {
	Limit        int
	Offset       int
	MachineID    int
	UserID       int
	ResourceType string
	Action       string
}
