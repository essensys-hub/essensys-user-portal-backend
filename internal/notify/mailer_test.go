package notify

import (
	"os"
	"testing"
)

func TestConfiguredMissingEnv(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_PASS", "")
	if Configured() {
		t.Fatal("expected SMTP not configured")
	}
}

func TestConfiguredAllSet(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USER", "user@example.com")
	t.Setenv("SMTP_PASS", "secret")
	if !Configured() {
		t.Fatal("expected SMTP configured")
	}
}

func TestSendMissingSMTP(t *testing.T) {
	os.Unsetenv("SMTP_HOST")
	os.Unsetenv("SMTP_PORT")
	os.Unsetenv("SMTP_USER")
	os.Unsetenv("SMTP_PASS")
	err := Send([]string{"a@b.c"}, "subj", "<p>hi</p>")
	if err == nil {
		t.Fatal("expected error when SMTP missing")
	}
}
