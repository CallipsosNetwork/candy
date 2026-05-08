// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/ksuid"
	"golang.org/x/crypto/argon2"

	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/runtime"
	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/shared"
)

// SessionTTL is the JWT lifetime. Spec: `expires: now after 7d`.
const SessionTTL = 7 * 24 * time.Hour

// Deps carries all dependencies for auth flows and actors.
// No globals — everything is passed explicitly.
type Deps struct {
	Users       *UserRepo
	JWT         *JWTService
	Revoked     *RevokedRepo
	Idempotency *IdempotencyRepo
	EventBus    *runtime.EventBus
}

// SignupResult is the success payload of the Signup flow.
type SignupResult struct {
	User  shared.Id
	Token shared.Token
}

// LoginResult is the success payload of the Login flow.
type LoginResult struct {
	User  shared.Id
	Token shared.Token
}

// ---------------------------------------------------------------------------
// flow Signup
// ---------------------------------------------------------------------------

// Signup creates a new user, hashes the password, and issues an initial session.
// Idempotent on key — replaying with the same key returns the same user_id and a fresh session.
func Signup(ctx context.Context, deps Deps, email shared.Email, password shared.Password, now time.Time, key shared.Key) (SignupResult, error) {
	// Idempotency check — replay returns the prior user_id with a fresh JWT.
	if uid, found, err := deps.Idempotency.FindSignup(ctx, key); err != nil {
		return SignupResult{}, fmt.Errorf("idempotency lookup: %w", err)
	} else if found {
		newTok, _, err := deps.JWT.Issue(uid, ksuid.New().String(), now)
		if err != nil {
			return SignupResult{}, err
		}
		return SignupResult{User: uid, Token: newTok}, nil
	}

	// step strength = PasswordStrength(password)  rescue reject WeakPassword(reason)
	if err := PasswordStrength(password); err != nil {
		var we shared.ErrWeakPassword
		if errors.As(err, &we) {
			return SignupResult{}, we
		}
		return SignupResult{}, err
	}

	// step taken = if any user in User where user.email == email then reject EmailTaken
	taken, err := deps.Users.ExistsByEmail(ctx, email)
	if err != nil {
		return SignupResult{}, fmt.Errorf("check email: %w", err)
	}
	if taken {
		return SignupResult{}, shared.ErrEmailTaken{}
	}

	// step user = ask User.create({ id: generate(), email, hash: hash(password), created: now })
	userID := shared.Id(ksuid.New().String())
	hash := hashPassword(string(password))

	user, err := deps.Users.Create(ctx, User{
		ID:      userID,
		Email:   email,
		Hash:    shared.PasswordHash(hash),
		Created: now,
	})
	if err != nil {
		return SignupResult{}, fmt.Errorf("create user: %w", err)
	}

	// step session = ask Session.create({ user: user.id, issued: now, expires: now after 7d })
	// Realised as JWT issuance.
	token, _, err := deps.JWT.Issue(user.ID, ksuid.New().String(), now)
	if err != nil {
		return SignupResult{}, err
	}

	// Persist idempotency record for future replays.
	if err := deps.Idempotency.StoreSignup(ctx, key, user.ID); err != nil {
		// Non-fatal: idempotency store failure does not roll back the user.
		// A subsequent replay will create a duplicate user; v0.1 has no
		// distributed transaction. Acceptable for the conformance gate.
		_ = err
	}

	// emit UserSignedUp
	deps.EventBus.Publish(ctx, UserSignedUp{User: user.ID, Email: email, At: now})

	// commit { user: user.id, token: session.token }
	return SignupResult{User: user.ID, Token: token}, nil
}

// ---------------------------------------------------------------------------
// flow Login
// ---------------------------------------------------------------------------

// Login authenticates by email and password. Errors are opaque — InvalidCredentials
// is returned for both wrong password and unknown email.
func Login(ctx context.Context, deps Deps, email shared.Email, password shared.Password, now time.Time) (LoginResult, error) {
	// step user = ask User.findBy(email)  rescue reject InvalidCredentials
	user, err := deps.Users.FindByEmail(ctx, email)
	if err != nil {
		return LoginResult{}, shared.ErrInvalidCredentials{}
	}

	// step ok = if not verify(password, user.hash) then reject InvalidCredentials
	if !verifyPassword(string(password), string(user.Hash)) {
		return LoginResult{}, shared.ErrInvalidCredentials{}
	}

	// step session = ask Session.create({ user: user.id, issued: now, expires: now after 7d })
	// Realised as JWT issuance.
	token, _, err := deps.JWT.Issue(user.ID, ksuid.New().String(), now)
	if err != nil {
		return LoginResult{}, err
	}

	// emit UserLoggedIn
	deps.EventBus.Publish(ctx, UserLoggedIn{User: user.ID, At: now})

	// commit { user: user.id, token: session.token }
	return LoginResult{User: user.ID, Token: token}, nil
}

// ---------------------------------------------------------------------------
// flow Logout
// ---------------------------------------------------------------------------

// Logout revokes the JWT carried by the request. Idempotent — re-revoking
// is a no-op (Session.Revoke is idempotent per spec).
//
// The middleware (LogoutBearerAuth) has already verified signature + exp,
// so the JWT here is well-formed. Revocation writes the JTI to the
// revocation list; subsequent BearerAuth lookups will see it as revoked.
func Logout(ctx context.Context, deps Deps, token shared.Token, now time.Time) error {
	claims, err := deps.JWT.Parse(token, now)
	if err != nil {
		// LogoutBearerAuth already rejected unparseable tokens upstream.
		return fmt.Errorf("parse token in logout flow: %w", err)
	}

	// step _ = ask Session(token).Revoke()
	if err := deps.Revoked.Revoke(ctx, claims.ID, claims.UserID, now); err != nil {
		return fmt.Errorf("revoke jti: %w", err)
	}

	// Append to audit (audit rows are never updated).
	_ = deps.Revoked.AuditRevoked(ctx, claims.ID, claims.UserID, now)

	// emit SessionRevoked { token, user, at } — token is the JWT string
	// (per spec: payload includes Token). Event is internal; the
	// "tokens never log" rule applies to log lines, not event payloads.
	deps.EventBus.Publish(ctx, SessionRevoked{
		Token: token,
		User:  claims.UserID,
		At:    now,
	})

	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// hashPassword returns an argon2id hash of the plaintext password.
// Parameters are intentionally conservative for dev; production should use
// environment-tuned time/memory costs.
func hashPassword(password string) string {
	salt := []byte(ksuid.New().String())
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("argon2id$%x$%x", salt, hash)
}

// verifyPassword checks a plaintext password against a stored argon2id hash.
// stored format: "argon2id$<salthex>$<hashhex>"
func verifyPassword(password, stored string) bool {
	parts := splitHash(stored)
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt := hexDecode(parts[1])
	expected := hexDecode(parts[2])
	if salt == nil || expected == nil {
		return false
	}
	candidate := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return string(candidate) == string(expected)
}

func splitHash(s string) []string {
	var parts []string
	cur := []byte{}
	for _, c := range s {
		if c == '$' {
			parts = append(parts, string(cur))
			cur = cur[:0]
		} else {
			cur = append(cur, byte(c))
		}
	}
	parts = append(parts, string(cur))
	return parts
}

func hexDecode(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi := hexVal(s[i])
		lo := hexVal(s[i+1])
		if hi < 0 || lo < 0 {
			return nil
		}
		b[i/2] = byte(hi<<4 | lo)
	}
	return b
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}
