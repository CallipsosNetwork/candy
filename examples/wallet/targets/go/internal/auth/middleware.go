// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
)

type contextKey int

const (
	ctxUserID contextKey = iota
	ctxRole
	ctxToken
)

// UserIDFromCtx extracts the authenticated user id from the request context.
func UserIDFromCtx(ctx context.Context) (shared.Id, bool) {
	v, ok := ctx.Value(ctxUserID).(shared.Id)
	return v, ok
}

// RoleFromCtx extracts the authenticated role from the request context.
func RoleFromCtx(ctx context.Context) (shared.Role, bool) {
	v, ok := ctx.Value(ctxRole).(shared.Role)
	return v, ok
}

// TokenFromCtx extracts the raw bearer token from the request context.
// Used by the logout handler to know which JTI to revoke.
func TokenFromCtx(ctx context.Context) (shared.Token, bool) {
	v, ok := ctx.Value(ctxToken).(shared.Token)
	return v, ok
}

// BearerAuth — parses + verifies signature + checks exp + checks revocation.
// Default for every authenticated route. Sets user id, role, raw token on
// context.
func BearerAuth(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now().UTC()
			token := shared.Token(extractBearer(r))
			userID, role, err := ValidateBearerToken(r.Context(), deps, token, now)
			if err != nil {
				respondJSON(w, 401, map[string]string{"error": "unauthenticated"})
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			ctx = context.WithValue(ctx, ctxRole, role)
			ctx = context.WithValue(ctx, ctxToken, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LogoutBearerAuth — parses + verifies signature + checks exp; intentionally
// SKIPS revocation. Logout is idempotent: re-sending a revoked JWT must
// reach the Logout flow (which is itself idempotent), not be 401-ed by the
// middleware. Malformed / unsigned / expired tokens are still rejected.
func LogoutBearerAuth(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now().UTC()
			raw := extractBearer(r)
			if raw == "" {
				respondJSON(w, 401, map[string]string{"error": "unauthenticated"})
				return
			}
			token := shared.Token(raw)
			claims, err := deps.JWT.Parse(token, now)
			if err != nil {
				respondJSON(w, 401, map[string]string{"error": "unauthenticated"})
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, shared.Id(claims.Subject))
			ctx = context.WithValue(ctx, ctxRole, claims.Role)
			ctx = context.WithValue(ctx, ctxToken, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
