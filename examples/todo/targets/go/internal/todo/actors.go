// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package todo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// Todo is the persistent state of a Todo actor.
type Todo struct {
	ID          shared.Id
	Text        string
	Done        bool
	Owner       shared.Id
	CreatorRole shared.Role
	AssignedTo  *shared.Id
	Created     time.Time
	Updated     time.Time
}

// TodoRepo owns reads/writes for the todos table.
type TodoRepo struct {
	DB *sql.DB
}

func (r *TodoRepo) Create(ctx context.Context, t Todo) (*Todo, error) {
	var assignedTo *string
	if t.AssignedTo != nil {
		s := string(*t.AssignedTo)
		assignedTo = &s
	}
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO todos(id,text,done,owner,creator_role,assigned_to,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		string(t.ID), t.Text, boolInt(t.Done),
		string(t.Owner), string(t.CreatorRole), assignedTo,
		t.Created.UTC().Format(time.RFC3339),
		t.Updated.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TodoRepo) FindByID(ctx context.Context, id shared.Id) (*Todo, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT id,text,done,owner,creator_role,assigned_to,created_at,updated_at
		 FROM todos WHERE id=?`, string(id))
	return scanTodo(row)
}

func (r *TodoRepo) Update(ctx context.Context, id shared.Id, text string, now time.Time) error {
	if len(text) == 0 {
		return shared.ErrEmptyText
	}
	res, err := r.DB.ExecContext(ctx,
		`UPDATE todos SET text=?, updated_at=? WHERE id=?`,
		text, now.UTC().Format(time.RFC3339), string(id),
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return shared.ErrTodoNotFound
	}
	return nil
}

func (r *TodoRepo) Toggle(ctx context.Context, id shared.Id, now time.Time) (bool, error) {
	t, err := r.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	newDone := !t.Done
	_, err = r.DB.ExecContext(ctx,
		`UPDATE todos SET done=?, updated_at=? WHERE id=?`,
		boolInt(newDone), now.UTC().Format(time.RFC3339), string(id),
	)
	return newDone, err
}

func (r *TodoRepo) Assign(ctx context.Context, id shared.Id, managerID shared.Id, now time.Time) error {
	res, err := r.DB.ExecContext(ctx,
		`UPDATE todos SET assigned_to=?, updated_at=? WHERE id=?`,
		string(managerID), now.UTC().Format(time.RFC3339), string(id),
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return shared.ErrTodoNotFound
	}
	return nil
}

func (r *TodoRepo) Delete(ctx context.Context, id shared.Id) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM todos WHERE id=?`, string(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return shared.ErrTodoNotFound
	}
	return nil
}

// ListAll returns all todos (Admin only).
func (r *TodoRepo) ListAll(ctx context.Context) ([]*Todo, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id,text,done,owner,creator_role,assigned_to,created_at,updated_at FROM todos ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTodos(rows)
}

// ListByOwner returns todos where owner == userID.
func (r *TodoRepo) ListByOwner(ctx context.Context, userID shared.Id) ([]*Todo, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id,text,done,owner,creator_role,assigned_to,created_at,updated_at
		 FROM todos WHERE owner=? ORDER BY created_at`, string(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTodos(rows)
}

// ListAssignedTo returns todos where assigned_to == managerID.
func (r *TodoRepo) ListAssignedTo(ctx context.Context, managerID shared.Id) ([]*Todo, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id,text,done,owner,creator_role,assigned_to,created_at,updated_at
		 FROM todos WHERE assigned_to=? ORDER BY created_at`, string(managerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTodos(rows)
}

func scanTodo(row *sql.Row) (*Todo, error) {
	var t Todo
	var id, text, owner, creatorRole, created, updated string
	var done int
	var assignedTo *string
	err := row.Scan(&id, &text, &done, &owner, &creatorRole, &assignedTo, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrTodoNotFound
	}
	if err != nil {
		return nil, err
	}
	t.ID = shared.Id(id)
	t.Text = text
	t.Done = done != 0
	t.Owner = shared.Id(owner)
	t.CreatorRole = shared.Role(creatorRole)
	if assignedTo != nil {
		aid := shared.Id(*assignedTo)
		t.AssignedTo = &aid
	}
	t.Created, _ = time.Parse(time.RFC3339, created)
	t.Updated, _ = time.Parse(time.RFC3339, updated)
	return &t, nil
}

func collectTodos(rows *sql.Rows) ([]*Todo, error) {
	var out []*Todo
	for rows.Next() {
		var t Todo
		var id, text, owner, creatorRole, created, updated string
		var done int
		var assignedTo *string
		if err := rows.Scan(&id, &text, &done, &owner, &creatorRole, &assignedTo, &created, &updated); err != nil {
			return nil, err
		}
		t.ID = shared.Id(id)
		t.Text = text
		t.Done = done != 0
		t.Owner = shared.Id(owner)
		t.CreatorRole = shared.Role(creatorRole)
		if assignedTo != nil {
			aid := shared.Id(*assignedTo)
			t.AssignedTo = &aid
		}
		t.Created, _ = time.Parse(time.RFC3339, created)
		t.Updated, _ = time.Parse(time.RFC3339, updated)
		out = append(out, &t)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
