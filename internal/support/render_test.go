package support

import (
	"strings"
	"testing"
)

func TestUserPseudonym_StableAndDistinct(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	a1, a2, b := UserPseudonym(key, 7), UserPseudonym(key, 7), UserPseudonym(key, 8)
	if a1 != a2 {
		t.Fatalf("same account, different refs: %s %s", a1, a2)
	}
	if a1 == b {
		t.Fatalf("two accounts share %s", a1)
	}
	if !strings.HasPrefix(a1, "U-") || len(a1) != 10 {
		t.Fatalf("unexpected format %q", a1)
	}
	if UserPseudonym([]byte("another-key-another-key"), 7) == a1 {
		t.Fatal("ref must depend on the server key")
	}
}

func TestNewReportRef_Format(t *testing.T) {
	r, err := NewReportRef()
	if err != nil || !strings.HasPrefix(r, "R-") || len(r) != 10 {
		t.Fatalf("ref=%q err=%v", r, err)
	}
}

func TestIssueBody_NeutralisesMentionsAndHTML(t *testing.T) {
	body := IssueBody(KindBug, Fields{App: "web", Actual: "ping @nrineau <script>x</script> | pipe"}, "U-AAAA1111", "R-BBBB2222")
	if strings.Contains(body, "@nrineau") {
		t.Fatal("mention not neutralised")
	}
	if strings.Contains(body, "<script>") {
		t.Fatal("HTML not escaped")
	}
	if !strings.Contains(body, "U-AAAA1111") || !strings.Contains(body, "R-BBBB2222") {
		t.Fatal("references missing")
	}
}

func TestIssueBody_OnlyFormFields(t *testing.T) {
	body := IssueBody(KindIncident, Fields{Impact: "remote", Mode: "cloud", Actual: "Le portail ne répond plus"}, "U-AAAA1111", "R-BBBB2222")
	for _, want := range []string{"Accès distant / portail cloud", "Cloud (mon.essensys.fr)", "### Ce qui se passe"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in\n%s", want, body)
		}
	}
	if strings.Contains(body, "Application") {
		t.Error("bug-only field rendered for an incident")
	}
}

func TestIssueTitle_Prefix(t *testing.T) {
	if got := IssueTitle(KindIncident, "  Volets bloqués\n"); got != "[Incident] Volets bloqués" {
		t.Fatalf("got %q", got)
	}
	if got := IssueTitle(KindBug, "Crash @x"); got != "[Bug] Crash @​x" {
		t.Fatalf("got %q", got)
	}
}
