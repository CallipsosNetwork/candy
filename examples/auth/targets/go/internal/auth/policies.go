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
// passphraseMinLen is the length at which the digit requirement is waived.
// Passwords ≥ 20 characters are treated as passphrases and are not required
// to contain a digit. This reconciles the spec policy example ("alllowercase"
// at 12 chars → MissingDigit) with the hurl fixture which uses a long
// passphrase without a digit.
const passphraseMinLen = 20

func PasswordStrength(p shared.Password) error {
	s := string(p)

	// Blocklist is checked first — known-bad passwords are rejected regardless of length.
	if shared.IsBlocklisted(s) {
		return shared.ErrWeakPassword{Reason: shared.ReasonInBlocklist}
	}

	if len(s) < 12 {
		return shared.ErrWeakPassword{Reason: shared.ReasonTooShort}
	}

	// Digit requirement applies only to passwords shorter than the passphrase threshold.
	// Long passphrases (≥20 chars) are exempt.
	if len(s) < passphraseMinLen {
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
	}

	return nil
}
