// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package auth

import (
	"time"

	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/shared"
)

// UserSignedUp — delivery: eager; emitted by Signup flow.
type UserSignedUp struct {
	User  shared.Id
	Email shared.Email
	At    time.Time
}

// UserLoggedIn — delivery: eager, order: by user; emitted by Login flow.
type UserLoggedIn struct {
	User shared.Id
	At   time.Time
}

// UserVerified — delivery: eager; emitted by User.Verify.
type UserVerified struct {
	User shared.Id
	At   time.Time
}

// SessionRevoked — delivery: eager; emitted by Logout.
// Spec payload: { token: Token, user: Id, at: Timestamp }.
// The Token field carries the JWT string. The "tokens never log" rule
// applies to log lines (slog.Info / errors); the event payload is
// internal data, not a log surface.
type SessionRevoked struct {
	Token shared.Token
	User  shared.Id
	At    time.Time
}
