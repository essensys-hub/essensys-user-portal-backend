package push

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// newOAuthClient returns an HTTP client that adds (and refreshes) the OAuth2
// bearer token of the service account.
func newOAuthClient(ctx context.Context, creds *google.Credentials) *http.Client {
	base := &http.Client{Timeout: 10 * time.Second}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, base)
	c := oauth2.NewClient(ctx, creds.TokenSource)
	c.Timeout = 10 * time.Second
	return c
}
