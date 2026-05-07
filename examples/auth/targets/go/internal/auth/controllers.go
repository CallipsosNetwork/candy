// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/shared"
)

// contextKey is the package-local type for context keys.
type contextKey int

const (
	ctxKeyPrincipal contextKey = iota
	ctxKeyToken
)

// ---------------------------------------------------------------------------
// BearerAuth middleware
// ---------------------------------------------------------------------------
// Policy attachment: prose scope — applies to every authenticated controller route.

// BearerAuth validates a bearer token, sets principal id and raw token on context.
// Returns 401 on absent or invalid bearer.
func BearerAuth(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// now bound at route boundary.
			now := time.Now().UTC()

			hdr := r.Header.Get("Authorization")
			if hdr == "" {
				writeJSON(w, 401, map[string]string{"error": "missing_bearer"})
				return
			}
			if !strings.HasPrefix(hdr, "Bearer ") {
				writeJSON(w, 401, map[string]string{"error": "invalid_bearer"})
				return
			}
			rawToken := strings.TrimPrefix(hdr, "Bearer ")
			token := shared.Token(rawToken)

			// Session.Validate: returns user id if live, else SessionInvalid.
			userID, err := deps.Sessions.Validate(r.Context(), token, now)
			if err != nil {
				writeJSON(w, 401, map[string]string{"error": "invalid_bearer"})
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyPrincipal, userID)
			ctx = context.WithValue(ctx, ctxKeyToken, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LogoutBearerAuth is the middleware for POST /logout.
// Unlike the general BearerAuth, it allows revoked sessions through — logout is
// idempotent and the flow must be reachable even when the session is already
// revoked. It rejects only when no bearer header is present or the token is not
// a known session at all.
func LogoutBearerAuth(deps Deps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if hdr == "" {
				writeJSON(w, 401, map[string]string{"error": "missing_bearer"})
				return
			}
			if !strings.HasPrefix(hdr, "Bearer ") {
				writeJSON(w, 401, map[string]string{"error": "invalid_bearer"})
				return
			}
			rawToken := strings.TrimPrefix(hdr, "Bearer ")
			token := shared.Token(rawToken)

			// Check that the session exists (even if revoked or expired).
			// A completely unknown token (not in DB) is rejected with 401.
			session, err := deps.Sessions.FindByToken(r.Context(), token)
			if err != nil {
				writeJSON(w, 401, map[string]string{"error": "invalid_bearer"})
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyPrincipal, session.UserID)
			ctx = context.WithValue(ctx, ctxKeyToken, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MountAuth registers all auth routes on r.
func MountAuth(r chi.Router, deps Deps) {
	r.Post("/signup", handleSignup(deps))
	r.Post("/login", handleLogin(deps))

	r.Group(func(r chi.Router) {
		r.Use(LogoutBearerAuth(deps))
		r.Post("/logout", handleLogout(deps))
	})
}

// ---------------------------------------------------------------------------
// POST /signup -> Signup(email, password, now, idempotency_key)
// ---------------------------------------------------------------------------

func handleSignup(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()

		var body struct {
			Email          string `json:"email"`
			Password       string `json:"password"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := decode(r, &body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		email, err := shared.NewEmail(body.Email)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request", "detail": err.Error()})
			return
		}

		key := shared.Key(body.IdempotencyKey)
		if err := key.Validate(); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request", "detail": err.Error()})
			return
		}

		result, err := Signup(r.Context(), deps, email, shared.Password(body.Password), now, key)
		if err != nil {
			var we shared.ErrWeakPassword
			if errors.As(err, &we) {
				writeJSON(w, 422, map[string]string{
					"error":  "weak_password",
					"reason": string(we.Reason),
				})
				return
			}
			var et shared.ErrEmailTaken
			if errors.As(err, &et) {
				writeJSON(w, 409, map[string]string{"error": "email_taken"})
				return
			}
			slog.ErrorContext(r.Context(), "signup error", "err", err)
			writeJSON(w, 500, map[string]string{"error": "internal"})
			return
		}

		// map: ok(result) -> 201 { user_id: result.user, token: result.token }
		writeJSON(w, 201, map[string]string{
			"user_id": string(result.User),
			"token":   string(result.Token),
		})
	}
}

// ---------------------------------------------------------------------------
// POST /login -> Login(email, password, now)
// ---------------------------------------------------------------------------

func handleLogin(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := decode(r, &body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		email, err := shared.NewEmail(body.Email)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		result, err := Login(r.Context(), deps, email, shared.Password(body.Password), now)
		if err != nil {
			var ic shared.ErrInvalidCredentials
			if errors.As(err, &ic) {
				writeJSON(w, 401, map[string]string{"error": "invalid_credentials"})
				return
			}
			slog.ErrorContext(r.Context(), "login error", "err", err)
			writeJSON(w, 500, map[string]string{"error": "internal"})
			return
		}

		// map: ok(result) -> 200 { user_id: result.user, token: result.token }
		writeJSON(w, 200, map[string]string{
			"user_id": string(result.User),
			"token":   string(result.Token),
		})
	}
}

// ---------------------------------------------------------------------------
// POST /logout -> Logout(bearer, now)
// ---------------------------------------------------------------------------

func handleLogout(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()

		// bearer is set by BearerAuth middleware.
		token := tokenFromContext(r.Context())

		if err := Logout(r.Context(), deps, token, now); err != nil {
			slog.ErrorContext(r.Context(), "logout error", "err", err)
			writeJSON(w, 500, map[string]string{"error": "internal"})
			return
		}

		// map: ok(_) -> 204
		w.WriteHeader(204)
	}
}

// ---------------------------------------------------------------------------
// context helpers
// ---------------------------------------------------------------------------

func principalFromContext(ctx context.Context) shared.Id {
	if v := ctx.Value(ctxKeyPrincipal); v != nil {
		if id, ok := v.(shared.Id); ok {
			return id
		}
	}
	return ""
}

func tokenFromContext(ctx context.Context) shared.Token {
	if v := ctx.Value(ctxKeyToken); v != nil {
		if t, ok := v.(shared.Token); ok {
			return t
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode error", "err", err)
	}
}
