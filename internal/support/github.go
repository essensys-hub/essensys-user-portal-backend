package support

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Issue is the part of a GitHub issue the portal keeps track of.
type Issue struct {
	Number      int    `json:"number"`
	NodeID      string `json:"node_id"`
	HTMLURL     string `json:"html_url"`
	State       string `json:"state"`
	StateReason string `json:"state_reason"`
}

// IssueInput is what the portal sends to create an issue.
type IssueInput struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
	Type   string   `json:"type,omitempty"`
}

// GitHub is the subset of the GitHub API the support module needs.
type GitHub interface {
	CreateIssue(ctx context.Context, repo string, in IssueInput) (Issue, error)
	AddToProject(ctx context.Context, projectID, contentNodeID string) error
	GetIssue(ctx context.Context, repo string, number int) (Issue, error)
}

// APIError is a non-2xx GitHub response. Message is GitHub's, never the
// report content, so it is safe to store in last_error.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("github: %d %s", e.Status, e.Message) }

// AppClient authenticates as the Essensys Support GitHub App (D7): an RS256
// app JWT is exchanged for an installation token, cached until 5 minutes
// before it expires.
type AppClient struct {
	AppID          string
	InstallationID string
	Key            *rsa.PrivateKey
	BaseURL        string
	HTTP           *http.Client
	Now            func() time.Time

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewAppClient(appID, installationID string, key *rsa.PrivateKey, baseURL string) *AppClient {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &AppClient{AppID: appID, InstallationID: installationID, Key: key,
		BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 10 * time.Second}, Now: time.Now}
}

// ParsePrivateKey reads the App's PEM private key.
func ParsePrivateKey(pem []byte) (*rsa.PrivateKey, error) {
	return jwt.ParseRSAPrivateKeyFromPEM(pem)
}

func (c *AppClient) appJWT() (string, error) {
	now := c.Now()
	return jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    c.AppID,
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}).SignedString(c.Key)
}

func (c *AppClient) installationToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.Now().Before(c.tokenExp.Add(-5*time.Minute)) {
		return c.token, nil
	}
	signed, err := c.appJWT()
	if err != nil {
		return "", err
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := c.do(ctx, http.MethodPost, "/app/installations/"+c.InstallationID+"/access_tokens", signed, nil, &out); err != nil {
		return "", err
	}
	c.token, c.tokenExp = out.Token, out.ExpiresAt
	return c.token, nil
}

func (c *AppClient) authed(ctx context.Context, method, path string, in, out any) error {
	tok, err := c.installationToken(ctx)
	if err != nil {
		return err
	}
	return c.do(ctx, method, path, tok, in, out)
}

func (c *AppClient) do(ctx context.Context, method, path, bearer string, in, out any) error {
	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &e)
		return &APIError{Status: resp.StatusCode, Message: e.Message}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// CreateIssue opens an issue. When the org has no issue type of that name,
// GitHub answers 422: the issue is then created without a type rather than
// left pending forever.
func (c *AppClient) CreateIssue(ctx context.Context, repo string, in IssueInput) (Issue, error) {
	var out Issue
	err := c.authed(ctx, http.MethodPost, "/repos/"+repo+"/issues", in, &out)
	if e, ok := err.(*APIError); ok && e.Status == http.StatusUnprocessableEntity && in.Type != "" {
		in.Type = ""
		err = c.authed(ctx, http.MethodPost, "/repos/"+repo+"/issues", in, &out)
	}
	return out, err
}

func (c *AppClient) GetIssue(ctx context.Context, repo string, number int) (Issue, error) {
	var out Issue
	err := c.authed(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/issues/%d", repo, number), nil, &out)
	return out, err
}

// AddToProject adds the issue to the Project v2 through GraphQL.
func (c *AppClient) AddToProject(ctx context.Context, projectID, contentNodeID string) error {
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	q := map[string]any{
		"query":     `mutation($p:ID!,$c:ID!){addProjectV2ItemById(input:{projectId:$p,contentId:$c}){item{id}}}`,
		"variables": map[string]string{"p": projectID, "c": contentNodeID},
	}
	if err := c.authed(ctx, http.MethodPost, "/graphql", q, &out); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return &APIError{Status: http.StatusOK, Message: out.Errors[0].Message}
	}
	return nil
}
