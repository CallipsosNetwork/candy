// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
)

// SessionTTL is the JWT lifetime (spec: `expires: now after 7d`).
const SessionTTL = 7 * 24 * time.Hour

// Deps bundles auth dependencies needed by flows + middleware.
type Deps struct {
	Users   *UserRepo
	JWT     *JWTService
	Revoked *RevokedRepo
}

// hashPassword produces an argon2id hash of the plaintext password.
func hashPassword(p shared.Password) (shared.PasswordHash, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(string(p)), salt, 1, 64*1024, 4, 32)
	return shared.PasswordHash(hex.EncodeToString(salt) + "$" + hex.EncodeToString(hash)), nil
}

// verifyPassword checks a plaintext password against a stored hash.
func verifyPassword(p shared.Password, stored shared.PasswordHash) bool {
	parts := splitHash(string(stored))
	if len(parts) != 2 {
		return false
	}
	saltBytes, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	hashBytes := argon2.IDKey([]byte(string(p)), saltBytes, 1, 64*1024, 4, 32)
	return hex.EncodeToString(hashBytes) == parts[1]
}

func splitHash(s string) []string {
	idx := -1
	for i, c := range s {
		if c == '$' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	return []string{s[:idx], s[idx+1:]}
}

// SignupArgs matches the spec's Signup flow parameters.
type SignupArgs struct {
	Email    shared.Email
	Password shared.Password
	Now      time.Time
	Key      shared.Key
}

// SignupResult is returned on successful signup.
type SignupResult struct {
	UserID shared.Id
	Role   shared.Role
	Token  shared.Token
}

// Signup creates a User account and issues a JWT session.
func Signup(ctx context.Context, deps Deps, a SignupArgs) (SignupResult, error) {
	// step strength = PasswordStrength(password) rescue reject WeakPassword
	if err := PasswordStrength(a.Password); err != nil {
		return SignupResult{}, err
	}

	// step taken = if any u in User where u.email == email then reject EmailTaken
	taken, err := deps.Users.EmailExists(ctx, a.Email)
	if err != nil {
		return SignupResult{}, err
	}
	if taken {
		return SignupResult{}, shared.ErrEmailTaken
	}

	// step user = ask User.create(...)
	hash, err := hashPassword(a.Password)
	if err != nil {
		return SignupResult{}, err
	}
	user, err := deps.Users.Create(ctx, CreateUserArgs{
		ID:      GenerateID(),
		Email:   a.Email,
		Hash:    hash,
		Role:    shared.RoleUser,
		Created: a.Now,
	})
	if err != nil {
		return SignupResult{}, err
	}

	// step session = ask Session.create(...)  — realised as JWT issuance
	token, _, err := deps.JWT.Issue(user.ID, user.Role, GenerateJTI(), a.Now)
	if err != nil {
		return SignupResult{}, err
	}

	return SignupResult{UserID: user.ID, Role: user.Role, Token: token}, nil
}

// LoginArgs matches the spec's Login flow parameters.
type LoginArgs struct {
	Email    shared.Email
	Password shared.Password
	Now      time.Time
}

// LoginResult is returned on successful login.
type LoginResult struct {
	UserID shared.Id
	Role   shared.Role
	Token  shared.Token
}

// Login authenticates by email + password and issues a JWT session.
func Login(ctx context.Context, deps Deps, a LoginArgs) (LoginResult, error) {
	// step user = ask User.findBy(email) rescue reject InvalidCredentials
	user, err := deps.Users.FindByEmail(ctx, a.Email)
	if err != nil {
		return LoginResult{}, shared.ErrInvalidCredentials
	}

	// step ok = if not verify(password, user.hash) then reject InvalidCredentials
	if !verifyPassword(a.Password, user.Hash) {
		return LoginResult{}, shared.ErrInvalidCredentials
	}

	// step session = ask Session.create(...) — realised as JWT issuance
	token, _, err := deps.JWT.Issue(user.ID, user.Role, GenerateJTI(), a.Now)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{UserID: user.ID, Role: user.Role, Token: token}, nil
}

// Logout revokes the JWT carried by the request. Idempotent.
//
// The middleware (LogoutBearerAuth) has already verified signature + exp,
// so the token is well-formed. We re-parse here to extract the JTI;
// re-parsing is cheap and keeps Logout self-contained.
func Logout(ctx context.Context, deps Deps, token shared.Token, now time.Time) error {
	claims, err := deps.JWT.Parse(token, now)
	if err != nil {
		return shared.ErrSessionInvalid
	}
	return deps.Revoked.Revoke(ctx, claims.ID, shared.Id(claims.Subject), now)
}

// ValidateBearerToken validates a token and returns the user id + role.
// Returns ErrSessionInvalid on missing / malformed / expired / revoked.
func ValidateBearerToken(ctx context.Context, deps Deps, token shared.Token, now time.Time) (shared.Id, shared.Role, error) {
	if token == "" {
		return "", "", shared.ErrUnauthenticated
	}
	claims, err := deps.JWT.Parse(token, now)
	if err != nil {
		return "", "", shared.ErrSessionInvalid
	}
	revoked, err := deps.Revoked.IsRevoked(ctx, claims.ID)
	if err != nil {
		return "", "", err
	}
	if revoked {
		return "", "", shared.ErrSessionInvalid
	}
	return shared.Id(claims.Subject), claims.Role, nil
}
