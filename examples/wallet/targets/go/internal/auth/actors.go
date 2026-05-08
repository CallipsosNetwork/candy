// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/segmentio/ksuid"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
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

// ---------------------------------------------------------------------------
// Session realisation — self-contained JWT
// ---------------------------------------------------------------------------
//
// The wallet.candy spec models Session as a stateful actor with
// id/user/issued/expires/revoked. Its prose pins the realisation:
// "Codegen targets JWT-signed sessions with argon2id password hashing
// and SQLite for dev." The auth.candy prose (which wallet inlines):
// "JWT semantics for production. No session-store lookup on the hot
// path; the JWT is self-contained. Revocation goes through a small …
// JWT claims." The two split cleanly:
//
//   user, issued, expires → JWT claims (sub, iat, exp).
//   role                  → JWT claim (role).
//   revoked: bool         → membership in the revoked_jtis table.

// SessionClaims is the JWT payload for an issued session.
type SessionClaims struct {
	Role shared.Role `json:"role"`
	jwt.RegisteredClaims
}

// JWTService signs and parses session JWTs.
type JWTService struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewJWTService constructs a JWTService.
func NewJWTService(secret []byte, issuer string, ttl time.Duration) *JWTService {
	return &JWTService{secret: secret, issuer: issuer, ttl: ttl}
}

// Issue signs a fresh JWT for the given user and role. Returns the
// encoded token plus the claims (callers may want the jti for audit).
func (s *JWTService) Issue(userID shared.Id, role shared.Role, jti string, now time.Time) (shared.Token, *SessionClaims, error) {
	claims := SessionClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   string(userID),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now.UTC()),
			ExpiresAt: jwt.NewNumericDate(now.UTC().Add(s.ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", nil, fmt.Errorf("sign jwt: %w", err)
	}
	return shared.Token(signed), &claims, nil
}

// Parse verifies the signature, checks exp, and returns the claims.
// Does NOT consult the revocation table — that is the caller's
// responsibility (see middleware).
func (s *JWTService) Parse(raw shared.Token, now time.Time) (*SessionClaims, error) {
	parsed, err := jwt.ParseWithClaims(string(raw), &SessionClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected alg: %s", t.Method.Alg())
		}
		return s.secret, nil
	}, jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil {
		return nil, shared.ErrSessionInvalid
	}
	claims, ok := parsed.Claims.(*SessionClaims)
	if !ok || !parsed.Valid {
		return nil, shared.ErrSessionInvalid
	}
	if claims.Subject == "" || claims.ID == "" {
		return nil, shared.ErrSessionInvalid
	}
	return claims, nil
}

// ---------------------------------------------------------------------------
// RevokedRepo — JWT revocation list
// ---------------------------------------------------------------------------

// RevokedRepo persists revoked JWT IDs. Revocation is idempotent —
// re-revoking the same JTI is a no-op via INSERT OR IGNORE.
type RevokedRepo struct{ db *sql.DB }

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

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// GenerateID returns a new KSUID-based Id.
func GenerateID() shared.Id { return shared.Id(ksuid.New().String()) }

// GenerateJTI returns a new KSUID for use as a JWT ID.
func GenerateJTI() string { return ksuid.New().String() }
