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

// BearerAuth middleware: validates the Bearer token, sets user id + role on context.
// Rejects with 401 on missing/invalid/expired/revoked token.
func BearerAuth(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now().UTC()
			token := extractBearer(r)
			userID, role, err := ValidateBearerToken(r.Context(), deps, shared.Token(token), now)
			if err != nil {
				respondJSON(w, 401, map[string]string{"error": "unauthenticated"})
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserID, userID)
			ctx = context.WithValue(ctx, ctxRole, role)
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
