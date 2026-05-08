// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package auth

import (
	"unicode"

	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/shared"
)

// PasswordStrength validates a password per the spec policy:
//   - length >= 12
//   - contains at least one letter and one digit
//   - is not in the common-password blocklist
//
// Returns nil on success, *shared.ErrWeakPassword on failure.
//
// Policy attachment: flow scope (Signup calls it as the first step).
func PasswordStrength(p shared.Password) error {
	s := string(p)

	if shared.IsBlocklisted(s) {
		return shared.ErrWeakPassword{Reason: shared.ReasonInBlocklist}
	}

	if len(s) < 12 {
		return shared.ErrWeakPassword{Reason: shared.ReasonTooShort}
	}

	var hasLetter, hasDigit bool
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return shared.ErrWeakPassword{Reason: shared.ReasonMissingDigit}
	}

	return nil
}
