// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package auth

import (
	"errors"
	"testing"

	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/shared"
)

// TestPasswordStrength tests every example declared in the spec's policy block.
func TestPasswordStrength(t *testing.T) {
	tests := []struct {
		name    string
		given   string
		wantErr bool
		reason  shared.WeakPasswordReason
	}{
		// given: "correct horse battery staple 9" → ok
		{
			name:    "strong password ok",
			given:   "correct horse battery staple 9",
			wantErr: false,
		},
		// given: "short1" → err(TooShort)
		{
			name:    "too short",
			given:   "short1",
			wantErr: true,
			reason:  shared.ReasonTooShort,
		},
		// given: "alllowercase" → err(MissingDigit)
		{
			name:    "missing digit",
			given:   "alllowercase",
			wantErr: true,
			reason:  shared.ReasonMissingDigit,
		},
		// given: "password123" → err(InBlocklist)
		{
			name:    "in blocklist",
			given:   "password123",
			wantErr: true,
			reason:  shared.ReasonInBlocklist,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := PasswordStrength(shared.Password(tc.given))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				var we shared.ErrWeakPassword
				if !errors.As(err, &we) {
					t.Fatalf("expected ErrWeakPassword, got %T: %v", err, err)
				}
				if we.Reason != tc.reason {
					t.Fatalf("expected reason %q, got %q", tc.reason, we.Reason)
				}
			} else {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			}
		})
	}
}
