// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package test

import (
	"testing"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/auth"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/todo"
)

// ── PasswordStrength policy examples (from spec) ──────────────────────────────

func TestPasswordStrength_ok(t *testing.T) {
	// given: "correct horse battery staple 9" → ok
	if err := auth.PasswordStrength("correct horse battery staple 9"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestPasswordStrength_tooShort(t *testing.T) {
	// given: "short1" → err(TooShort)
	err := auth.PasswordStrength("short1")
	if err == nil {
		t.Fatal("expected TooShort error")
	}
	if !isWeakWith(err, shared.ErrTooShort) {
		t.Fatalf("expected TooShort, got %v", err)
	}
}

func TestPasswordStrength_missingDigit(t *testing.T) {
	// given: "alllowercase" → err(MissingDigit)
	err := auth.PasswordStrength("alllowercase")
	if err == nil {
		t.Fatal("expected MissingDigit error")
	}
	if !isWeakWith(err, shared.ErrMissingDigit) {
		t.Fatalf("expected MissingDigit, got %v", err)
	}
}

func TestPasswordStrength_inBlocklist(t *testing.T) {
	// given: "password123" → err(InBlocklist)
	err := auth.PasswordStrength("password123")
	if err == nil {
		t.Fatal("expected InBlocklist error")
	}
	if !isWeakWith(err, shared.ErrInBlocklist) {
		t.Fatalf("expected InBlocklist, got %v", err)
	}
}

// ── RoleGated policy examples ─────────────────────────────────────────────────

func TestRoleGated_adminRequired_adminCaller(t *testing.T) {
	// required Admin, caller Admin → ok
	if shared.RoleLevel(shared.RoleAdmin) < shared.RoleLevel(shared.RoleAdmin) {
		t.Fatal("expected ok")
	}
}

func TestRoleGated_adminRequired_managerCaller(t *testing.T) {
	// required Admin, caller Manager → err(NotAuthorized)
	if shared.RoleLevel(shared.RoleManager) >= shared.RoleLevel(shared.RoleAdmin) {
		t.Fatal("expected not_authorized")
	}
}

func TestRoleGated_adminRequired_userCaller(t *testing.T) {
	// required Admin, caller User → err(NotAuthorized)
	if shared.RoleLevel(shared.RoleUser) >= shared.RoleLevel(shared.RoleAdmin) {
		t.Fatal("expected not_authorized")
	}
}

func TestRoleGated_managerRequired_adminCaller(t *testing.T) {
	// required Manager, caller Admin → ok
	if shared.RoleLevel(shared.RoleAdmin) < shared.RoleLevel(shared.RoleManager) {
		t.Fatal("expected ok")
	}
}

func TestRoleGated_managerRequired_managerCaller(t *testing.T) {
	// required Manager, caller Manager → ok
	if shared.RoleLevel(shared.RoleManager) < shared.RoleLevel(shared.RoleManager) {
		t.Fatal("expected ok")
	}
}

func TestRoleGated_managerRequired_userCaller(t *testing.T) {
	// required Manager, caller User → err(NotAuthorized)
	if shared.RoleLevel(shared.RoleUser) >= shared.RoleLevel(shared.RoleManager) {
		t.Fatal("expected not_authorized")
	}
}

// ── CanEditTodo policy examples ───────────────────────────────────────────────

func ptr(id shared.Id) *shared.Id { return &id }

func TestCanEditTodo_ownerEdits(t *testing.T) {
	// caller user U1 (role User), todo.owner = U1 → ok
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser}
	if err := todo.CanEditTodo("U1", shared.RoleUser, t1); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestCanEditTodo_adminEditsAny(t *testing.T) {
	// caller user A1 (role Admin), todo.owner = U1 → ok
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser}
	if err := todo.CanEditTodo("A1", shared.RoleAdmin, t1); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestCanEditTodo_assignedManagerEdits(t *testing.T) {
	// caller user M1 (role Manager), todo.owner = U1, todo.assigned_to = M1 → ok
	aid := shared.Id("M1")
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser, AssignedTo: &aid}
	if err := todo.CanEditTodo("M1", shared.RoleManager, t1); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestCanEditTodo_unassignedManagerFails(t *testing.T) {
	// caller M2, assigned_to = M1 → err(NotAuthorized)
	aid := shared.Id("M1")
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser, AssignedTo: &aid}
	if err := todo.CanEditTodo("M2", shared.RoleManager, t1); err == nil {
		t.Fatal("expected not_authorized")
	}
}

func TestCanEditTodo_nonOwnerUserFails(t *testing.T) {
	// caller U2, owner = U1 → err(NotAuthorized)
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser}
	if err := todo.CanEditTodo("U2", shared.RoleUser, t1); err == nil {
		t.Fatal("expected not_authorized")
	}
}

func TestCanEditTodo_managerWithNullAssignedFails(t *testing.T) {
	// caller M1, todo.assigned_to = null → err(NotAuthorized)
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser, AssignedTo: nil}
	if err := todo.CanEditTodo("M1", shared.RoleManager, t1); err == nil {
		t.Fatal("expected not_authorized")
	}
}

// ── CanDeleteTodo policy examples ─────────────────────────────────────────────

func TestCanDeleteTodo_adminDeletesAny(t *testing.T) {
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser}
	if err := todo.CanDeleteTodo("A1", shared.RoleAdmin, t1); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestCanDeleteTodo_userDeletesOwn_userCreated(t *testing.T) {
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser}
	if err := todo.CanDeleteTodo("U1", shared.RoleUser, t1); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestCanDeleteTodo_userDeletesOwn_adminCreated_fails(t *testing.T) {
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleAdmin}
	if err := todo.CanDeleteTodo("U1", shared.RoleUser, t1); err == nil {
		t.Fatal("expected not_authorized")
	}
}

func TestCanDeleteTodo_managerDeletesOwn_fails(t *testing.T) {
	t1 := &todo.Todo{Owner: "M1", CreatorRole: shared.RoleManager}
	if err := todo.CanDeleteTodo("M1", shared.RoleManager, t1); err == nil {
		t.Fatal("expected not_authorized")
	}
}

func TestCanDeleteTodo_nonOwnerUserFails(t *testing.T) {
	t1 := &todo.Todo{Owner: "U1", CreatorRole: shared.RoleUser}
	if err := todo.CanDeleteTodo("U2", shared.RoleUser, t1); err == nil {
		t.Fatal("expected not_authorized")
	}
}

func TestCanDeleteTodo_adminDeletesOwnAdminCreated(t *testing.T) {
	t1 := &todo.Todo{Owner: "A1", CreatorRole: shared.RoleAdmin}
	if err := todo.CanDeleteTodo("A1", shared.RoleAdmin, t1); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

// ── CanAssignTodo policy examples ─────────────────────────────────────────────

func TestCanAssignTodo_adminOk(t *testing.T) {
	if err := todo.CanAssignTodo(shared.RoleAdmin); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestCanAssignTodo_managerFails(t *testing.T) {
	if err := todo.CanAssignTodo(shared.RoleManager); err == nil {
		t.Fatal("expected not_authorized")
	}
}

func TestCanAssignTodo_userFails(t *testing.T) {
	if err := todo.CanAssignTodo(shared.RoleUser); err == nil {
		t.Fatal("expected not_authorized")
	}
}

// helpers

func isWeakWith(err error, reason error) bool {
	var wpe *shared.WeakPasswordErr
	if !isAs(err, &wpe) {
		return false
	}
	return wpe.Reason == reason
}

func isAs(err error, target any) bool {
	switch t := target.(type) {
	case **shared.WeakPasswordErr:
		if wpe, ok := err.(*shared.WeakPasswordErr); ok {
			*t = wpe
			return true
		}
	}
	return false
}
