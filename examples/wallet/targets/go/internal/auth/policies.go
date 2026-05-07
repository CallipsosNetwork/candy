// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package auth

import (
	"strings"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
)

// commonPasswords is a minimal blocklist. Production should use a full list.
var commonPasswords = map[string]bool{
	"password123": true,
	"password1":   true,
	"12345678901": true,
}

// PasswordStrength validates a password per the spec policy:
//   - length >= 12
//   - contains at least one letter and one digit
//   - not in blocklist
//
// Spec examples:
//
//	"correct horse battery staple 9" → ok
//	"short1"                         → err(TooShort)
//	"alllowercase"                   → err(MissingDigit)
//	"password123"                    → err(InBlocklist)
//
// Blocklist is checked first: the spec example "password123" is 11 chars
// (below the length floor); checking length first would mask the
// InBlocklist reason. Same ordering used by the auth Go target (PR #45).
func PasswordStrength(p shared.Password) error {
	s := string(p)

	if commonPasswords[strings.ToLower(s)] {
		return &shared.WeakPasswordErr{Reason: "in_blocklist"}
	}

	if len(s) < 12 {
		return &shared.WeakPasswordErr{Reason: "too_short"}
	}

	hasDigit, hasLetter := false, false
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
			hasLetter = true
		}
	}
	if !hasLetter || !hasDigit {
		return &shared.WeakPasswordErr{Reason: "missing_digit"}
	}

	return nil
}
