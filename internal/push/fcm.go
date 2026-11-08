package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

const (
	fcmScope    = "https://www.googleapis.com/auth/firebase.messaging"
	fcmEndpoint = "https://fcm.googleapis.com"
)

// FCMSender sends through the FCM HTTP v1 API.
type FCMSender struct {
	projectID string
	baseURL   string
	client    *http.Client
}

// NewFCMSender authenticates with a service account (the JSON content, from an
// environment variable). projectID may be empty: the one in the JSON is used.
func NewFCMSender(ctx context.Context, serviceAccountJSON, projectID string) (*FCMSender, error) {
	creds, err := google.CredentialsFromJSON(ctx, []byte(serviceAccountJSON), fcmScope)
	if err != nil {
		// The library error never quotes the key, but keep ours generic anyway.
		return nil, fmt.Errorf("fcm: invalid service account")
	}
	if strings.TrimSpace(projectID) == "" {
		projectID = creds.ProjectID
	}
	if projectID == "" {
		return nil, fmt.Errorf("fcm: project id missing")
	}
	return &FCMSender{
		projectID: projectID,
		baseURL:   fcmEndpoint,
		client:    newOAuthClient(ctx, creds),
	}, nil
}

// NewFCMSenderWithClient is for tests and custom transports: no OAuth2 here,
// baseURL replaces the Google endpoint.
func NewFCMSenderWithClient(baseURL, projectID string, client *http.Client) *FCMSender {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &FCMSender{projectID: projectID, baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

type fcmRequest struct {
	Message fcmMessage `json:"message"`
}

type fcmMessage struct {
	Token        string            `json:"token"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data"`
	Android      fcmAndroid        `json:"android"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroid struct {
	Priority     string `json:"priority"`
	Notification struct {
		ChannelID string `json:"channel_id"`
		// Tag replaces a notification already shown with the same tag, so two
		// deliveries of one ring collapse into one on the device.
		Tag string `json:"tag,omitempty"`
	} `json:"notification"`
}

// Send delivers one notification. ErrUnregistered when FCM says the token is dead.
func (f *FCMSender) Send(ctx context.Context, token string, msg Message) error {
	m := fcmMessage{
		Token:        token,
		Notification: fcmNotification{Title: msg.Title, Body: msg.Body},
		Data:         map[string]string{"type": TypeDoorbell, "ring_id": msg.RingID, "at": msg.At},
		Android:      fcmAndroid{Priority: "HIGH"},
	}
	m.Android.Notification.ChannelID = AndroidChannel
	m.Android.Notification.Tag = msg.RingID
	body, err := json.Marshal(fcmRequest{Message: m})
	if err != nil {
		return err
	}
	url := f.baseURL + "/v1/projects/" + f.projectID + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("fcm: request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		// net/url errors quote the URL, which holds no token; no body is in them.
		return fmt.Errorf("fcm: transport error")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode/100 == 2 {
		return nil
	}
	if fcmUnregistered(resp.StatusCode, raw) {
		return ErrUnregistered
	}
	return fmt.Errorf("fcm: http %d", resp.StatusCode)
}

// fcmUnregistered recognises the permanent refusal: a 404 whose details carry
// the errorCode UNREGISTERED (or status NOT_FOUND on that 404).
func fcmUnregistered(status int, raw []byte) bool {
	var e struct {
		Error struct {
			Status  string `json:"status"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &e)
	for _, d := range e.Error.Details {
		if d.ErrorCode == "UNREGISTERED" {
			return true
		}
	}
	return status == http.StatusNotFound && (e.Error.Status == "NOT_FOUND" || e.Error.Status == "UNREGISTERED")
}
