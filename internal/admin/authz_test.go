package admin

import (
	"errors"
	"net/http"
	"testing"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
)

type mockUserStore struct {
	users map[int]*domain.User
}

func (m *mockUserStore) GetUserByID(id int) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserStore) CountAdminGlobal() (int, error) {
	n := 0
	for _, u := range m.users {
		if u.Role == domain.RoleAdminGlobal {
			n++
		}
	}
	return n, nil
}

func machineID(v int) *int { return &v }

func TestAuthorizeAdminTargetLocalCannotForbidAdmin(t *testing.T) {
	store := &mockUserStore{
		users: map[int]*domain.User{
			1: {ID: 1, Role: domain.RoleAdminLocal, LinkedMachineID: machineID(10)},
			2: {ID: 2, Role: domain.RoleAdminLocal, LinkedMachineID: machineID(10)},
		},
	}
	caller := store.users[1]
	_, err := AuthorizeAdminTarget(store, caller, 2, ActionForbid, "")
	if !errors.Is(err, errForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAuthorizeAdminTargetLocalCanForbidGuest(t *testing.T) {
	store := &mockUserStore{
		users: map[int]*domain.User{
			1: {ID: 1, Role: domain.RoleAdminLocal, LinkedMachineID: machineID(10)},
			2: {ID: 2, Role: domain.RoleGuestLocal, LinkedMachineID: machineID(10)},
		},
	}
	caller := store.users[1]
	target, err := AuthorizeAdminTarget(store, caller, 2, ActionForbid, "")
	if err != nil || target.ID != 2 {
		t.Fatalf("expected allow, got target=%v err=%v", target, err)
	}
}

func TestAuthorizeAdminTargetCannotDeleteSelf(t *testing.T) {
	store := &mockUserStore{
		users: map[int]*domain.User{
			1: {ID: 1, Role: domain.RoleAdminGlobal},
		},
	}
	caller := store.users[1]
	_, err := AuthorizeAdminTarget(store, caller, 1, ActionDelete, "")
	if !errors.Is(err, errSelfAction) {
		t.Fatalf("expected self action, got %v", err)
	}
}

func TestWriteAuthzError(t *testing.T) {
	rec := &recordingResponseWriter{}
	writeAuthzError(rec, errLastAdmin)
	if rec.code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.code)
	}
}

type recordingResponseWriter struct {
	code int
}

func (r *recordingResponseWriter) Header() http.Header         { return http.Header{} }
func (r *recordingResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (r *recordingResponseWriter) WriteHeader(code int)        { r.code = code }
