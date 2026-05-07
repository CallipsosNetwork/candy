// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
	"github.com/segmentio/ksuid"
)

// UserRepo manages the users table.
type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

type User struct {
	ID      shared.Id
	Email   shared.Email
	Hash    shared.PasswordHash
	Role    shared.Role
	Created time.Time
}

type CreateUserArgs struct {
	ID      shared.Id
	Email   shared.Email
	Hash    shared.PasswordHash
	Role    shared.Role
	Created time.Time
}

func (r *UserRepo) Create(ctx context.Context, a CreateUserArgs) (*User, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users(id, email, hash, role, created) VALUES(?,?,?,?,?)`,
		string(a.ID), string(a.Email), string(a.Hash), string(a.Role), a.Created.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: a.ID, Email: a.Email, Hash: a.Hash, Role: a.Role, Created: a.Created}, nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email shared.Email) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, email, hash, role, created FROM users WHERE email=?`, string(email))
	return scanUser(row)
}

func (r *UserRepo) FindByID(ctx context.Context, id shared.Id) (*User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, email, hash, role, created FROM users WHERE id=?`, string(id))
	return scanUser(row)
}

func (r *UserRepo) UpdateRole(ctx context.Context, id shared.Id, role shared.Role) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET role=? WHERE id=?`, string(role), string(id))
	return err
}

func (r *UserRepo) EmailExists(ctx context.Context, email shared.Email) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email=?`, string(email)).Scan(&n)
	return n > 0, err
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var id, email, hash, role, created string
	err := row.Scan(&id, &email, &hash, &role, &created)
	if err == sql.ErrNoRows {
		return nil, shared.ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	t, _ := time.Parse(time.RFC3339, created)
	u.ID = shared.Id(id)
	u.Email = shared.Email(email)
	u.Hash = shared.PasswordHash(hash)
	u.Role = shared.Role(role)
	u.Created = t
	return &u, nil
}

// SessionRepo manages the sessions table.
type SessionRepo struct{ db *sql.DB }

func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{db: db} }

type Session struct {
	Token   shared.Token
	UserID  shared.Id
	Role    shared.Role
	Issued  time.Time
	Expires time.Time
	Revoked bool
}

type CreateSessionArgs struct {
	Token   shared.Token
	UserID  shared.Id
	Role    shared.Role
	Issued  time.Time
	Expires time.Time
}

func (r *SessionRepo) Create(ctx context.Context, a CreateSessionArgs) (*Session, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions(token, user_id, role, issued, expires, revoked) VALUES(?,?,?,?,?,0)`,
		string(a.Token), string(a.UserID), string(a.Role),
		a.Issued.UTC().Format(time.RFC3339), a.Expires.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &Session{Token: a.Token, UserID: a.UserID, Role: a.Role, Issued: a.Issued, Expires: a.Expires}, nil
}

func (r *SessionRepo) FindByToken(ctx context.Context, token shared.Token) (*Session, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT token, user_id, role, issued, expires, revoked FROM sessions WHERE token=?`, string(token))
	var s Session
	var tok, uid, role, issued, expires string
	var revoked int
	err := row.Scan(&tok, &uid, &role, &issued, &expires, &revoked)
	if err == sql.ErrNoRows {
		return nil, shared.ErrSessionInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	tIssued, _ := time.Parse(time.RFC3339, issued)
	tExpires, _ := time.Parse(time.RFC3339, expires)
	s.Token = shared.Token(tok)
	s.UserID = shared.Id(uid)
	s.Role = shared.Role(role)
	s.Issued = tIssued
	s.Expires = tExpires
	s.Revoked = revoked == 1
	return &s, nil
}

func (r *SessionRepo) Revoke(ctx context.Context, token shared.Token) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET revoked=1 WHERE token=?`, string(token))
	return err
}

// GenerateID returns a new KSUID-based Id.
func GenerateID() shared.Id {
	return shared.Id(ksuid.New().String())
}

// GenerateToken returns a new KSUID-based Token.
func GenerateToken() shared.Token {
	return shared.Token(ksuid.New().String())
}
