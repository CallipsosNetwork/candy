// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/segmentio/ksuid"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// MountAuth registers all auth routes on r.
func MountAuth(r chi.Router, deps Deps) {
	// POST /signup — auth: none
	r.Post("/signup", handleSignup(deps))

	// POST /login — auth: none
	r.Post("/login", handleLogin(deps))

	// POST /logout — auth: bearer (LogoutBearerAuth skips revocation check)
	r.Group(func(r chi.Router) {
		r.Use(LogoutBearerAuth(deps.JWTSecret))
		r.Post("/logout", handleLogout(deps))
	})

	// POST /admin/users/:id/promote — auth: bearer + RoleGated(Admin)
	r.Group(func(r chi.Router) {
		r.Use(BearerAuthWithUsers(deps.JWTSecret, deps.Sessions, deps.Users))
		r.Use(RoleGated(shared.RoleAdmin))
		r.Post("/admin/users/{id}/promote", handlePromote(deps))
	})
}

func handleSignup(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		var body struct {
			Email          string `json:"email"`
			Password       string `json:"password"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		result, err := Signup(r.Context(), deps,
			shared.Email(body.Email),
			shared.Password(body.Password),
			now,
			shared.Key(body.IdempotencyKey),
		)
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrWeakPassword):
				var wpe *shared.WeakPasswordErr
				reason := "weak"
				if errors.As(err, &wpe) {
					reason = wpe.Reason.Error()
				}
				writeJSON(w, 422, map[string]string{"error": "weak_password", "reason": reason})
			case errors.Is(err, shared.ErrEmailTaken):
				writeJSON(w, 409, map[string]string{"error": "email_taken"})
			default:
				writeJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}

		// Bootstrap-admin: if FIRST_ADMIN_EMAIL matches this signup email and no
		// admin exists yet, promote this user to Admin and re-issue an Admin JWT.
		// This resolves the chicken-and-egg problem documented in todo.md §bootstrap-admin gap.
		firstAdminEmail := strings.TrimSpace(os.Getenv("FIRST_ADMIN_EMAIL"))
		if firstAdminEmail != "" && strings.EqualFold(firstAdminEmail, body.Email) {
			// Only promote if there is currently no Admin in the DB.
			var adminCount int
			_ = deps.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE role='Admin'`).Scan(&adminCount)
			if adminCount == 0 {
				_ = deps.Users.Promote(r.Context(), result.User, shared.RoleAdmin)
				// Re-issue JWT with Admin role.
				jti := ksuid.New().String()
				tokenStr, err2 := IssueJWT(deps.JWTSecret, result.User, shared.RoleAdmin, jti, now)
				if err2 == nil {
					sm := SessionMeta{
						JTI:       jti,
						UserID:    result.User,
						Role:      shared.RoleAdmin,
						IssuedAt:  now,
						ExpiresAt: now.Add(7 * 24 * time.Hour),
					}
					_ = deps.Sessions.Create(r.Context(), sm)
					result.Token = shared.Token(tokenStr)
				}
			}
		}

		writeJSON(w, 201, map[string]string{
			"user_id": string(result.User),
			"token":   string(result.Token),
		})
	}
}

func handleLogin(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		var body struct {
			Email          string `json:"email"`
			Password       string `json:"password"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		result, err := Login(r.Context(), deps,
			shared.Email(body.Email),
			shared.Password(body.Password),
			now,
			shared.Key(body.IdempotencyKey),
		)
		if err != nil {
			if errors.Is(err, shared.ErrInvalidCredentials) {
				writeJSON(w, 401, map[string]string{"error": "invalid_credentials"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": "internal"})
			return
		}
		writeJSON(w, 200, map[string]any{
			"user_id": string(result.User),
			"role":    string(result.Role),
			"token":   string(result.Token),
		})
	}
}

func handleLogout(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		var body struct {
			IdempotencyKey string `json:"idempotency_key"`
		}
		// body is optional-ish; parse best-effort
		_ = json.NewDecoder(r.Body).Decode(&body)

		tokenStr := GetRawToken(r.Context())
		_ = Logout(r.Context(), deps, tokenStr, now, shared.Key(body.IdempotencyKey))
		w.WriteHeader(204)
	}
}

func handlePromote(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body struct {
			Role           string `json:"role"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		toRole := shared.Role(body.Role)
		if toRole != shared.RoleAdmin && toRole != shared.RoleManager && toRole != shared.RoleUser {
			writeJSON(w, 422, map[string]string{"error": "invalid_role"})
			return
		}
		if err := deps.Users.Promote(r.Context(), shared.Id(id), toRole); err != nil {
			switch {
			case errors.Is(err, shared.ErrInvalidPromotion):
				writeJSON(w, 422, map[string]string{"error": "invalid_promotion"})
			case errors.Is(err, shared.ErrUserNotFound):
				writeJSON(w, 404, map[string]string{"error": "not_found"})
			default:
				writeJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}
		writeJSON(w, 200, map[string]bool{"promoted": true})
	}
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
