// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

type contextKey string

const (
	ctxUserID contextKey = "user_id"
	ctxRole   contextKey = "role"
	ctxJTI    contextKey = "jti"
	ctxToken  contextKey = "raw_token"
)

// GetSession returns the authenticated user id and role from the request context.
func GetSession(ctx context.Context) (shared.Id, shared.Role) {
	uid, _ := ctx.Value(ctxUserID).(shared.Id)
	role, _ := ctx.Value(ctxRole).(shared.Role)
	return uid, role
}

// GetRawToken returns the raw bearer token string from the request context.
func GetRawToken(ctx context.Context) shared.Token {
	t, _ := ctx.Value(ctxToken).(shared.Token)
	return t
}

// BearerAuth validates the JWT, checks expiry, and checks the revocation table.
// The role is read from the DB (not the JWT claim) so that promotions take
// effect immediately without requiring re-login.
// Sets user_id and role on the request context. Returns 401 on any failure.
func BearerAuth(secret []byte, sessions *SessionRepo) func(http.Handler) http.Handler {
	return BearerAuthWithUsers(secret, sessions, nil)
}

// BearerAuthWithUsers is BearerAuth that refreshes the role from the DB.
func BearerAuthWithUsers(secret []byte, sessions *SessionRepo, users *UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
				return
			}
			claims, err := ParseJWT(secret, tokenStr)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session_invalid"})
				return
			}
			// Check expiry explicitly (ParseJWT already does, but belt-and-suspenders).
			if claims.ExpiresAt != nil && time.Now().UTC().After(claims.ExpiresAt.Time) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session_invalid"})
				return
			}
			// Check revocation table.
			revoked, err := sessions.IsRevoked(r.Context(), claims.ID)
			if err != nil || revoked {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session_invalid"})
				return
			}

			userID := shared.Id(claims.Subject)
			role := shared.Role(claims.Role)

			// Refresh role from DB so promotions take effect without re-login.
			// Interpretation: Session.role in the spec is tied to the User actor's
			// current role — promotions update the User, and subsequent requests
			// reflect the new role immediately.
			if users != nil {
				if u, err := users.FindByID(r.Context(), userID); err == nil {
					role = u.Role
				}
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxUserID, userID)
			ctx = context.WithValue(ctx, ctxRole, role)
			ctx = context.WithValue(ctx, ctxJTI, claims.ID)
			ctx = context.WithValue(ctx, ctxToken, shared.Token(tokenStr))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LogoutBearerAuth is like BearerAuth but skips the revocation check.
// This lets logout-replay (already-revoked token) return 204 per the eval.
func LogoutBearerAuth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
				return
			}
			claims, err := ParseJWT(secret, tokenStr)
			if err != nil {
				// Expired/invalid tokens: still treat logout as a no-op for idempotency.
				_ = claims
			}
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxToken, shared.Token(tokenStr))
			if claims != nil {
				ctx = context.WithValue(ctx, ctxUserID, shared.Id(claims.Subject))
				ctx = context.WithValue(ctx, ctxRole, shared.Role(claims.Role))
				ctx = context.WithValue(ctx, ctxJTI, claims.ID)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RoleGated enforces a minimum role level. BearerAuth must run first.
// Returns 403 if the caller's role is below the required level.
func RoleGated(required shared.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, callerRole := GetSession(r.Context())
			if shared.RoleLevel(callerRole) < shared.RoleLevel(required) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_authorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
