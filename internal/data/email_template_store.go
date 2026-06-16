package data

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type EmailTemplateStore struct {
	db *sqlx.DB
}

func NewEmailTemplateStore(db *sqlx.DB) *EmailTemplateStore {
	return &EmailTemplateStore{db: db}
}

func (s *EmailTemplateStore) EnsureTablesExist() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS email_templates (
			slug VARCHAR(64) PRIMARY KEY,
			name VARCHAR(128) NOT NULL,
			subject TEXT NOT NULL,
			body_html TEXT NOT NULL DEFAULT '',
			body_text TEXT NOT NULL DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT false,
			auto_send BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS email_send_log (
			id SERIAL PRIMARY KEY,
			recipient VARCHAR(255) NOT NULL,
			template_slug VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL,
			error_message TEXT,
			admin_id INT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	return err
}

func (s *EmailTemplateStore) List() ([]domain.EmailTemplate, error) {
	var list []domain.EmailTemplate
	err := s.db.Select(&list, `
		SELECT slug, name, subject, body_html, body_text, enabled, auto_send, created_at, updated_at
		FROM email_templates ORDER BY slug`)
	if list == nil {
		list = []domain.EmailTemplate{}
	}
	return list, err
}

func (s *EmailTemplateStore) Get(slug string) (*domain.EmailTemplate, error) {
	var t domain.EmailTemplate
	err := s.db.Get(&t, `
		SELECT slug, name, subject, body_html, body_text, enabled, auto_send, created_at, updated_at
		FROM email_templates WHERE slug = $1`, slug)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found")
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *EmailTemplateStore) Upsert(t *domain.EmailTemplate) error {
	now := time.Now()
	t.UpdatedAt = now
	_, err := s.db.Exec(`
		INSERT INTO email_templates (slug, name, subject, body_html, body_text, enabled, auto_send, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (slug) DO UPDATE SET
			name = EXCLUDED.name,
			subject = EXCLUDED.subject,
			body_html = EXCLUDED.body_html,
			body_text = EXCLUDED.body_text,
			enabled = EXCLUDED.enabled,
			auto_send = EXCLUDED.auto_send,
			updated_at = EXCLUDED.updated_at`,
		t.Slug, t.Name, t.Subject, t.BodyHTML, t.BodyText, t.Enabled, t.AutoSend, now, now)
	return err
}

func (s *EmailTemplateStore) LogSend(recipient, slug, status, errMsg string, adminID *int) error {
	var errVal interface{}
	if errMsg != "" {
		errVal = errMsg
	}
	_, err := s.db.Exec(`
		INSERT INTO email_send_log (recipient, template_slug, status, error_message, admin_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		recipient, slug, status, errVal, adminID, time.Now())
	return err
}
