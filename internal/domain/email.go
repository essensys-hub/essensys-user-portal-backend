package domain

import "time"

const (
	EmailSlugUserWelcome       = "user_welcome"
	EmailSlugDeviceAllocation  = "device_allocation"
	EmailSlugPasswordReset     = "password_reset"
	EmailSlugRoleUpdated       = "role_updated"
)

type EmailTemplate struct {
	Slug      string    `db:"slug" json:"slug"`
	Name      string    `db:"name" json:"name"`
	Subject   string    `db:"subject" json:"subject"`
	BodyHTML  string    `db:"body_html" json:"body_html"`
	BodyText  string    `db:"body_text" json:"body_text"`
	Enabled   bool      `db:"enabled" json:"enabled"`
	AutoSend  bool      `db:"auto_send" json:"auto_send"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type EmailSendLog struct {
	ID           int       `db:"id" json:"id"`
	Recipient    string    `db:"recipient" json:"recipient"`
	TemplateSlug string    `db:"template_slug" json:"template_slug"`
	Status       string    `db:"status" json:"status"`
	ErrorMessage string    `db:"error_message" json:"error_message,omitempty"`
	AdminID      *int      `db:"admin_id" json:"admin_id,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}
