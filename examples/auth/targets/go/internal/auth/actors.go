// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/shared"
)

// ---------------------------------------------------------------------------
// actor User
// ---------------------------------------------------------------------------

// User is the persistent state of a user account.
type User struct {
	ID       shared.Id
	Email    shared.Email
	Hash     shared.PasswordHash
	Created  time.Time
	Verified bool
}

// UserRepo owns all reads and writes for the users table.
type UserRepo struct{ db *sql.DB }

// NewUserRepo constructs a UserRepo.
func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

// Create inserts a new User row. Enforces the unique-email invariant via the DB UNIQUE constraint.
func (r *UserRepo) Create(ctx context.Context, u User) (*User, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, hash, created, verified) VALUES (?, ?, ?, ?, ?)`,
		string(u.ID), string(u.Email), string(u.Hash),
		u.Created.UTC().Format(time.RFC3339Nano), boolToInt(u.Verified),
	)
	if err != nil {
		return nil, fmt.Errorf("user create: %w", err)
	}
	return &u, nil
}

// FindByEmail returns the user with the given email, or ErrNotFound.
func (r *UserRepo) FindByEmail(ctx context.Context, email shared.Email) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, email, hash, created, verified FROM users WHERE email = ?`, string(email))
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound{Entity: "user"}
	}
	return u, err
}

// FindByID returns the user with the given id, or ErrNotFound.
func (r *UserRepo) FindByID(ctx context.Context, id shared.Id) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, email, hash, created, verified FROM users WHERE id = ?`, string(id))
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound{Entity: "user"}
	}
	return u, err
}

// ExistsByEmail returns true if any user has the given email.
func (r *UserRepo) ExistsByEmail(ctx context.Context, email shared.Email) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, string(email)).Scan(&count)
	return count > 0, err
}

// Verify marks the user's email as verified. Enforces: verified == false precondition.
func (r *UserRepo) Verify(ctx context.Context, id shared.Id, now time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET verified = 1 WHERE id = ? AND verified = 0`, string(id))
	if err != nil {
		return fmt.Errorf("user verify: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return shared.ErrAlreadyVerified{}
	}
	return nil
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var idStr, emailStr, hashStr, createdStr string
	var verifiedInt int
	if err := row.Scan(&idStr, &emailStr, &hashStr, &createdStr, &verifiedInt); err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339Nano, createdStr)
	if err != nil {
		return nil, fmt.Errorf("parse created: %w", err)
	}
	u.ID = shared.Id(idStr)
	u.Email = shared.Email(emailStr)
	u.Hash = shared.PasswordHash(hashStr)
	u.Created = t.UTC()
	u.Verified = verifiedInt != 0
	return &u, nil
}

// ---------------------------------------------------------------------------
// actor Session
// ---------------------------------------------------------------------------

// Session is the persistent state of an auth session.
type Session struct {
	Token   shared.Token
	UserID  shared.Id
	Issued  time.Time
	Expires time.Time
	Revoked bool
}

// invariant: expires > issued (enforced at creation time in Create).

// SessionRepo owns all reads and writes for the sessions table.
type SessionRepo struct{ db *sql.DB }

// NewSessionRepo constructs a SessionRepo.
func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{db: db} }

// Create inserts a new Session. Enforces expires > issued invariant.
func (r *SessionRepo) Create(ctx context.Context, s Session) (*Session, error) {
	if !s.Expires.After(s.Issued) {
		return nil, fmt.Errorf("session invariant: expires must be after issued")
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, issued, expires, revoked) VALUES (?, ?, ?, ?, ?)`,
		string(s.Token), string(s.UserID),
		s.Issued.UTC().Format(time.RFC3339Nano),
		s.Expires.UTC().Format(time.RFC3339Nano),
		boolToInt(s.Revoked),
	)
	if err != nil {
		return nil, fmt.Errorf("session create: %w", err)
	}
	return &s, nil
}

// FindByToken returns the session with the given token, or ErrNotFound.
func (r *SessionRepo) FindByToken(ctx context.Context, token shared.Token) (*Session, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT token, user_id, issued, expires, revoked FROM sessions WHERE token = ?`, string(token))
	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound{Entity: "session"}
	}
	return s, err
}

// Validate returns the user id if the session is live, else SessionInvalid.
// Spec: require revoked == false; require now < expires.
func (r *SessionRepo) Validate(ctx context.Context, token shared.Token, now time.Time) (shared.Id, error) {
	s, err := r.FindByToken(ctx, token)
	if err != nil {
		return "", shared.ErrSessionInvalid{}
	}
	if s.Revoked {
		return "", shared.ErrSessionInvalid{}
	}
	if !now.Before(s.Expires) {
		return "", shared.ErrSessionInvalid{}
	}
	return s.UserID, nil
}

// Revoke marks the session as revoked. Idempotent — re-revoking is a no-op.
// Spec: "effect: revoked = true" — always succeeds, no precondition.
func (r *SessionRepo) Revoke(ctx context.Context, token shared.Token, now time.Time) (*Session, error) {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET revoked = 1 WHERE token = ?`, string(token))
	if err != nil {
		return nil, fmt.Errorf("session revoke: %w", err)
	}
	return r.FindByToken(ctx, token)
}

// AuditRevoked appends a row to the session_revoked audit table.
// Audit rows are never updated — append-only.
func (r *SessionRepo) AuditRevoked(ctx context.Context, token shared.Token, userID shared.Id, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_session_revoked (token, user_id, at) VALUES (?, ?, ?)`,
		string(token), string(userID), at.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func scanSession(row *sql.Row) (*Session, error) {
	var s Session
	var tokenStr, userStr, issuedStr, expiresStr string
	var revokedInt int
	if err := row.Scan(&tokenStr, &userStr, &issuedStr, &expiresStr, &revokedInt); err != nil {
		return nil, err
	}
	issued, err := time.Parse(time.RFC3339Nano, issuedStr)
	if err != nil {
		return nil, fmt.Errorf("parse issued: %w", err)
	}
	expires, err := time.Parse(time.RFC3339Nano, expiresStr)
	if err != nil {
		return nil, fmt.Errorf("parse expires: %w", err)
	}
	s.Token = shared.Token(tokenStr)
	s.UserID = shared.Id(userStr)
	s.Issued = issued.UTC()
	s.Expires = expires.UTC()
	s.Revoked = revokedInt != 0
	return &s, nil
}

// ---------------------------------------------------------------------------
// Idempotency store
// ---------------------------------------------------------------------------

// IdempotencyRepo persists signup idempotency records.
type IdempotencyRepo struct{ db *sql.DB }

// NewIdempotencyRepo constructs an IdempotencyRepo.
func NewIdempotencyRepo(db *sql.DB) *IdempotencyRepo { return &IdempotencyRepo{db: db} }

// FindSignup returns the cached result for the given key, or (nil, nil) if absent.
func (r *IdempotencyRepo) FindSignup(ctx context.Context, key shared.Key) (userID shared.Id, token shared.Token, found bool, err error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id, token FROM signup_idempotency WHERE key = ?`, string(key))
	var uid, tok string
	if err = row.Scan(&uid, &tok); errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return shared.Id(uid), shared.Token(tok), true, nil
}

// StoreSignup persists an idempotency result. Ignores conflicts (key already stored).
func (r *IdempotencyRepo) StoreSignup(ctx context.Context, key shared.Key, userID shared.Id, token shared.Token) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO signup_idempotency (key, user_id, token) VALUES (?, ?, ?)`,
		string(key), string(userID), string(token),
	)
	return err
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
