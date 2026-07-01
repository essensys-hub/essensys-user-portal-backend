package domain

import "time"

type LinkRequest struct {
	ID            int        `db:"id" json:"id"`
	UserID        int        `db:"user_id" json:"user_id"`
	MachineSerial string     `db:"machine_serial" json:"machine_serial"`
	Message       *string    `db:"message" json:"message,omitempty"`
	Status        string     `db:"status" json:"status"`
	ReviewedBy    *string    `db:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time `db:"reviewed_at" json:"reviewed_at,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
}

// LinkRequestAdminView enriches LinkRequest for the admin UI (user identity).
type LinkRequestAdminView struct {
	LinkRequest
	UserEmail string `db:"user_email" json:"user_email"`
	FirstName string `db:"first_name" json:"first_name,omitempty"`
	LastName  string `db:"last_name" json:"last_name,omitempty"`
}

type CloudAction struct {
	GUID      string      `db:"guid" json:"guid"`
	UserID    int         `db:"user_id" json:"user_id"`
	MachineID *int        `db:"machine_id" json:"machine_id,omitempty"`
	Params    []ExchangeKV `json:"params"`
	Status    string      `db:"status" json:"status"`
	CreatedAt time.Time   `db:"created_at" json:"created_at"`
}

type GatewaySession struct {
	GatewayID string     `db:"gateway_id" json:"gateway_id"`
	TokenHash string     `db:"token_hash" json:"-"`
	MachineID *int       `db:"machine_id" json:"machine_id,omitempty"`
	Eth0MAC   *string    `db:"eth0_mac" json:"eth0_mac,omitempty"`
	Eth1MAC   *string    `db:"eth1_mac" json:"eth1_mac,omitempty"`
	LastSeen  *time.Time `db:"last_seen" json:"last_seen,omitempty"`
}

type UserProfile struct {
	ID              int     `db:"id" json:"id"`
	Email           string  `db:"email" json:"email"`
	Role            string  `db:"role" json:"role"`
	FirstName       string  `db:"first_name" json:"first_name"`
	LastName        string  `db:"last_name" json:"last_name"`
	LinkedMachineID *int    `db:"linked_machine_id" json:"linked_machine_id"`
	LinkedGatewayID *string `db:"linked_gateway_id" json:"linked_gateway_id"`
	LinkedArmoireID *int    `db:"linked_armoire_id" json:"linked_armoire_id"`
}

const (
	LinkStatusPending  = "pending"
	LinkStatusApproved = "approved"
	LinkStatusRejected = "rejected"

	ActionStatusPending   = "pending"
	ActionStatusDelivered = "delivered"
	ActionStatusDone      = "done"
)
