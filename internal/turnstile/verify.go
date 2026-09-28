package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultSiteVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Verifier checks a Turnstile token with Cloudflare siteverify.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

// Client calls Cloudflare Turnstile siteverify over HTTPS.
type Client struct {
	Secret     string
	HTTPClient *http.Client
	VerifyURL  string
}

// NewClient builds a siteverify client with a short timeout (fail-closed on hang).
func NewClient(secret string) *Client {
	return &Client{
		Secret: secret,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		VerifyURL: defaultSiteVerifyURL,
	}
}

type siteVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify validates token with Cloudflare. Empty secret, empty token, HTTP errors,
// timeouts, and unsuccessful responses all return an error (fail-closed).
func (c *Client) Verify(ctx context.Context, token, remoteIP string) error {
	if c == nil || strings.TrimSpace(c.Secret) == "" {
		return fmt.Errorf("turnstile secret not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("turnstile token missing")
	}

	form := url.Values{}
	form.Set("secret", c.Secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	verifyURL := c.VerifyURL
	if verifyURL == "" {
		verifyURL = defaultSiteVerifyURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("turnstile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile siteverify: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("turnstile read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile siteverify HTTP %d", resp.StatusCode)
	}

	var parsed siteVerifyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("turnstile decode: %w", err)
	}
	if !parsed.Success {
		if len(parsed.ErrorCodes) > 0 {
			return fmt.Errorf("turnstile rejected: %s", strings.Join(parsed.ErrorCodes, ","))
		}
		return fmt.Errorf("turnstile rejected")
	}
	return nil
}
