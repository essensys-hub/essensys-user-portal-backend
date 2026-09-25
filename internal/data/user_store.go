package data

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type UserStore struct {
	db *sqlx.DB
}

func NewUserStore(db *sqlx.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) EnsureTableExists() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255),
		role VARCHAR(50) DEFAULT 'user',
		first_name VARCHAR(100),
		last_name VARCHAR(100),
		provider VARCHAR(50) DEFAULT 'email',
		provider_id VARCHAR(255),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_login TIMESTAMP,
		linked_machine_id INT,
		linked_gateway_id VARCHAR(255),
		linked_armoire_id INT
	);
	ALTER TABLE users ADD COLUMN IF NOT EXISTS linked_machine_id INT;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS linked_gateway_id VARCHAR(255);
	ALTER TABLE users ADD COLUMN IF NOT EXISTS linked_armoire_id INT;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS forbidden_at TIMESTAMPTZ NULL;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS password_change_required_at TIMESTAMPTZ NULL;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS temp_password_expires_at TIMESTAMPTZ NULL;
	ALTER TABLE users ADD COLUMN IF NOT EXISTS temp_password_issued_by INT NULL REFERENCES users(id) ON DELETE SET NULL;
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *UserStore) CreateUser(u *domain.User) error {
	query := `
		INSERT INTO users (email, password_hash, role, first_name, last_name, provider, provider_id, created_at, last_login)
		VALUES (:email, :password_hash, :role, :first_name, :last_name, :provider, :provider_id, :created_at, :last_login)
		RETURNING id`
	rows, err := s.db.NamedQuery(query, u)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Scan(&u.ID)
	}
	return fmt.Errorf("failed to retrieve last insert id")
}

func (s *UserStore) GetUserByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := s.db.Get(&user, `SELECT * FROM users WHERE email = $1`, email)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) GetUserByID(id int) (*domain.User, error) {
	var user domain.User
	err := s.db.Get(&user, `SELECT * FROM users WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) UpdateLastLogin(userID int) error {
	_, err := s.db.Exec(`UPDATE users SET last_login = $1 WHERE id = $2`, time.Now(), userID)
	return err
}

func (s *UserStore) UpdateUser(userID int, firstName, lastName, passwordHash string) error {
	if passwordHash != "" {
		_, err := s.db.Exec(`UPDATE users SET first_name = $1, last_name = $2, password_hash = $3 WHERE id = $4`,
			firstName, lastName, passwordHash, userID)
		return err
	}
	_, err := s.db.Exec(`UPDATE users SET first_name = $1, last_name = $2 WHERE id = $3`, firstName, lastName, userID)
	return err
}

// UpdatePasswordHash touches only the credential. UpdateUser would force the
// caller to re-send first and last name, which risks clobbering a concurrent
// profile edit with stale values.
func (s *UserStore) UpdatePasswordHash(userID int, passwordHash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID)
	return err
}

func (s *UserStore) DeleteUser(userID int) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = $1`, userID)
	return err
}

func (s *UserStore) ForbidUser(userID int) error {
	_, err := s.db.Exec(`UPDATE users SET forbidden_at = $1 WHERE id = $2`, time.Now(), userID)
	return err
}

func (s *UserStore) UnforbidUser(userID int) error {
	_, err := s.db.Exec(`UPDATE users SET forbidden_at = NULL WHERE id = $1`, userID)
	return err
}

func (s *UserStore) CountAdminGlobal() (int, error) {
	var count int
	err := s.db.Get(&count, `SELECT count(*) FROM users WHERE role = $1`, domain.RoleAdminGlobal)
	return count, err
}

func (s *UserStore) UpdateUserLinks(userID int, machineID *int, gatewayID *string, armoireID *int) error {
	_, err := s.db.Exec(`UPDATE users SET linked_machine_id = $1, linked_gateway_id = $2, linked_armoire_id = $3 WHERE id = $4`,
		machineID, gatewayID, armoireID, userID)
	return err
}

// SetTemporaryPassword installs an admin-issued temporary password: it
// overwrites the credential and marks the account as needing a change in the
// same statement, so the two can never be observed out of sync by a
// concurrent read. issuedBy is the admin's user ID, recorded for audit
// traceability (temp_password_issued_by), separate from audit_logs so it
// survives even if the log entry is pruned.
func (s *UserStore) SetTemporaryPassword(userID int, hash string, expiresAt time.Time, issuedBy int) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET password_hash = $1,
		    password_change_required_at = NOW(),
		    temp_password_expires_at = $2,
		    temp_password_issued_by = $3
		WHERE id = $4`,
		hash, expiresAt, issuedBy, userID)
	return err
}

// ClearPasswordChangeRequired installs the user's own new password and lifts
// the forced-change lock in one statement: the account must never be
// observably "changed" but still "required" (or vice versa), since either
// gap would either strand the user behind the lock or leave a window where
// the old temporary password briefly reads as still required.
func (s *UserStore) ClearPasswordChangeRequired(userID int, hash string) error {
	_, err := s.db.Exec(`
		UPDATE users
		SET password_hash = $1,
		    password_change_required_at = NULL,
		    temp_password_expires_at = NULL,
		    temp_password_issued_by = NULL
		WHERE id = $2`,
		hash, userID)
	return err
}

func (s *UserStore) GetAllUsers() ([]*domain.User, error) {
	var users []*domain.User
	err := s.db.Select(&users, `
		SELECT id, email, role, first_name, last_name, provider, created_at, last_login, forbidden_at,
		       linked_machine_id, linked_gateway_id, linked_armoire_id,
		       password_change_required_at, temp_password_expires_at
		FROM users ORDER BY created_at DESC`)
	if users == nil {
		users = []*domain.User{}
	}
	return users, err
}

func (s *UserStore) GetUsersByMachineID(machineID int) ([]*domain.User, error) {
	var users []*domain.User
	err := s.db.Select(&users, `
		SELECT id, email, role, first_name, last_name, provider, created_at, last_login, forbidden_at,
		       linked_machine_id, linked_gateway_id, linked_armoire_id,
		       password_change_required_at, temp_password_expires_at
		FROM users WHERE linked_machine_id = $1 ORDER BY created_at DESC`, machineID)
	if users == nil {
		users = []*domain.User{}
	}
	return users, err
}

func (s *UserStore) UpdateUserRole(userID int, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role = $1 WHERE id = $2`, role, userID)
	return err
}
