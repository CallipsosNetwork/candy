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

	jwt "github.com/golang-jwt/jwt/v5"

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
// actor Session — realised as a self-contained JWT
// ---------------------------------------------------------------------------
//
// The auth.candy spec models Session as a stateful actor with id+user+
// issued+expires+revoked. The same spec's prose pins the realisation:
// "Token is opaque to the spec but is realized as a JWT by codegen…
// JWT semantics for production. No session-store lookup on the hot path;
// the JWT is self-contained. Revocation goes through a small … JWT
// claims."
//
// The Session actor's persistent state therefore splits:
//
//   user, issued, expires → encoded in the JWT claims (sub, iat, exp).
//   revoked: bool         → realised by presence in the revoked_jtis table.
//
// `Session.Revoke()` writes the JTI to that table; `Session.Validate()`
// parses the JWT and consults the revocation table.

// SessionClaims is the JWT payload for an issued session.
type SessionClaims struct {
	UserID shared.Id `json:"-"`
	jwt.RegisteredClaims
}

// JWTService signs and parses session JWTs.
type JWTService struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewJWTService constructs a JWTService. ttl is the session lifetime
// (the spec says `expires: now after 7d`).
func NewJWTService(secret []byte, issuer string, ttl time.Duration) *JWTService {
	return &JWTService{secret: secret, issuer: issuer, ttl: ttl}
}

// Issue signs a fresh JWT for userID. Returns the encoded token plus the
// issued claims (caller wants jti for the idempotency record).
func (s *JWTService) Issue(userID shared.Id, jti string, now time.Time) (shared.Token, *SessionClaims, error) {
	claims := &SessionClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   string(userID),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now.UTC()),
			ExpiresAt: jwt.NewNumericDate(now.UTC().Add(s.ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims.RegisteredClaims)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", nil, fmt.Errorf("sign jwt: %w", err)
	}
	return shared.Token(signed), claims, nil
}

// Parse verifies the signature, checks exp, and returns the claims.
// Does NOT consult the revocation table — callers that need that check
// must do it explicitly via RevokedRepo.
func (s *JWTService) Parse(raw shared.Token, now time.Time) (*SessionClaims, error) {
	parsed, err := jwt.ParseWithClaims(string(raw), &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected alg: %s", t.Method.Alg())
		}
		return s.secret, nil
	}, jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil {
		return nil, shared.ErrSessionInvalid{}
	}
	rc, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return nil, shared.ErrSessionInvalid{}
	}
	if rc.Subject == "" || rc.ID == "" {
		return nil, shared.ErrSessionInvalid{}
	}
	return &SessionClaims{
		UserID:           shared.Id(rc.Subject),
		RegisteredClaims: *rc,
	}, nil
}

// ---------------------------------------------------------------------------
// RevokedRepo — the small revocation list backing JWT revocation
// ---------------------------------------------------------------------------

// RevokedRepo persists revoked JWT IDs. Revocation is idempotent —
// re-revoking the same JTI is a no-op via INSERT OR IGNORE.
type RevokedRepo struct{ db *sql.DB }

// NewRevokedRepo constructs a RevokedRepo.
func NewRevokedRepo(db *sql.DB) *RevokedRepo { return &RevokedRepo{db: db} }

// Revoke inserts the JTI into the revocation table. Idempotent.
func (r *RevokedRepo) Revoke(ctx context.Context, jti string, userID shared.Id, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO revoked_jtis (jti, user_id, revoked_at) VALUES (?, ?, ?)`,
		jti, string(userID), at.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("revoke jti: %w", err)
	}
	return nil
}

// IsRevoked returns true if the JTI is in the revocation list.
func (r *RevokedRepo) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM revoked_jtis WHERE jti = ?`, jti).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check revoked: %w", err)
	}
	return count > 0, nil
}

// AuditRevoked appends a row to the session_revoked audit table.
// Audit rows are never updated — append-only.
func (r *RevokedRepo) AuditRevoked(ctx context.Context, jti string, userID shared.Id, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_session_revoked (jti, user_id, at) VALUES (?, ?, ?)`,
		jti, string(userID), at.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// ---------------------------------------------------------------------------
// Idempotency store
// ---------------------------------------------------------------------------

// IdempotencyRepo persists signup idempotency records.
type IdempotencyRepo struct{ db *sql.DB }

// NewIdempotencyRepo constructs an IdempotencyRepo.
func NewIdempotencyRepo(db *sql.DB) *IdempotencyRepo { return &IdempotencyRepo{db: db} }

// FindSignup returns the cached user id for the given key, or (id, false, nil).
func (r *IdempotencyRepo) FindSignup(ctx context.Context, key shared.Key) (userID shared.Id, found bool, err error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id FROM signup_idempotency WHERE key = ?`, string(key))
	var uid string
	if err = row.Scan(&uid); errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return shared.Id(uid), true, nil
}

// StoreSignup persists an idempotency result. Ignores conflicts (key already stored).
func (r *IdempotencyRepo) StoreSignup(ctx context.Context, key shared.Key, userID shared.Id) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO signup_idempotency (key, user_id) VALUES (?, ?)`,
		string(key), string(userID),
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
