package admin

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handlers struct {
	users     *data.UserStore
	audit     *data.AuditStore
	inventory *data.AdminInventoryStore
	news      *data.NewsletterStore
}

type Deps struct {
	Users     *data.UserStore
	Audit     *data.AuditStore
	Inventory *data.AdminInventoryStore
	News      *data.NewsletterStore
}

func NewHandlers(d Deps) *Handlers {
	return &Handlers{
		users:     d.Users,
		audit:     d.Audit,
		inventory: d.Inventory,
		news:      d.News,
	}
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	expectedToken := os.Getenv("ADMIN_TOKEN")
	if expectedToken == "" {
		expectedToken = "essensys-admin-secret"
	}
	if req.Token != expectedToken {
		http.Error(w, "Invalid Token", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) Stats(w http.ResponseWriter, r *http.Request) {
	if h.inventory == nil {
		writeJSON(w, http.StatusOK, domain.AdminStatsResponse{})
		return
	}
	stats, err := h.inventory.GetStats()
	if err != nil {
		log.Printf("[admin] stats: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handlers) Machines(w http.ResponseWriter, r *http.Request) {
	if h.inventory == nil {
		writeJSON(w, http.StatusOK, []*domain.MachineDetail{})
		return
	}
	machines, err := h.inventory.GetMachines()
	if err != nil {
		log.Printf("[admin] machines: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, machines)
}

func (h *Handlers) Gateways(w http.ResponseWriter, r *http.Request) {
	if h.inventory == nil {
		writeJSON(w, http.StatusOK, []*domain.GatewayStatus{})
		return
	}
	gateways, err := h.inventory.GetGateways()
	if err != nil {
		log.Printf("[admin] gateways: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, gateways)
}

func (h *Handlers) GetUsers(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		http.Error(w, "User Store not initialized", http.StatusServiceUnavailable)
		return
	}
	email, ok := r.Context().Value(middleware.UserEmailKey).(string)
	if !ok || email == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	currentUser, err := h.users.GetUserByEmail(email)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var users []*domain.User
	switch currentUser.Role {
	case domain.RoleAdminGlobal:
		users, err = h.users.GetAllUsers()
	case domain.RoleAdminLocal:
		if currentUser.LinkedMachineID == nil {
			users = []*domain.User{}
		} else {
			users, err = h.users.GetUsersByMachineID(*currentUser.LinkedMachineID)
		}
	default:
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handlers) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		http.Error(w, "User Store not initialized", http.StatusServiceUnavailable)
		return
	}
	email, _ := r.Context().Value(middleware.UserEmailKey).(string)
	currentUser, err := h.users.GetUserByEmail(email)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	targetUserID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	allowed := false
	if currentUser.Role == domain.RoleAdminGlobal {
		allowed = true
	} else if currentUser.Role == domain.RoleAdminLocal {
		if currentUser.LinkedMachineID != nil {
			myUsers, _ := h.users.GetUsersByMachineID(*currentUser.LinkedMachineID)
			for _, u := range myUsers {
				if u.ID == targetUserID {
					allowed = true
					break
				}
			}
		}
		if req.Role != domain.RoleUser && req.Role != domain.RoleGuestLocal {
			allowed = false
		}
	}
	if !allowed {
		http.Error(w, "Forbidden: Insufficient Permissions", http.StatusForbidden)
		return
	}

	if err := h.users.UpdateUserRole(targetUserID, req.Role); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	h.logAudit(currentUser.ID, currentUser.Email, "UPDATE_ROLE", "USER", idStr, clientIP(r),
		"Updated role for user "+idStr+" to "+req.Role)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		http.Error(w, "User Store not initialized", http.StatusServiceUnavailable)
		return
	}
	adminEmail, _ := r.Context().Value(middleware.UserEmailKey).(string)
	adminID := 0
	if adminEmail != "" {
		if u, err := h.users.GetUserByEmail(adminEmail); err == nil && u != nil {
			adminID = u.ID
		}
	}

	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	existing, err := h.users.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		return
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hashed),
		Role:         domain.RoleGuestLocal,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Provider:     domain.ProviderEmail,
		CreatedAt:    time.Now(),
		LastLogin:    time.Now(),
	}
	if err := h.users.CreateUser(user); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	h.logAudit(adminID, adminEmail, "CREATE_USER", "USER", user.Email, clientIP(r), "Created user by admin")
	writeJSON(w, http.StatusOK, map[string]string{"message": "User created successfully"})
}

func (h *Handlers) UpdateUserLinks(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		http.Error(w, "User Store not initialized", http.StatusServiceUnavailable)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}

	var req struct {
		MachineID *int    `json:"linked_machine_id"`
		GatewayID *string `json:"linked_gateway_id"`
		ArmoireID *int    `json:"linked_armoire_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.GatewayID != nil && !domain.IsRemoteEligibleGateway(req.GatewayID) {
		if req.ArmoireID != nil || req.MachineID != nil {
			http.Error(w, domain.RemoteBlockedMessage(), http.StatusBadRequest)
			return
		}
		req.ArmoireID = nil
		req.MachineID = nil
	}
	if err := h.users.UpdateUserLinks(id, req.MachineID, req.GatewayID, req.ArmoireID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) AuditLogs(w http.ResponseWriter, r *http.Request) {
	if h.audit == nil || h.users == nil {
		http.Error(w, "Audit Store not initialized", http.StatusServiceUnavailable)
		return
	}
	email, _ := r.Context().Value(middleware.UserEmailKey).(string)
	currentUser, err := h.users.GetUserByEmail(email)
	if err != nil || currentUser == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	filter := domain.AuditFilter{Limit: 100}
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil {
			filter.Limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil {
			filter.Offset = val
		}
	}

	switch currentUser.Role {
	case domain.RoleAdminGlobal:
	case domain.RoleAdminLocal:
		if currentUser.LinkedMachineID != nil {
			filter.MachineID = *currentUser.LinkedMachineID
		} else {
			writeJSON(w, http.StatusOK, []*domain.AuditLog{})
			return
		}
	case domain.RoleUser:
		filter.UserID = currentUser.ID
	default:
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	logs, err := h.audit.GetAuditLogs(filter)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []*domain.AuditLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}

func (h *Handlers) logAudit(userID int, username, action, resourceType, resourceID, ip, details string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.CreateAuditLog(&domain.AuditLog{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    ip,
		Details:      details,
		CreatedAt:    time.Now(),
	})
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
