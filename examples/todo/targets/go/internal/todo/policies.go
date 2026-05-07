// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package todo

import (
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// CanEditTodo checks whether caller may update or toggle the given todo.
//
// A todo may be edited when at least one of these holds:
//  1. The caller is the todo's owner (regardless of role).
//  2. The caller's role is Admin.
//  3. The caller's role is Manager AND todo.assigned_to == caller's user id.
func CanEditTodo(callerID shared.Id, callerRole shared.Role, t *Todo) error {
	if t.Owner == callerID {
		return nil
	}
	if callerRole == shared.RoleAdmin {
		return nil
	}
	if callerRole == shared.RoleManager && t.AssignedTo != nil && *t.AssignedTo == callerID {
		return nil
	}
	return shared.ErrNotAuthorized
}

// CanDeleteTodo checks whether caller may delete the given todo.
//
// A todo may be deleted when:
//   - The caller's role is Admin (may delete any todo).
//   - OR: the caller is the todo's owner AND creator_role != Admin.
func CanDeleteTodo(callerID shared.Id, callerRole shared.Role, t *Todo) error {
	if callerRole == shared.RoleAdmin {
		return nil
	}
	// Managers cannot delete anything.
	if callerRole == shared.RoleManager {
		return shared.ErrNotAuthorized
	}
	// User: must be owner and creator_role must not be Admin.
	if t.Owner == callerID && t.CreatorRole != shared.RoleAdmin {
		return nil
	}
	return shared.ErrNotAuthorized
}

// CanAssignTodo checks whether caller may assign a todo to a manager.
// Only admins may assign.
func CanAssignTodo(callerRole shared.Role) error {
	if callerRole == shared.RoleAdmin {
		return nil
	}
	return shared.ErrNotAuthorized
}
