package support

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// fakeGitHub is an httptest GitHub that checks the App JWT and the
// installation token, and records created issues.
type fakeGitHub struct {
	t          *testing.T
	key        *rsa.PrivateKey
	tokenCalls atomic.Int32
	created    []map[string]any
	rejectType bool
	fail       int
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/app/installations/99/access_tokens":
		tok, err := jwt.Parse(auth, func(*jwt.Token) (any, error) { return &f.key.PublicKey, nil })
		if err != nil || !tok.Valid || tok.Method.Alg() != "RS256" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if iss, _ := tok.Claims.(jwt.MapClaims)["iss"].(string); iss != "12345" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.tokenCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_install", "expires_at": time.Now().Add(time.Hour)})
	case auth != "ghs_install":
		w.WriteHeader(http.StatusUnauthorized)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/issues"):
		if f.fail > 0 {
			f.fail--
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"message":"Server Error"}`))
			return
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if f.rejectType && in["type"] != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"Validation Failed"}`))
			return
		}
		in["_path"] = r.URL.Path
		f.created = append(f.created, in)
		n := len(f.created)
		_ = json.NewEncoder(w).Encode(Issue{Number: n, NodeID: "I_node", HTMLURL: fmt.Sprintf("https://github.com%s/%d", r.URL.Path, n), State: "open"})
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/"):
		_ = json.NewEncoder(w).Encode(Issue{Number: 1, State: "closed", StateReason: "completed"})
	case r.URL.Path == "/graphql":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newFakeGitHub(t *testing.T) (*fakeGitHub, *AppClient) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGitHub{t: t, key: key}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, NewAppClient("12345", "99", key, srv.URL)
}

func TestAppClient_CreateIssueWithInstallationToken(t *testing.T) {
	f, c := newFakeGitHub(t)
	iss, err := c.CreateIssue(context.Background(), "essensys-hub/essensys-support-site", IssueInput{Title: "[Bug] x", Body: "b", Labels: []string{"bug"}, Type: "Bug"})
	if err != nil || iss.Number != 1 {
		t.Fatalf("iss=%+v err=%v", iss, err)
	}
	if f.created[0]["_path"] != "/repos/essensys-hub/essensys-support-site/issues" || f.created[0]["type"] != "Bug" {
		t.Fatalf("unexpected request %+v", f.created[0])
	}
}

func TestAppClient_CachesInstallationToken(t *testing.T) {
	f, c := newFakeGitHub(t)
	for i := 0; i < 3; i++ {
		if _, err := c.GetIssue(context.Background(), "o/r", 1); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.tokenCalls.Load(); n != 1 {
		t.Fatalf("token minted %d times, want 1", n)
	}
}

func TestAppClient_RetriesWithoutUnknownIssueType(t *testing.T) {
	f, c := newFakeGitHub(t)
	f.rejectType = true
	if _, err := c.CreateIssue(context.Background(), "o/r", IssueInput{Title: "t", Type: "Support"}); err != nil {
		t.Fatal(err)
	}
	if _, has := f.created[0]["type"]; has {
		t.Fatal("type should have been dropped")
	}
}

func TestAppClient_ServerErrorIsAPIError(t *testing.T) {
	f, c := newFakeGitHub(t)
	f.fail = 1
	_, err := c.CreateIssue(context.Background(), "o/r", IssueInput{Title: "t"})
	e, ok := err.(*APIError)
	if !ok || e.Status != http.StatusBadGateway {
		t.Fatalf("err=%v", err)
	}
}

func TestAppClient_WrongKeyIsRejected(t *testing.T) {
	_, c := newFakeGitHub(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	c.Key = other
	if _, err := c.GetIssue(context.Background(), "o/r", 1); err == nil {
		t.Fatal("expected 401 with a foreign key")
	}
}
