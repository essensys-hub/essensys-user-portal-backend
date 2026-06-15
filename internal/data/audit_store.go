package data

import (
	"log"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type AuditStore struct {
	db *sqlx.DB
}

func NewAuditStore(db *sqlx.DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) EnsureTableExists() error {
	schema := `
	CREATE TABLE IF NOT EXISTS audit_logs (
		id SERIAL PRIMARY KEY,
		user_id INT NOT NULL,
		username VARCHAR(255) NOT NULL,
		action VARCHAR(50) NOT NULL,
		resource_type VARCHAR(50) NOT NULL,
		resource_id VARCHAR(50) NOT NULL,
		ip_address VARCHAR(50) NOT NULL,
		details TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit_logs(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_user_id ON audit_logs(user_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *AuditStore) CreateAuditLog(l *domain.AuditLog) error {
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	query := `
		INSERT INTO audit_logs (user_id, username, action, resource_type, resource_id, ip_address, details, created_at)
		VALUES (:user_id, :username, :action, :resource_type, :resource_id, :ip_address, :details, :created_at)
	`
	_, err := s.db.NamedExec(query, l)
	if err != nil {
		log.Printf("Error inserting audit log: %v", err)
	}
	return err
}

func (s *AuditStore) GetAuditLogs(filter domain.AuditFilter) ([]*domain.AuditLog, error) {
	if filter.Limit == 0 {
		filter.Limit = 100
	}

	logs := []*domain.AuditLog{}
	var err error

	if filter.MachineID != 0 {
		err = s.db.Select(&logs, `
			SELECT a.*
			FROM audit_logs a
			JOIN users u ON a.user_id = u.id
			WHERE u.linked_machine_id = $1
			ORDER BY a.created_at DESC
			LIMIT $2 OFFSET $3`, filter.MachineID, filter.Limit, filter.Offset)
	} else if filter.UserID != 0 {
		err = s.db.Select(&logs, `
			SELECT * FROM audit_logs
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`, filter.UserID, filter.Limit, filter.Offset)
	} else {
		err = s.db.Select(&logs, `
			SELECT * FROM audit_logs
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2`, filter.Limit, filter.Offset)
	}
	return logs, err
}
