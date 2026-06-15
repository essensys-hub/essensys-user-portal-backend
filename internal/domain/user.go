package domain

import "time"

const (
	RoleAdminGlobal = "admin_global"
	RoleAdminLocal  = "admin_local"
	RoleUser        = "user"
	RoleGuestLocal  = "guest_local"
	RoleSupport     = "support"

	ProviderEmail  = "email"
	ProviderGoogle = "google"
	ProviderApple  = "apple"
)

type User struct {
	ID              int       `db:"id" json:"id"`
	Email           string    `db:"email" json:"email"`
	PasswordHash    string    `db:"password_hash" json:"-"`
	Role            string    `db:"role" json:"role"`
	FirstName       string    `db:"first_name" json:"first_name"`
	LastName        string    `db:"last_name" json:"last_name"`
	Provider        string    `db:"provider" json:"provider"`
	ProviderID      string    `db:"provider_id" json:"-"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	LastLogin       time.Time `db:"last_login" json:"last_login"`
	LinkedMachineID *int      `db:"linked_machine_id" json:"linked_machine_id"`
	LinkedGatewayID *string   `db:"linked_gateway_id" json:"linked_gateway_id"`
	LinkedArmoireID *int      `db:"linked_armoire_id" json:"linked_armoire_id"`
}

type RegisterRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID              int     `json:"id"`
	Email           string  `json:"email"`
	Role            string  `json:"role"`
	FirstName       string  `json:"first_name"`
	LastName        string  `json:"last_name"`
	Provider        string  `json:"provider"`
	LinkedMachineID *int    `json:"linked_machine_id"`
	LinkedGatewayID *string `json:"linked_gateway_id"`
	LinkedArmoireID *int    `json:"linked_armoire_id"`
}

func UserToResponse(u *User) UserResponse {
	if u == nil {
		return UserResponse{}
	}
	return UserResponse{
		ID:              u.ID,
		Email:           u.Email,
		Role:            u.Role,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		Provider:        u.Provider,
		LinkedMachineID: u.LinkedMachineID,
		LinkedGatewayID: u.LinkedGatewayID,
		LinkedArmoireID: u.LinkedArmoireID,
	}
}
