// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package todo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/auth"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/runtime"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// Deps holds dependencies for todo flows.
type Deps struct {
	DB    *sql.DB
	Todos *TodoRepo
	Users *auth.UserRepo
	Bus   *runtime.EventBus
}

// CreateTodoResult is the ok payload from CreateTodo.
type CreateTodoResult struct {
	Todo shared.Id `json:"todo_id"`
}

// CreateTodo creates a new todo owned by the authenticated caller.
// creator_role is captured from session.role at creation time.
func CreateTodo(ctx context.Context, deps Deps, callerID shared.Id, callerRole shared.Role, text string, now time.Time, key shared.Key) (CreateTodoResult, error) {
	// Idempotency check.
	if cached, err := loadIdem(ctx, deps.DB, key, "create_todo"); err == nil {
		var r CreateTodoResult
		if err2 := json.Unmarshal([]byte(cached), &r); err2 == nil {
			return r, nil
		}
	}

	// step empty = if length(text) == 0 then reject EmptyText
	if len(text) == 0 {
		return CreateTodoResult{}, shared.ErrEmptyText
	}

	// step todo = ask Todo.create(...)
	todoID := shared.Id(ksuid.New().String())
	t := Todo{
		ID:          todoID,
		Text:        text,
		Done:        false,
		Owner:       callerID,
		CreatorRole: callerRole,
		AssignedTo:  nil,
		Created:     now,
		Updated:     now,
	}
	if _, err := deps.Todos.Create(ctx, t); err != nil {
		return CreateTodoResult{}, err
	}

	result := CreateTodoResult{Todo: todoID}
	saveIdem(ctx, deps.DB, key, "create_todo", string(callerID), result) //nolint:errcheck

	// emit TodoCreated
	deps.Bus.Publish(ctx, TodoCreated{
		Todo:        todoID,
		Owner:       callerID,
		CreatorRole: callerRole,
		At:          now,
	})

	return result, nil
}

// UpdateTodo replaces the text of an existing todo.
// Governed by CanEditTodo.
func UpdateTodo(ctx context.Context, deps Deps, callerID shared.Id, callerRole shared.Role, todoID shared.Id, text string, now time.Time, key shared.Key) error {
	// step t = ask Todo.findBy(id: todo)
	t, err := deps.Todos.FindByID(ctx, todoID)
	if err != nil {
		return shared.ErrTodoNotFound
	}

	// policy: CanEditTodo
	if err := CanEditTodo(callerID, callerRole, t); err != nil {
		return err
	}

	// step _ = ask Todo(todo).Update(text, now)
	if err := deps.Todos.Update(ctx, todoID, text, now); err != nil {
		return err
	}

	// emit TodoUpdated
	deps.Bus.Publish(ctx, TodoUpdated{Todo: todoID, Owner: t.Owner, Text: text, At: now})

	return nil
}

// ToggleTodoResult is the ok payload from ToggleTodo.
type ToggleTodoResult struct {
	Done bool `json:"done"`
}

// ToggleTodo flips the done flag on a todo.
// Governed by CanEditTodo.
func ToggleTodo(ctx context.Context, deps Deps, callerID shared.Id, callerRole shared.Role, todoID shared.Id, now time.Time, key shared.Key) (ToggleTodoResult, error) {
	// step t = ask Todo.findBy(id: todo)
	t, err := deps.Todos.FindByID(ctx, todoID)
	if err != nil {
		return ToggleTodoResult{}, shared.ErrTodoNotFound
	}

	// policy: CanEditTodo
	if err := CanEditTodo(callerID, callerRole, t); err != nil {
		return ToggleTodoResult{}, err
	}

	// step _ = ask Todo(todo).Toggle(now)
	newDone, err := deps.Todos.Toggle(ctx, todoID, now)
	if err != nil {
		return ToggleTodoResult{}, err
	}

	// emit TodoToggled
	deps.Bus.Publish(ctx, TodoToggled{Todo: todoID, Owner: t.Owner, Done: newDone, At: now})

	return ToggleTodoResult{Done: newDone}, nil
}

// DeleteTodo deletes a todo. Governed by CanDeleteTodo.
func DeleteTodo(ctx context.Context, deps Deps, callerID shared.Id, callerRole shared.Role, todoID shared.Id, now time.Time, key shared.Key) error {
	// step t = ask Todo.findBy(id: todo)
	t, err := deps.Todos.FindByID(ctx, todoID)
	if err != nil {
		return shared.ErrTodoNotFound
	}

	// policy: CanDeleteTodo
	if err := CanDeleteTodo(callerID, callerRole, t); err != nil {
		return err
	}

	// step _ = ask Todo(todo).MarkDeleted(now)
	if err := deps.Todos.Delete(ctx, todoID); err != nil {
		return err
	}

	// emit TodoDeleted
	deps.Bus.Publish(ctx, TodoDeleted{Todo: todoID, Owner: t.Owner, At: now})

	return nil
}

// AssignTodo assigns a todo to a manager. Only admins may call this.
func AssignTodo(ctx context.Context, deps Deps, callerID shared.Id, callerRole shared.Role, todoID shared.Id, managerID shared.Id, now time.Time, key shared.Key) error {
	// policy: CanAssignTodo
	if err := CanAssignTodo(callerRole); err != nil {
		return err
	}

	// step t = ask Todo.findBy(id: todo)
	if _, err := deps.Todos.FindByID(ctx, todoID); err != nil {
		return shared.ErrTodoNotFound
	}

	// step m = ask User.findBy(id: manager)
	m, err := deps.Users.FindByID(ctx, managerID)
	if err != nil {
		return shared.ErrUserNotFound
	}

	// step _ = if m.role != Manager then reject NotManager
	if m.Role != shared.RoleManager {
		return shared.ErrNotManager
	}

	// step _ = ask Todo(todo).Assign(manager, now)
	if err := deps.Todos.Assign(ctx, todoID, managerID, now); err != nil {
		return err
	}

	t, _ := deps.Todos.FindByID(ctx, todoID)
	if t != nil {
		deps.Bus.Publish(ctx, TodoAssigned{Todo: todoID, Owner: t.Owner, Manager: managerID, At: now})
	}

	return nil
}

// ListTodosResult is the ok payload from ListTodos.
type ListTodosResult struct {
	Todos []*Todo `json:"todos"`
}

// ListTodos returns todos according to the filter.
func ListTodos(ctx context.Context, deps Deps, callerID shared.Id, callerRole shared.Role, filter shared.ListFilter, now time.Time) (ListTodosResult, error) {
	// step _ = if filter == All and session.role != Admin then reject NotAuthorized
	if filter == shared.ListFilterAll && callerRole != shared.RoleAdmin {
		return ListTodosResult{}, shared.ErrNotAuthorized
	}

	var todos []*Todo
	var err error
	switch filter {
	case shared.ListFilterAll:
		todos, err = deps.Todos.ListAll(ctx)
	case shared.ListFilterMineOnly:
		todos, err = deps.Todos.ListByOwner(ctx, callerID)
	default: // AssignedToMe
		todos, err = deps.Todos.ListAssignedTo(ctx, callerID)
	}
	if err != nil {
		return ListTodosResult{}, err
	}
	if todos == nil {
		todos = []*Todo{}
	}
	return ListTodosResult{Todos: todos}, nil
}

// --- Idempotency helpers ---

func loadIdem(ctx context.Context, db *sql.DB, key shared.Key, flow string) (string, error) {
	var result string
	err := db.QueryRowContext(ctx,
		`SELECT result_json FROM idempotency_keys WHERE key_val=? AND flow=?`,
		string(key), flow,
	).Scan(&result)
	return result, err
}

func saveIdem(ctx context.Context, db *sql.DB, key shared.Key, flow string, userID string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`INSERT OR IGNORE INTO idempotency_keys(key_val,user_id,flow,result_json,created_at) VALUES(?,?,?,?,?)`,
		string(key), userID, flow, string(b), time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
