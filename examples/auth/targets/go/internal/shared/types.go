// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package shared

import (
	"fmt"
	"net/mail"
	"strings"
)

// Id is an opaque identifier (max 64 bytes).
type Id string

// Email is a validated RFC-5322 email address (max 320 chars).
type Email string

// NewEmail validates and returns an Email.
func NewEmail(s string) (Email, error) {
	if len(s) > 320 {
		return "", fmt.Errorf("email too long")
	}
	if _, err := mail.ParseAddress(s); err != nil {
		return "", fmt.Errorf("invalid email format: %w", err)
	}
	return Email(s), nil
}

// Password is a plaintext password value. Never persisted; only its hash is stored.
type Password string

// PasswordHash is an opaque hash blob.
type PasswordHash string

// Token is an opaque bearer token (max 256 bytes).
type Token string

// Key is an opaque idempotency key (max 128 bytes).
type Key string

// Ensure Key max constraint.
func (k Key) Validate() error {
	if len(k) > 128 {
		return fmt.Errorf("key too long (max 128)")
	}
	return nil
}

// WeakPasswordReason is the sub-reason for a WeakPassword error.
type WeakPasswordReason string

const (
	ReasonTooShort    WeakPasswordReason = "too_short"
	ReasonMissingDigit WeakPasswordReason = "missing_digit"
	ReasonInBlocklist  WeakPasswordReason = "in_blocklist"
)

// --- Sentinel errors ---

// ErrAlreadyVerified is returned when Session.Verify is called on an already-verified user.
type ErrAlreadyVerified struct{}

func (e ErrAlreadyVerified) Error() string { return "already_verified" }

// ErrSessionInvalid is returned when a session is revoked or expired.
type ErrSessionInvalid struct{}

func (e ErrSessionInvalid) Error() string { return "session_invalid" }

// ErrWeakPassword carries the specific reason a password was rejected.
type ErrWeakPassword struct {
	Reason WeakPasswordReason
}

func (e ErrWeakPassword) Error() string { return "weak_password: " + string(e.Reason) }

// ErrEmailTaken is returned when the email is already registered.
type ErrEmailTaken struct{}

func (e ErrEmailTaken) Error() string { return "email_taken" }

// ErrInvalidCredentials is the opaque login failure.
type ErrInvalidCredentials struct{}

func (e ErrInvalidCredentials) Error() string { return "invalid_credentials" }

// ErrNotFound is returned when an actor lookup fails.
type ErrNotFound struct{ Entity string }

func (e ErrNotFound) Error() string { return e.Entity + " not found" }

// commonPasswords is a minimal blocklist covering the PasswordStrength policy example.
var commonPasswords = map[string]struct{}{
	"password123":    {},
	"password1234":   {},
	"123456789012":   {},
	"qwerty12345678": {},
}

// IsBlocklisted returns true if the password appears in the blocklist.
func IsBlocklisted(p string) bool {
	_, ok := commonPasswords[strings.ToLower(p)]
	return ok
}
