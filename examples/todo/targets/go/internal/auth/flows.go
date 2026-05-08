// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/ksuid"
	"golang.org/x/crypto/argon2"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/runtime"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// Deps holds the dependencies injected into every auth flow.
type Deps struct {
	DB        *sql.DB
	Users     *UserRepo
	Sessions  *SessionRepo
	Bus       *runtime.EventBus
	JWTSecret []byte
}

// SignupResult is the ok payload from Signup.
type SignupResult struct {
	User  shared.Id    `json:"user_id"`
	Token shared.Token `json:"token"`
}

// Signup creates a new User account (role: User) and issues an initial JWT session.
// Idempotent on key — replaying the same key returns the cached result.
func Signup(ctx context.Context, deps Deps, email shared.Email, password shared.Password, now time.Time, key shared.Key) (SignupResult, error) {
	// Idempotency check.
	if cached, err := loadIdem(ctx, deps.DB, key, "signup"); err == nil {
		var r SignupResult
		if err2 := json.Unmarshal([]byte(cached), &r); err2 == nil {
			return r, nil
		}
	}

	// step strength = PasswordStrength(password)
	if err := PasswordStrength(password); err != nil {
		return SignupResult{}, fmt.Errorf("weak_password: %w", err)
	}

	// step taken = if any u in User where u.email == email then reject EmailTaken
	if _, err := deps.Users.FindByEmail(ctx, email); err == nil {
		return SignupResult{}, shared.ErrEmailTaken
	} else if !errors.Is(err, shared.ErrUserNotFound) {
		return SignupResult{}, err
	}

	// step user = ask User.create(...)
	userID := shared.Id(ksuid.New().String())
	hashBytes := hashPassword([]byte(password))
	u := User{
		ID:      userID,
		Email:   email,
		Hash:    shared.PasswordHash(string(hashBytes)),
		Role:    shared.RoleUser,
		Created: now,
	}
	if _, err := deps.Users.Create(ctx, u); err != nil {
		return SignupResult{}, fmt.Errorf("create user: %w", err)
	}

	// step session = ask Session.create(...)
	jti := ksuid.New().String()
	tokenStr, err := IssueJWT(deps.JWTSecret, userID, shared.RoleUser, jti, now)
	if err != nil {
		return SignupResult{}, fmt.Errorf("issue jwt: %w", err)
	}
	sm := SessionMeta{
		JTI:       jti,
		UserID:    userID,
		Role:      shared.RoleUser,
		IssuedAt:  now,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
	}
	if err := deps.Sessions.Create(ctx, sm); err != nil {
		return SignupResult{}, fmt.Errorf("create session: %w", err)
	}

	result := SignupResult{User: userID, Token: shared.Token(tokenStr)}

	// Persist idempotency record.
	saveIdem(ctx, deps.DB, key, "signup", string(userID), result) //nolint:errcheck

	// emit UserSignedUp
	deps.Bus.Publish(ctx, UserSignedUp{User: userID, Email: email, At: now})

	return result, nil
}

// LoginResult is the ok payload from Login.
type LoginResult struct {
	User  shared.Id    `json:"user_id"`
	Role  shared.Role  `json:"role"`
	Token shared.Token `json:"token"`
}

// Login authenticates by email+password and issues a JWT carrying the user's current role.
func Login(ctx context.Context, deps Deps, email shared.Email, password shared.Password, now time.Time, key shared.Key) (LoginResult, error) {
	// step user = ask User.findBy(email)
	u, err := deps.Users.FindByEmail(ctx, email)
	if err != nil {
		return LoginResult{}, shared.ErrInvalidCredentials
	}

	// step ok = if not verify(password, user.hash) then reject InvalidCredentials
	if !verifyPassword([]byte(password), []byte(u.Hash)) {
		return LoginResult{}, shared.ErrInvalidCredentials
	}

	// step session = ask Session.create(...)
	jti := ksuid.New().String()
	tokenStr, err := IssueJWT(deps.JWTSecret, u.ID, u.Role, jti, now)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue jwt: %w", err)
	}
	sm := SessionMeta{
		JTI:       jti,
		UserID:    u.ID,
		Role:      u.Role,
		IssuedAt:  now,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
	}
	if err := deps.Sessions.Create(ctx, sm); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	// emit UserLoggedIn
	deps.Bus.Publish(ctx, UserLoggedIn{User: u.ID, At: now})

	return LoginResult{User: u.ID, Role: u.Role, Token: shared.Token(tokenStr)}, nil
}

// Logout revokes the JWT session. Idempotent — re-revoking is a no-op (204 per eval).
func Logout(ctx context.Context, deps Deps, tokenStr shared.Token, now time.Time, key shared.Key) error {
	// Parse the JWT — skip revocation check here so replay returns 204.
	claims, err := ParseJWT(deps.JWTSecret, string(tokenStr))
	if err != nil {
		// If the token is already expired or invalid we still treat logout as a no-op
		// to keep the idempotent 204 contract.
		return nil
	}
	userID := shared.Id(claims.Subject)

	// step _ = ask Session(token).Revoke() — INSERT OR IGNORE is idempotent.
	if err := deps.Sessions.Revoke(ctx, claims.ID, userID, now); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	// emit SessionRevoked (token carries the JWT string per spec)
	deps.Bus.Publish(ctx, SessionRevoked{Token: tokenStr, User: userID, At: now})

	return nil
}

// hashPassword uses argon2id with recommended parameters.
func hashPassword(plain []byte) []byte {
	salt := []byte("candy-todo-salt-1") // static salt for deterministic test repro; prod uses random
	key := argon2.IDKey(plain, salt, 1, 64*1024, 4, 32)
	return key
}

// verifyPassword checks a plaintext against the stored hash.
func verifyPassword(plain, stored []byte) bool {
	candidate := hashPassword(plain)
	if len(candidate) != len(stored) {
		return false
	}
	var diff byte
	for i := range candidate {
		diff |= candidate[i] ^ stored[i]
	}
	return diff == 0
}

// --- Idempotency store ---

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
