package identity

import "testing"

func TestIsAdminEmail(t *testing.T) {
	list := " admin@essensys.fr , other@test.com "
	if !isAdminEmail("admin@essensys.fr", list) {
		t.Fatal("expected admin match")
	}
	if isAdminEmail("nobody@test.com", list) {
		t.Fatal("expected no match")
	}
}
