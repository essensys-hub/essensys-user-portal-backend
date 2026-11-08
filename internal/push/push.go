// Package push sends mobile notifications (feature
// sonnette-plages-push-2026-11-008, #61). Two providers sit behind one
// interface: FCM HTTP v1 for Android and APNs for iPhone. A device token is
// a secret of the user: it is never logged nor included in an error.
package push

import (
	"context"
	"errors"
	"log"
	"sync"
)

const (
	PlatformAndroid = "android"
	PlatformIOS     = "ios"

	DoorbellTitle = "Quelqu'un sonne"
	DoorbellBody  = "Sonnette de votre maison"
	TypeDoorbell  = "doorbell"
	// AndroidChannel is the notification channel created by the Android app.
	AndroidChannel = "doorbell"
)

// ErrUnregistered : the provider refused the token for good (FCM UNREGISTERED,
// APNs 410). The device must be deleted.
var ErrUnregistered = errors.New("push: token unregistered")

// Message is the content of one doorbell notification.
type Message struct {
	Title  string
	Body   string
	RingID string
	At     string // RFC 3339, UTC
}

// DoorbellMessage builds the doorbell notification (no cabinet name, no image).
func DoorbellMessage(ringID, at string) Message {
	return Message{Title: DoorbellTitle, Body: DoorbellBody, RingID: ringID, At: at}
}

// PushSender delivers a message to one device token.
type PushSender interface {
	Send(ctx context.Context, token string, msg Message) error
}

// Dispatcher routes a message to the sender of the device platform. A nil
// sender means the provider is not configured.
type Dispatcher struct {
	Android PushSender
	IOS     PushSender

	unavailable sync.Once
}

// For returns the sender of a platform, or nil if it is not configured. The
// first call that finds a missing provider logs push_unavailable, once.
func (d *Dispatcher) For(platform string) PushSender {
	if d == nil {
		return nil
	}
	var s PushSender
	switch platform {
	case PlatformAndroid:
		s = d.Android
	case PlatformIOS:
		s = d.IOS
	}
	if s == nil {
		d.unavailable.Do(func() {
			log.Printf("[push] push_unavailable: provider for %q not configured, notifications are not sent", platform)
		})
	}
	return s
}
