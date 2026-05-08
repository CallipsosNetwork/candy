// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package auth

import (
	"time"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// UserSignedUp is emitted by the Signup flow.
// delivery: eager
type UserSignedUp struct {
	User  shared.Id
	Email shared.Email
	At    time.Time
}

// UserLoggedIn is emitted by the Login flow.
// delivery: eager, order: by user
type UserLoggedIn struct {
	User shared.Id
	At   time.Time
}

// UserVerified is emitted by User.Verify().
// delivery: eager
type UserVerified struct {
	User shared.Id
	At   time.Time
}

// SessionRevoked is emitted by Logout.
// payload keeps token (the JWT string) per spec — "Tokens never log" applies
// to log lines, not event payloads.
// delivery: eager
type SessionRevoked struct {
	Token shared.Token
	User  shared.Id
	At    time.Time
}
