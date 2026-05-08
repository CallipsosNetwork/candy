// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package todo

import (
	"time"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// TodoCreated — delivery: eager
type TodoCreated struct {
	Todo        shared.Id
	Owner       shared.Id
	CreatorRole shared.Role
	At          time.Time
}

// TodoUpdated — delivery: eager
type TodoUpdated struct {
	Todo  shared.Id
	Owner shared.Id
	Text  string
	At    time.Time
}

// TodoToggled — delivery: eager
type TodoToggled struct {
	Todo  shared.Id
	Owner shared.Id
	Done  bool
	At    time.Time
}

// TodoDeleted — delivery: eager
type TodoDeleted struct {
	Todo  shared.Id
	Owner shared.Id
	At    time.Time
}

// TodoAssigned — delivery: eager
type TodoAssigned struct {
	Todo    shared.Id
	Owner   shared.Id
	Manager shared.Id
	At      time.Time
}
