package notify

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

// Configured reports whether SMTP env vars are set for sending.
func Configured() bool {
	return os.Getenv("SMTP_HOST") != "" &&
		os.Getenv("SMTP_PORT") != "" &&
		os.Getenv("SMTP_USER") != "" &&
		os.Getenv("SMTP_PASS") != ""
}

// Send delivers an HTML email via SMTP using environment configuration.
func Send(to []string, subject, bodyHTML string) error {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	if host == "" || portStr == "" || user == "" || pass == "" {
		return fmt.Errorf("SMTP configuration missing in environment variables")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %v", err)
	}
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = user
	}
	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", bodyHTML)
	d := gomail.NewDialer(host, port, user, pass)
	return d.DialAndSend(m)
}
