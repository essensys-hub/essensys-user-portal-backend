package identity

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var googleOauthConfig *oauth2.Config

func getOAuthConfig() *oauth2.Config {
	if googleOauthConfig == nil {
		googleOauthConfig = &oauth2.Config{
			RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
			Endpoint:     google.Endpoint,
		}
	}
	return googleOauthConfig
}

func isAdminEmail(email, adminList string) bool {
	for _, admin := range strings.Split(adminList, ",") {
		if strings.TrimSpace(admin) == email {
			return true
		}
	}
	return false
}

func (h *Handlers) GoogleLogin(w http.ResponseWriter, req *http.Request) {
	oauthState := generateStateOauthCookie(w)
	u := getOAuthConfig().AuthCodeURL(oauthState)
	http.Redirect(w, req, u, http.StatusTemporaryRedirect)
}

func (h *Handlers) GoogleCallback(w http.ResponseWriter, req *http.Request) {
	oauthState, err := req.Cookie("oauthstate")
	if err != nil {
		log.Println("OAuth cookie missing in Google callback")
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}
	if req.FormValue("state") != oauthState.Value {
		log.Println("Invalid oauth google state")
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	data, err := getUserDataFromGoogle(req.FormValue("code"))
	if err != nil {
		log.Println(err.Error())
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	var user struct {
		Email string `json:"email"`
		ID    string `json:"id"`
	}
	if err := json.Unmarshal(data, &user); err != nil {
		log.Println("Failed to parse user data")
		http.Error(w, "Failed to parse user data", http.StatusInternalServerError)
		return
	}

	userDB, err := h.users.GetUserByEmail(user.Email)
	if err != nil {
		log.Println("Database error checking user:", err)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}

	var role string
	if userDB == nil {
		if isAdminEmail(user.Email, os.Getenv("ADMIN_EMAILS")) {
			role = domain.RoleAdminGlobal
		} else {
			role = domain.RoleUser
		}
		newUser := &domain.User{
			Email:      user.Email,
			Role:       role,
			Provider:   domain.ProviderGoogle,
			ProviderID: user.ID,
			CreatedAt:  time.Now(),
			LastLogin:  time.Now(),
		}
		if err := h.users.CreateUser(newUser); err != nil {
			log.Println("Failed to create user from Google:", err)
			http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
			return
		}
	} else {
		_ = h.users.UpdateLastLogin(userDB.ID)
		role = userDB.Role
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	tokenString, err := middleware.GenerateJWT(user.Email, role, expirationTime)
	if err != nil {
		log.Println("Failed to generate token:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "/"
	}
	http.Redirect(w, req, frontendURL+"admin?token="+tokenString+"&role="+role, http.StatusTemporaryRedirect)
}

func getUserDataFromGoogle(code string) ([]byte, error) {
	token, err := getOAuthConfig().Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("code exchange wrong: %s", err.Error())
	}
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	return io.ReadAll(response.Body)
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    state,
		Expires:  time.Now().Add(20 * time.Minute),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/",
	})
	return state
}

func getAppleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:    os.Getenv("APPLE_CLIENT_ID"),
		RedirectURL: os.Getenv("APPLE_REDIRECT_URL"),
		Scopes:      []string{"name", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://appleid.apple.com/auth/authorize",
			TokenURL: "https://appleid.apple.com/auth/token",
		},
	}
}

func generateAppleClientSecret() (string, error) {
	keyBytes, err := os.ReadFile(os.Getenv("APPLE_KEY_FILE"))
	if err != nil {
		return "", fmt.Errorf("could not read private key file: %v", err)
	}
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block containing private key")
	}
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("could not parse private key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": os.Getenv("APPLE_TEAM_ID"),
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"aud": "https://appleid.apple.com",
		"sub": os.Getenv("APPLE_CLIENT_ID"),
	})
	token.Header["kid"] = os.Getenv("APPLE_KEY_ID")
	return token.SignedString(privateKey)
}

func (h *Handlers) AppleLogin(w http.ResponseWriter, req *http.Request) {
	oauthState := generateStateOauthCookie(w)
	authURL := getAppleOAuthConfig().AuthCodeURL(oauthState, oauth2.SetAuthURLParam("response_mode", "form_post"))
	http.Redirect(w, req, authURL, http.StatusTemporaryRedirect)
}

func (h *Handlers) AppleCallback(w http.ResponseWriter, req *http.Request) {
	oauthState, err := req.Cookie("oauthstate")
	if err != nil {
		log.Printf("OAuth cookie missing in Apple callback")
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}
	if req.FormValue("state") != oauthState.Value {
		log.Println("Invalid oauth apple state")
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}
	code := req.FormValue("code")
	if code == "" {
		http.Error(w, "No code returned from Apple", http.StatusBadRequest)
		return
	}

	clientSecret, err := generateAppleClientSecret()
	if err != nil {
		log.Printf("Failed to generate apple client secret: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	conf := getAppleOAuthConfig()
	values := url.Values{}
	values.Set("client_id", conf.ClientID)
	values.Set("client_secret", clientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", conf.RedirectURL)

	resp, err := http.PostForm(conf.Endpoint.TokenURL, values)
	if err != nil {
		log.Printf("Failed to exchange token with Apple: %v", err)
		http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Apple Token Exchange failed: %v body=%s", err, string(bodyBytes))
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}

	var tokenResp struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}

	parser := jwt.Parser{SkipClaimsValidation: true}
	idToken, _, err := parser.ParseUnverified(tokenResp.IDToken, jwt.MapClaims{})
	if err != nil {
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}
	claims, ok := idToken.Claims.(jwt.MapClaims)
	if !ok {
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}
	email, _ := claims["email"].(string)

	userDB, err := h.users.GetUserByEmail(email)
	if err != nil {
		http.Redirect(w, req, "/", http.StatusSeeOther)
		return
	}

	var role string
	if userDB == nil {
		if isAdminEmail(email, os.Getenv("ADMIN_EMAILS")) {
			role = domain.RoleAdminGlobal
		} else {
			role = domain.RoleUser
		}
		newUser := &domain.User{
			Email:     email,
			Role:      role,
			Provider:  domain.ProviderApple,
			CreatedAt: time.Now(),
			LastLogin: time.Now(),
		}
		if err := h.users.CreateUser(newUser); err != nil {
			http.Redirect(w, req, "/", http.StatusSeeOther)
			return
		}
	} else {
		_ = h.users.UpdateLastLogin(userDB.ID)
		role = userDB.Role
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	tokenString, err := middleware.GenerateJWT(email, role, expirationTime)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "/"
	}
	http.Redirect(w, req, frontendURL+"admin?token="+tokenString+"&role="+role, http.StatusSeeOther)
}
