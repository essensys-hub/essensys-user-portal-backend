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

func (s *UserStore) DeleteUser(userID int) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = $1`, userID)
	return err
}

func (s *UserStore) UpdateUserLinks(userID int, machineID *int, gatewayID *string, armoireID *int) error {
	_, err := s.db.Exec(`UPDATE users SET linked_machine_id = $1, linked_gateway_id = $2, linked_armoire_id = $3 WHERE id = $4`,
		machineID, gatewayID, armoireID, userID)
	return err
}

func (s *UserStore) GetAllUsers() ([]*domain.User, error) {
	var users []*domain.User
	err := s.db.Select(&users, `
		SELECT id, email, role, first_name, last_name, provider, created_at, last_login,
		       linked_machine_id, linked_gateway_id, linked_armoire_id
		FROM users ORDER BY created_at DESC`)
	if users == nil {
		users = []*domain.User{}
	}
	return users, err
}

func (s *UserStore) GetUsersByMachineID(machineID int) ([]*domain.User, error) {
	var users []*domain.User
	err := s.db.Select(&users, `
		SELECT id, email, role, first_name, last_name, provider, created_at, last_login,
		       linked_machine_id, linked_gateway_id, linked_armoire_id
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
