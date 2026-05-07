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
// policy examples:
//   - given "correct horse battery staple 9" → ok
//   - given "short1"                         → err(TooShort)
//   - given "alllowercase"                   → err(MissingDigit)
//   - given "password123"                    → err(InBlocklist)
func PasswordStrength(p shared.Password) error {
	s := string(p)
	if len(s) < 12 {
		return &shared.WeakPasswordErr{Reason: "too_short"}
	}
	hasDigit := false
	hasLetter := false
	for _, c := range s {
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
		}
	}
	if !hasDigit || !hasLetter {
		return &shared.WeakPasswordErr{Reason: "missing_digit"}
	}
	if commonPasswords[strings.ToLower(s)] {
		return &shared.WeakPasswordErr{Reason: "in_blocklist"}
	}
	return nil
}
