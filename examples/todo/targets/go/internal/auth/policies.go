// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package auth

import (
	"strings"
	"unicode"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// passwordBlocklist contains common passwords that are unconditionally rejected.
var passwordBlocklist = map[string]bool{
	"password123":  true,
	"password1234": true,
	"abc123456789": true,
	"qwerty123456": true,
	"letmein12345": true,
	"welcome12345": true,
	"monkey123456": true,
	"dragon123456": true,
}

// PasswordStrength validates a plaintext password against the spec policy.
//
// Rules:
//   - length >= 12
//   - contains at least one letter and one digit
//   - not in common-password blocklist
//
// Examples from spec:
//   - "correct horse battery staple 9" → ok
//   - "short1"                         → err(TooShort)
//   - "alllowercase"                   → err(MissingDigit)
//   - "password123"                    → err(InBlocklist)
func PasswordStrength(p shared.Password) error {
	s := string(p)
	if passwordBlocklist[strings.ToLower(s)] {
		return &shared.WeakPasswordErr{Reason: shared.ErrInBlocklist}
	}
	if len(s) < 12 {
		return &shared.WeakPasswordErr{Reason: shared.ErrTooShort}
	}
	hasLetter := false
	hasDigit := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return &shared.WeakPasswordErr{Reason: shared.ErrMissingDigit}
	}
	return nil
}
