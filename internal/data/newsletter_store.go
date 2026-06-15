package data

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type NewsletterStore struct {
	db *sqlx.DB
}

func NewNewsletterStore(db *sqlx.DB) *NewsletterStore {
	return &NewsletterStore{db: db}
}

func (s *NewsletterStore) EnsureTablesExist() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS newsletter_subscribers (
			email TEXT PRIMARY KEY,
			date_joined TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS newsletters (
			id TEXT PRIMARY KEY,
			subject TEXT NOT NULL DEFAULT 'Nouvelle Newsletter',
			content TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			sent_at TIMESTAMPTZ
		);
	`)
	return err
}

func (s *NewsletterStore) AddSubscriber(email string) error {
	_, err := s.db.Exec(`
		INSERT INTO newsletter_subscribers (email, date_joined)
		VALUES ($1, $2)
		ON CONFLICT (email) DO NOTHING`, email, time.Now())
	return err
}

func (s *NewsletterStore) DeleteSubscriber(email string) error {
	res, err := s.db.Exec(`DELETE FROM newsletter_subscribers WHERE email = $1`, email)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("subscriber not found")
	}
	return nil
}

func (s *NewsletterStore) GetSubscribers() ([]domain.Subscriber, error) {
	var subs []domain.Subscriber
	err := s.db.Select(&subs, `SELECT email, date_joined FROM newsletter_subscribers ORDER BY date_joined DESC`)
	if subs == nil {
		subs = []domain.Subscriber{}
	}
	return subs, err
}

func (s *NewsletterStore) GetNewsletters() ([]domain.Newsletter, error) {
	var list []domain.Newsletter
	err := s.db.Select(&list, `SELECT id, subject, content, status, version, created_at, updated_at, sent_at FROM newsletters ORDER BY updated_at DESC`)
	if list == nil {
		list = []domain.Newsletter{}
	}
	return list, err
}

func (s *NewsletterStore) GetNewsletter(id string) (*domain.Newsletter, error) {
	var n domain.Newsletter
	err := s.db.Get(&n, `SELECT id, subject, content, status, version, created_at, updated_at, sent_at FROM newsletters WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("newsletter not found")
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *NewsletterStore) SaveNewsletter(n domain.Newsletter) error {
	_, err := s.db.Exec(`
		INSERT INTO newsletters (id, subject, content, status, version, created_at, updated_at, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			subject = EXCLUDED.subject,
			content = EXCLUDED.content,
			status = EXCLUDED.status,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at,
			sent_at = EXCLUDED.sent_at`,
		n.ID, n.Subject, n.Content, n.Status, n.Version, n.CreatedAt, n.UpdatedAt, n.SentAt)
	return err
}

func (s *NewsletterStore) DeleteNewsletter(id string) error {
	res, err := s.db.Exec(`DELETE FROM newsletters WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("newsletter not found")
	}
	return nil
}
