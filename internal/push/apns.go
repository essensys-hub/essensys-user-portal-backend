package push

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	apnsProduction  = "https://api.push.apple.com"
	apnsDevelopment = "https://api.sandbox.push.apple.com"
	// Apple accepts a provider token between 20 and 60 minutes old.
	apnsTokenTTL = 50 * time.Minute
)

// APNsConfig holds the credentials of the Apple key (.p8).
type APNsConfig struct {
	KeyP8       string // PEM content of the .p8 file
	KeyID       string
	TeamID      string
	BundleID    string
	Environment string // "production" (default) or "development"
}

// APNsSender sends through APNs over HTTP/2 with a JWT (ES256) provider token.
type APNsSender struct {
	cfg     APNsConfig
	key     *ecdsa.PrivateKey
	baseURL string
	client  *http.Client
	now     func() time.Time

	mu       sync.Mutex
	jwt      string
	jwtIssue time.Time
}

// NewAPNsSender validates the configuration and parses the key.
func NewAPNsSender(cfg APNsConfig) (*APNsSender, error) {
	if cfg.KeyP8 == "" || cfg.KeyID == "" || cfg.TeamID == "" || cfg.BundleID == "" {
		return nil, fmt.Errorf("apns: incomplete configuration")
	}
	block, _ := pem.Decode([]byte(strings.ReplaceAll(cfg.KeyP8, `\n`, "\n")))
	if block == nil {
		return nil, fmt.Errorf("apns: key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns: invalid key")
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("apns: key is not an EC key")
	}
	base := apnsProduction
	switch strings.ToLower(strings.TrimSpace(cfg.Environment)) {
	case "", "production":
	case "development":
		base = apnsDevelopment
	default:
		return nil, fmt.Errorf("apns: unknown environment")
	}
	return &APNsSender{
		cfg: cfg, key: key, baseURL: base,
		client: &http.Client{Timeout: 10 * time.Second},
		now:    time.Now,
	}, nil
}

// WithEndpoint points the sender at another server (tests).
func (a *APNsSender) WithEndpoint(baseURL string, client *http.Client) *APNsSender {
	a.baseURL = strings.TrimRight(baseURL, "/")
	if client != nil {
		a.client = client
	}
	return a
}

func (a *APNsSender) providerToken() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	if a.jwt != "" && now.Sub(a.jwtIssue) < apnsTokenTTL {
		return a.jwt, nil
	}
	t := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{"iss": a.cfg.TeamID, "iat": now.Unix()})
	t.Header["kid"] = a.cfg.KeyID
	signed, err := t.SignedString(a.key)
	if err != nil {
		return "", fmt.Errorf("apns: signing failed")
	}
	a.jwt, a.jwtIssue = signed, now
	return signed, nil
}

type apnsPayload struct {
	APS    apnsAPS `json:"aps"`
	Type   string  `json:"type"`
	RingID string  `json:"ring_id"`
	At     string  `json:"at"`
}

type apnsAPS struct {
	Alert apnsAlert `json:"alert"`
	Sound string    `json:"sound"`
}

type apnsAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Send delivers one notification. ErrUnregistered on HTTP 410.
func (a *APNsSender) Send(ctx context.Context, token string, msg Message) error {
	bearer, err := a.providerToken()
	if err != nil {
		return err
	}
	body, err := json.Marshal(apnsPayload{
		APS:  apnsAPS{Alert: apnsAlert{Title: msg.Title, Body: msg.Body}, Sound: "default"},
		Type: TypeDoorbell, RingID: msg.RingID, At: msg.At,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/3/device/"+token, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("apns: request")
	}
	req.Header.Set("authorization", "bearer "+bearer)
	req.Header.Set("apns-topic", a.cfg.BundleID)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	if id := msg.RingID; id != "" && len(id) <= 64 { // Apple limit
		req.Header.Set("apns-collapse-id", id)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		// The URL of a failed request holds the token: never wrap err.
		return fmt.Errorf("apns: transport error")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	switch {
	case resp.StatusCode/100 == 2:
		return nil
	case resp.StatusCode == http.StatusGone:
		return ErrUnregistered
	}
	return fmt.Errorf("apns: http %d", resp.StatusCode)
}
