// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"time"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
	"golang.org/x/crypto/argon2"
	"crypto/rand"
	"encoding/hex"
)

// Deps bundles auth repositories needed by flows.
type Deps struct {
	Users    *UserRepo
	Sessions *SessionRepo
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
	Token  shared.Token
}

// Signup creates a User account and issues a session. Idempotent on key.
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

	// Also create a wallet for this user.
	// Wallet creation is handled in the wallet package, but we need the wallet to
	// exist. This is wired at startup via the wallet.Deps passed to the full server.
	// For auth-only flows, wallet creation is done post-signup in the controller.

	// step session = ask Session.create(...)
	session, err := deps.Sessions.Create(ctx, CreateSessionArgs{
		Token:   GenerateToken(),
		UserID:  user.ID,
		Role:    shared.RoleUser,
		Issued:  a.Now,
		Expires: a.Now.Add(7 * 24 * time.Hour),
	})
	if err != nil {
		return SignupResult{}, err
	}

	return SignupResult{UserID: user.ID, Token: session.Token}, nil
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

// Login authenticates by email + password and issues a session.
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

	// step session = ask Session.create(...)
	session, err := deps.Sessions.Create(ctx, CreateSessionArgs{
		Token:   GenerateToken(),
		UserID:  user.ID,
		Role:    user.Role,
		Issued:  a.Now,
		Expires: a.Now.Add(7 * 24 * time.Hour),
	})
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{UserID: user.ID, Role: user.Role, Token: session.Token}, nil
}

// Logout revokes the session for the given token. Idempotent.
func Logout(ctx context.Context, deps Deps, token shared.Token) error {
	// step _ = ask Session(token).Revoke()
	// Revoke is idempotent — UPDATE sets revoked=1 regardless of prior state.
	return deps.Sessions.Revoke(ctx, token)
}

// ValidateBearerToken validates a token and returns the user id + role.
// Returns ErrSessionInvalid if the token is missing, expired, or revoked.
// Returns ErrUnauthenticated if the token is empty.
func ValidateBearerToken(ctx context.Context, deps Deps, token shared.Token, now time.Time) (shared.Id, shared.Role, error) {
	if token == "" {
		return "", "", shared.ErrUnauthenticated
	}
	session, err := deps.Sessions.FindByToken(ctx, token)
	if err != nil {
		return "", "", shared.ErrSessionInvalid
	}
	if session.Revoked {
		return "", "", shared.ErrSessionInvalid
	}
	if now.After(session.Expires) {
		return "", "", shared.ErrSessionInvalid
	}
	return session.UserID, session.Role, nil
}
