// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package auth

import (
	"errors"
	"testing"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
)

// TestPasswordStrength asserts every spec example.
//
//	"correct horse battery staple 9" → ok
//	"short1"                         → err(TooShort)
//	"alllowercase"                   → err(MissingDigit)
//	"password123"                    → err(InBlocklist)
func TestPasswordStrength(t *testing.T) {
	tests := []struct {
		name    string
		given   string
		wantErr bool
		reason  string
	}{
		{"strong passphrase ok", "correct horse battery staple 9", false, ""},
		{"too short", "short1", true, "too_short"},
		{"missing digit", "alllowercase", true, "missing_digit"},
		{"in blocklist", "password123", true, "in_blocklist"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := PasswordStrength(shared.Password(tc.given))
			if tc.wantErr {
				var we *shared.WeakPasswordErr
				if !errors.As(err, &we) {
					t.Fatalf("expected WeakPasswordErr, got %T: %v", err, err)
				}
				if we.Reason != tc.reason {
					t.Fatalf("expected reason %q, got %q", tc.reason, we.Reason)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}
