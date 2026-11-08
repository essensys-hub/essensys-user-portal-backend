package push

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// sonnette-plages-push-2026-11-008 (#61): FCM and APNs senders against fake
// httptest servers. Nothing here leaves the machine; every key is generated.

const fakeToken = "device-token-for-tests-0123456789"

func TestFCM_formatDeLaRequete(t *testing.T) {
	var gotPath, gotCT string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotCT = r.URL.Path, r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"name":"projects/p/messages/1"}`))
	}))
	defer srv.Close()
	s := NewFCMSenderWithClient(srv.URL, "demo-project", srv.Client())
	if err := s.Send(context.Background(), fakeToken, DoorbellMessage("ring-1", "2026-11-08T10:15:30Z")); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/projects/demo-project/messages:send" || gotCT != "application/json" {
		t.Fatalf("path %q ct %q", gotPath, gotCT)
	}
	m := got["message"].(map[string]any)
	if m["token"] != fakeToken {
		t.Fatalf("token: %v", m["token"])
	}
	n := m["notification"].(map[string]any)
	if n["title"] != "Quelqu'un sonne" || n["body"] != "Sonnette de votre maison" {
		t.Fatalf("notification: %v", n)
	}
	d := m["data"].(map[string]any)
	if d["type"] != "doorbell" || d["ring_id"] != "ring-1" || d["at"] != "2026-11-08T10:15:30Z" {
		t.Fatalf("data: %v", d)
	}
	a := m["android"].(map[string]any)
	if a["priority"] != "HIGH" || a["notification"].(map[string]any)["channel_id"] != "doorbell" {
		t.Fatalf("android: %v", a)
	}
	if a["notification"].(map[string]any)["tag"] != "ring-1" {
		t.Fatalf("android tag must be the ring_id: %v", a)
	}
}

func TestFCM_UNREGISTEREDEtErreurs(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
		any    bool
	}{
		{"unregistered", 404, `{"error":{"code":404,"status":"NOT_FOUND","details":[{"@type":"x","errorCode":"UNREGISTERED"}]}}`, ErrUnregistered, false},
		{"404 sans detail", 404, `{"error":{"status":"NOT_FOUND"}}`, ErrUnregistered, false},
		{"500", 500, `{"error":{"status":"INTERNAL"}}`, nil, true},
		{"invalid argument", 400, `{"error":{"status":"INVALID_ARGUMENT"}}`, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			defer srv.Close()
			err := NewFCMSenderWithClient(srv.URL, "p", srv.Client()).Send(context.Background(), fakeToken, DoorbellMessage("r", "t"))
			if c.any {
				if err == nil || errors.Is(err, ErrUnregistered) {
					t.Fatalf("want a plain error, got %v", err)
				}
			} else if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
			if err != nil && strings.Contains(err.Error(), fakeToken) {
				t.Fatal("the error must not contain the token")
			}
		})
	}
}

func TestFCM_injoignableSansJetonDansLErreur(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	err := NewFCMSenderWithClient(url, "p", &http.Client{Timeout: time.Second}).Send(context.Background(), fakeToken, DoorbellMessage("r", "t"))
	if err == nil || strings.Contains(err.Error(), fakeToken) {
		t.Fatalf("err: %v", err)
	}
}

// The real constructor authenticates with a service account: OAuth2 JWT bearer
// exchange against a fake token endpoint, then the bearer on the FCM call.
func TestFCM_compteDeServiceOAuth2(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))

	tokenCalls := 0
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || r.Form.Get("assertion") == "" {
			t.Errorf("bad token request: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-access-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()
	saJSON, _ := json.Marshal(map[string]string{
		"type": "service_account", "project_id": "from-json", "private_key_id": "k1",
		"private_key": pemKey, "client_email": "sa@example.iam.gserviceaccount.com",
		"token_uri": tokenSrv.URL,
	})

	var auth, path string
	fcmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer fcmSrv.Close()

	s, err := NewFCMSender(context.Background(), string(saJSON), "")
	if err != nil {
		t.Fatal(err)
	}
	s.baseURL = fcmSrv.URL
	if err := s.Send(context.Background(), fakeToken, DoorbellMessage("r", "t")); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer fake-access-token" || path != "/v1/projects/from-json/messages:send" || tokenCalls != 1 {
		t.Fatalf("auth %q path %q tokenCalls %d", auth, path, tokenCalls)
	}
	if s2, err := NewFCMSender(context.Background(), string(saJSON), "override"); err != nil || s2.projectID != "override" {
		t.Fatalf("project id from env must win: %v", err)
	}
	if _, err := NewFCMSender(context.Background(), `{"not":"a key"}`, "p"); err == nil {
		t.Fatal("invalid service account must be rejected")
	}
}

func apnsTestKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestAPNs_formatDeLaRequete(t *testing.T) {
	key, p8 := apnsTestKey(t)
	var gotPath string
	var hdr http.Header
	var got map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, hdr = r.URL.Path, r.Header.Clone()
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()
	s, err := NewAPNsSender(APNsConfig{KeyP8: p8, KeyID: "KEYID12345", TeamID: "TEAMID1234", BundleID: "essensys.essensys-iphone"})
	if err != nil {
		t.Fatal(err)
	}
	s.WithEndpoint(srv.URL, srv.Client())
	if err := s.Send(context.Background(), fakeToken, DoorbellMessage("ring-1", "2026-11-08T10:15:30Z")); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/3/device/"+fakeToken {
		t.Fatalf("path %q", gotPath)
	}
	if hdr.Get("Apns-Topic") != "essensys.essensys-iphone" || hdr.Get("Apns-Push-Type") != "alert" || hdr.Get("Apns-Priority") != "10" {
		t.Fatalf("headers: %v", hdr)
	}
	if hdr.Get("Apns-Collapse-Id") != "ring-1" {
		t.Fatalf("collapse id must be the ring_id: %v", hdr)
	}
	raw := strings.TrimPrefix(hdr.Get("Authorization"), "bearer ")
	tok, err := jwt.Parse(raw, func(tk *jwt.Token) (any, error) {
		if tk.Method.Alg() != "ES256" {
			t.Errorf("alg %s", tk.Method.Alg())
		}
		return &key.PublicKey, nil
	})
	if err != nil || !tok.Valid {
		t.Fatalf("provider token: %v", err)
	}
	claims := tok.Claims.(jwt.MapClaims)
	if tok.Header["kid"] != "KEYID12345" || claims["iss"] != "TEAMID1234" || claims["iat"] == nil {
		t.Fatalf("jwt header %v claims %v", tok.Header, claims)
	}
	aps := got["aps"].(map[string]any)
	alert := aps["alert"].(map[string]any)
	if alert["title"] != "Quelqu'un sonne" || alert["body"] != "Sonnette de votre maison" {
		t.Fatalf("alert: %v", alert)
	}
	if got["type"] != "doorbell" || got["ring_id"] != "ring-1" || got["at"] != "2026-11-08T10:15:30Z" {
		t.Fatalf("payload: %v", got)
	}
}

func TestAPNs_jetonFournisseurMisEnCache(t *testing.T) {
	_, p8 := apnsTestKey(t)
	var auths []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
	}))
	defer srv.Close()
	s, _ := NewAPNsSender(APNsConfig{KeyP8: p8, KeyID: "K", TeamID: "T", BundleID: "b"})
	s.WithEndpoint(srv.URL, srv.Client())
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		_ = s.Send(context.Background(), fakeToken, DoorbellMessage("r", "t"))
	}
	now = now.Add(55 * time.Minute)
	_ = s.Send(context.Background(), fakeToken, DoorbellMessage("r", "t"))
	if auths[0] != auths[1] || auths[1] == auths[2] {
		t.Fatal("the provider token must be reused within 50 minutes and renewed after")
	}
}

func TestAPNs_410EtErreurs(t *testing.T) {
	_, p8 := apnsTestKey(t)
	for _, c := range []struct {
		status int
		want   error
	}{{410, ErrUnregistered}, {500, nil}, {403, nil}} {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(`{"reason":"Unregistered"}`))
		}))
		s, _ := NewAPNsSender(APNsConfig{KeyP8: p8, KeyID: "K", TeamID: "T", BundleID: "b"})
		s.WithEndpoint(srv.URL, srv.Client())
		err := s.Send(context.Background(), fakeToken, DoorbellMessage("r", "t"))
		srv.Close()
		if c.want != nil && !errors.Is(err, c.want) {
			t.Fatalf("%d: got %v", c.status, err)
		}
		if c.want == nil && (err == nil || errors.Is(err, ErrUnregistered)) {
			t.Fatalf("%d: want a plain error, got %v", c.status, err)
		}
		if err != nil && strings.Contains(err.Error(), fakeToken) {
			t.Fatal("the error must not contain the token")
		}
	}
}

func TestAPNs_injoignableSansJetonDansLErreur(t *testing.T) {
	_, p8 := apnsTestKey(t)
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	s, _ := NewAPNsSender(APNsConfig{KeyP8: p8, KeyID: "K", TeamID: "T", BundleID: "b"})
	s.WithEndpoint(url, &http.Client{Timeout: time.Second})
	err := s.Send(context.Background(), fakeToken, DoorbellMessage("r", "t"))
	if err == nil || strings.Contains(err.Error(), fakeToken) {
		t.Fatalf("err: %v", err)
	}
}

func TestAPNs_environnementEtConfigurationIncomplete(t *testing.T) {
	_, p8 := apnsTestKey(t)
	dev, err := NewAPNsSender(APNsConfig{KeyP8: p8, KeyID: "K", TeamID: "T", BundleID: "b", Environment: "development"})
	if err != nil || dev.baseURL != apnsDevelopment {
		t.Fatalf("development: %v %v", dev, err)
	}
	prod, _ := NewAPNsSender(APNsConfig{KeyP8: p8, KeyID: "K", TeamID: "T", BundleID: "b", Environment: "production"})
	if prod.baseURL != apnsProduction {
		t.Fatal("production endpoint")
	}
	for _, bad := range []APNsConfig{
		{KeyID: "K", TeamID: "T", BundleID: "b"},
		{KeyP8: p8, TeamID: "T", BundleID: "b"},
		{KeyP8: p8, KeyID: "K", BundleID: "b"},
		{KeyP8: p8, KeyID: "K", TeamID: "T"},
		{KeyP8: "not pem", KeyID: "K", TeamID: "T", BundleID: "b"},
		{KeyP8: p8, KeyID: "K", TeamID: "T", BundleID: "b", Environment: "moon"},
	} {
		if _, err := NewAPNsSender(bad); err == nil {
			t.Fatalf("must reject %+v", bad.Environment)
		}
	}
}

func TestDispatcher_fournisseurAbsent(t *testing.T) {
	var d *Dispatcher
	if d.For(PlatformAndroid) != nil {
		t.Fatal("nil dispatcher has no sender")
	}
	d = &Dispatcher{}
	if d.For(PlatformAndroid) != nil || d.For(PlatformIOS) != nil || d.For("windows") != nil {
		t.Fatal("unconfigured providers must be nil")
	}
}
