// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// User is the persistent state of a User actor.
type User struct {
	ID       shared.Id
	Email    shared.Email
	Hash     shared.PasswordHash
	Role     shared.Role
	Verified bool
	Created  time.Time
}

// UserRepo owns reads/writes for the users table.
type UserRepo struct {
	DB *sql.DB
}

func (r *UserRepo) Create(ctx context.Context, u User) (*User, error) {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO users(id,email,hash,role,verified,created_at) VALUES(?,?,?,?,?,?)`,
		string(u.ID), string(u.Email), string(u.Hash),
		string(u.Role), boolInt(u.Verified), u.Created.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email shared.Email) (*User, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT id,email,hash,role,verified,created_at FROM users WHERE email=?`, string(email))
	return scanUser(row)
}

func (r *UserRepo) FindByID(ctx context.Context, id shared.Id) (*User, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT id,email,hash,role,verified,created_at FROM users WHERE id=?`, string(id))
	return scanUser(row)
}

// Promote changes the user's role. Guards: Admin → User is rejected (InvalidPromotion).
func (r *UserRepo) Promote(ctx context.Context, id shared.Id, to shared.Role) error {
	u, err := r.FindByID(ctx, id)
	if err != nil {
		return shared.ErrUserNotFound
	}
	// invariant: not (role == Admin and to == User)
	if u.Role == shared.RoleAdmin && to == shared.RoleUser {
		return shared.ErrInvalidPromotion
	}
	_, err = r.DB.ExecContext(ctx, `UPDATE users SET role=? WHERE id=?`, string(to), string(id))
	return err
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var id, email, hash, role, created string
	var verified int
	err := row.Scan(&id, &email, &hash, &role, &verified, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.ID = shared.Id(id)
	u.Email = shared.Email(email)
	u.Hash = shared.PasswordHash(hash)
	u.Role = shared.Role(role)
	u.Verified = verified != 0
	u.Created, _ = time.Parse(time.RFC3339, created)
	return &u, nil
}

// Session is the metadata stored server-side for a JWT-backed session.
// The JWT is the bearer token; revocation is tracked in revoked_jtis.
type SessionMeta struct {
	JTI       string
	UserID    shared.Id
	Role      shared.Role
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// SessionRepo manages the sessions and revoked_jtis tables.
type SessionRepo struct {
	DB *sql.DB
}

func (r *SessionRepo) Create(ctx context.Context, s SessionMeta) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO sessions(jti,user_id,role,issued_at,expires_at) VALUES(?,?,?,?,?)`,
		s.JTI, string(s.UserID), string(s.Role),
		s.IssuedAt.UTC().Format(time.RFC3339),
		s.ExpiresAt.UTC().Format(time.RFC3339),
	)
	return err
}

// Revoke inserts the JTI into revoked_jtis. Idempotent via INSERT OR IGNORE.
func (r *SessionRepo) Revoke(ctx context.Context, jti string, userID shared.Id, now time.Time) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT OR IGNORE INTO revoked_jtis(jti,user_id,revoked_at) VALUES(?,?,?)`,
		jti, string(userID), now.UTC().Format(time.RFC3339),
	)
	return err
}

// IsRevoked returns true if the JTI is in the revocation table.
func (r *SessionRepo) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var count int
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM revoked_jtis WHERE jti=?`, jti).Scan(&count)
	return count > 0, err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
