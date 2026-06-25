package admin

import (
	"errors"
	"net/http"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

var (
	errForbidden     = errors.New("forbidden")
	errNotFound      = errors.New("not found")
	errConflict      = errors.New("conflict")
	errBadRequest    = errors.New("bad request")
	errSelfAction    = errors.New("self action")
	errLastAdmin     = errors.New("last admin")
	errInvalidRole   = errors.New("invalid role")
)

type AdminAction string

const (
	ActionUpdateRole  AdminAction = "update_role"
	ActionUpdateLinks AdminAction = "update_links"
	ActionForbid      AdminAction = "forbid"
	ActionUnforbid    AdminAction = "unforbid"
	ActionDelete      AdminAction = "delete"
)

func isPrivilegedRole(role string) bool {
	return role == domain.RoleAdminGlobal || role == domain.RoleAdminLocal || role == domain.RoleSupport
}

func isLocalManageableRole(role string) bool {
	return role == domain.RoleUser || role == domain.RoleGuestLocal
}

func callerOwnsTarget(caller *domain.User, target *domain.User) bool {
	if caller.LinkedMachineID == nil || target.LinkedMachineID == nil {
		return false
	}
	return *caller.LinkedMachineID == *target.LinkedMachineID
}

type adminUserStore interface {
	GetUserByID(id int) (*domain.User, error)
	CountAdminGlobal() (int, error)
}

// AuthorizeAdminTarget checks whether caller may perform action on targetUserID.
func AuthorizeAdminTarget(users adminUserStore, caller *domain.User, targetUserID int, action AdminAction, newRole string) (*domain.User, error) {
	if users == nil || caller == nil {
		return nil, errForbidden
	}
	target, err := users.GetUserByID(targetUserID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, errNotFound
	}

	switch caller.Role {
	case domain.RoleAdminGlobal:
		switch action {
		case ActionDelete:
			if caller.ID == target.ID {
				return nil, errSelfAction
			}
			if target.Role == domain.RoleAdminGlobal {
				count, err := users.CountAdminGlobal()
				if err != nil {
					return nil, err
				}
				if count <= 1 {
					return nil, errLastAdmin
				}
			}
		case ActionForbid, ActionUnforbid:
			if caller.ID == target.ID {
				return nil, errSelfAction
			}
		case ActionUpdateRole:
			// global admin may set any role
		case ActionUpdateLinks:
			// global admin may link any user
		}
		return target, nil

	case domain.RoleAdminLocal:
		if !callerOwnsTarget(caller, target) {
			return nil, errForbidden
		}
		if isPrivilegedRole(target.Role) {
			return nil, errForbidden
		}
		switch action {
		case ActionUpdateRole:
			if !isLocalManageableRole(newRole) {
				return nil, errInvalidRole
			}
		case ActionUpdateLinks, ActionForbid, ActionUnforbid, ActionDelete:
			if !isLocalManageableRole(target.Role) {
				return nil, errForbidden
			}
		}
		return target, nil

	default:
		return nil, errForbidden
	}
}

func writeAuthzError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errNotFound):
		http.Error(w, "User not found", http.StatusNotFound)
	case errors.Is(err, errSelfAction):
		http.Error(w, "Forbidden: cannot perform this action on your own account", http.StatusForbidden)
	case errors.Is(err, errLastAdmin):
		http.Error(w, "Conflict: cannot remove the last global admin", http.StatusConflict)
	case errors.Is(err, errInvalidRole):
		http.Error(w, "Forbidden: invalid role for local admin", http.StatusForbidden)
	case errors.Is(err, errForbidden):
		http.Error(w, "Forbidden: Insufficient Permissions", http.StatusForbidden)
	default:
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
