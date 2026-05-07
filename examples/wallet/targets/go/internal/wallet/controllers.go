// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package wallet

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/auth"
	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
)

// FullDeps bundles all dependencies for the wallet controllers.
type FullDeps struct {
	Auth   auth.Deps
	Wallet Deps
}

// adminEmail returns the configured admin email from the ADMIN_EMAIL env var,
// defaulting to the fixture value used by the wallet hurl eval.
func adminEmail() shared.Email {
	if e := os.Getenv("ADMIN_EMAIL"); e != "" {
		return shared.Email(e)
	}
	return "admin@candy.local"
}

// Mount wires all Wallets controller routes onto r.
// BearerAuth is applied to all routes except /signup and /login.
func Mount(r chi.Router, deps FullDeps) {
	// Auth routes — no bearer required.
	r.Post("/signup", handleSignup(deps))
	r.Post("/login", handleLogin(deps))

	// Bearer-protected routes.
	r.Group(func(r chi.Router) {
		r.Use(auth.BearerAuth(deps.Auth))

		r.Post("/logout", handleLogout(deps))

		// Admin routes (AdminGated policy applied inside each handler).
		r.Post("/admin/wallets/{owner}/fund", handleFundWallet(deps))
		r.Post("/admin/users/{id}/promote", handlePromote(deps))

		// Wallet reads.
		r.Get("/wallets/me", handleGetBalance(deps))
		r.Get("/wallets/me/journal", handleGetJournal(deps))

		// Wallet writes.
		r.Post("/wallets/me/withdraw", handleWithdraw(deps))

		// Transfers.
		r.Post("/transfers", handleTransfer(deps))
		r.Post("/transfers/schedule", handleScheduleTransfer(deps))
		r.Post("/transfers/schedule/{id}/cancel", handleCancelSchedule(deps))
		r.Get("/transfers/schedule", handleListSchedules(deps))
	})
}

// --- Handlers -----------------------------------------------------------------

func handleSignup(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		var body struct {
			Email          shared.Email    `json:"email"`
			Password       shared.Password `json:"password"`
			IdempotencyKey shared.Key      `json:"idempotency_key"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		result, err := auth.Signup(r.Context(), deps.Auth, auth.SignupArgs{
			Email:    body.Email,
			Password: body.Password,
			Now:      now,
			Key:      body.IdempotencyKey,
		})
		if err != nil {
			var wpErr *shared.WeakPasswordErr
			if errors.As(err, &wpErr) {
				respondJSON(w, 422, map[string]string{"error": "weak_password", "reason": wpErr.Reason})
				return
			}
			if errors.Is(err, shared.ErrEmailTaken) {
				respondJSON(w, 409, map[string]string{"error": "email_taken"})
				return
			}
			respondJSON(w, 500, map[string]string{"error": "internal"})
			return
		}

		// Create a wallet for the new user.
		_ = deps.Wallet.Wallets.Create(r.Context(), result.UserID, now)

		// If this is the designated admin email, auto-promote to Admin.
		// This is the "backend seeding" approach described in wallet.md — the admin
		// account is created via signup (role User) and immediately elevated to Admin
		// so that login returns role Admin on the very first login.
		if body.Email == adminEmail() {
			_ = deps.Auth.Users.UpdateRole(r.Context(), result.UserID, shared.RoleAdmin)
		}

		respondJSON(w, 201, map[string]any{
			"user_id": result.UserID,
			"token":   result.Token,
		})
	}
}

func handleLogin(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		var body struct {
			Email    shared.Email    `json:"email"`
			Password shared.Password `json:"password"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		result, err := auth.Login(r.Context(), deps.Auth, auth.LoginArgs{
			Email:    body.Email,
			Password: body.Password,
			Now:      now,
		})
		if err != nil {
			respondJSON(w, 401, map[string]string{"error": "invalid_credentials"})
			return
		}

		respondJSON(w, 200, map[string]any{
			"user_id": result.UserID,
			"role":    result.Role,
			"token":   result.Token,
		})
	}
}

func handleLogout(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		// BearerAuth set the raw token on context.
		token, _ := auth.TokenFromCtx(r.Context())
		_ = auth.Logout(r.Context(), deps.Auth, token, now)
		w.WriteHeader(204)
	}
}

func handleFundWallet(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()

		// AdminGated policy: caller must have Admin role.
		role, _ := auth.RoleFromCtx(r.Context())
		if role != shared.RoleAdmin {
			respondJSON(w, 403, map[string]string{"error": "not_authorized"})
			return
		}

		callerID, _ := auth.UserIDFromCtx(r.Context())
		owner := shared.Id(chi.URLParam(r, "owner"))

		var body struct {
			Amount         shared.Money `json:"amount"`
			IdempotencyKey shared.Key   `json:"idempotency_key"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		entry, err := FundWallet(r.Context(), deps.Wallet, FundWalletArgs{
			Wallet: owner,
			Amount: body.Amount,
			By:     callerID,
			Now:    now,
			Key:    body.IdempotencyKey,
		})
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrInvalidAmount):
				respondJSON(w, 422, map[string]string{"error": "invalid_amount"})
			case errors.Is(err, shared.ErrWalletNotFound):
				respondJSON(w, 404, map[string]string{"error": "wallet_not_found"})
			case errors.Is(err, shared.ErrNotAuthorized):
				respondJSON(w, 403, map[string]string{"error": "not_authorized"})
			default:
				respondJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}

		respondJSON(w, 201, map[string]any{"entry": entry})
	}
}

func handlePromote(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// AdminGated policy: caller must have Admin role.
		role, _ := auth.RoleFromCtx(r.Context())
		if role != shared.RoleAdmin {
			respondJSON(w, 403, map[string]string{"error": "not_authorized"})
			return
		}

		userID := shared.Id(chi.URLParam(r, "id"))

		var body struct {
			Role shared.Role `json:"role"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		if err := deps.Auth.Users.UpdateRole(r.Context(), userID, body.Role); err != nil {
			respondJSON(w, 500, map[string]string{"error": "internal"})
			return
		}

		respondJSON(w, 200, map[string]any{"promoted": true})
	}
}

func handleGetBalance(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromCtx(r.Context())
		balance, err := deps.Wallet.Wallets.Balance(r.Context(), userID)
		if err != nil {
			respondJSON(w, 404, map[string]string{"error": "wallet_not_found"})
			return
		}
		respondJSON(w, 200, map[string]any{"balance": balance})
	}
}

func handleGetJournal(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := auth.UserIDFromCtx(r.Context())
		entries, err := deps.Wallet.Wallets.Journal(r.Context(), userID)
		if err != nil {
			respondJSON(w, 404, map[string]string{"error": "wallet_not_found"})
			return
		}
		if entries == nil {
			entries = []shared.JournalEntry{}
		}
		respondJSON(w, 200, map[string]any{"entries": entries})
	}
}

func handleWithdraw(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, _ := auth.UserIDFromCtx(r.Context())

		var body struct {
			Amount         shared.Money `json:"amount"`
			IdempotencyKey shared.Key   `json:"idempotency_key"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		entry, err := Withdraw(r.Context(), deps.Wallet, WithdrawArgs{
			Wallet:   callerID,
			Amount:   body.Amount,
			CallerID: callerID,
			Now:      now,
			Key:      body.IdempotencyKey,
		})
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrInsufficientFunds):
				respondJSON(w, 409, map[string]string{"error": "insufficient_funds"})
			case errors.Is(err, shared.ErrInvalidAmount):
				respondJSON(w, 422, map[string]string{"error": "invalid_amount"})
			case errors.Is(err, shared.ErrWalletNotFound):
				respondJSON(w, 404, map[string]string{"error": "wallet_not_found"})
			case errors.Is(err, shared.ErrNotAuthorized):
				respondJSON(w, 403, map[string]string{"error": "not_authorized"})
			default:
				respondJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}

		respondJSON(w, 201, map[string]any{"entry": entry})
	}
}

func handleTransfer(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, _ := auth.UserIDFromCtx(r.Context())

		var body struct {
			From           *shared.Id   `json:"from"` // optional; if provided and != callerID, triggers WalletOwner rejection
			To             shared.Id    `json:"to"`
			Amount         shared.Money `json:"amount"`
			IdempotencyKey shared.Key   `json:"idempotency_key"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		// Spec maps from=self. If body.from is present and differs from callerID,
		// the Transfer flow's WalletOwner check will reject with NotAuthorized.
		fromID := callerID
		if body.From != nil {
			fromID = *body.From
		}

		result, err := Transfer(r.Context(), deps.Wallet, TransferArgs{
			From:     fromID,
			To:       body.To,
			Amount:   body.Amount,
			CallerID: callerID,
			Now:      now,
			Key:      body.IdempotencyKey,
		})
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrInsufficientFunds):
				respondJSON(w, 409, map[string]string{"error": "insufficient_funds"})
			case errors.Is(err, shared.ErrInvalidAmount):
				respondJSON(w, 422, map[string]string{"error": "invalid_amount"})
			case errors.Is(err, shared.ErrSelfTransfer):
				respondJSON(w, 422, map[string]string{"error": "self_transfer"})
			case errors.Is(err, shared.ErrWalletNotFound):
				respondJSON(w, 404, map[string]string{"error": "wallet_not_found"})
			case errors.Is(err, shared.ErrNotAuthorized):
				respondJSON(w, 403, map[string]string{"error": "not_authorized"})
			default:
				respondJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}

		respondJSON(w, 201, map[string]any{
			"out": result.Out,
			"in":  result.In,
		})
	}
}

func handleScheduleTransfer(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, _ := auth.UserIDFromCtx(r.Context())

		var body struct {
			To             shared.Id    `json:"to"`
			Amount         shared.Money `json:"amount"`
			FireAt         time.Time    `json:"fire_at"`
			IdempotencyKey shared.Key   `json:"idempotency_key"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		result, err := ScheduleTransfer(r.Context(), deps.Wallet, ScheduleTransferArgs{
			From:     callerID,
			To:       body.To,
			Amount:   body.Amount,
			FireAt:   body.FireAt,
			CallerID: callerID,
			Now:      now,
			Key:      body.IdempotencyKey,
		})
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrInvalidAmount):
				respondJSON(w, 422, map[string]string{"error": "invalid_amount"})
			case errors.Is(err, shared.ErrWalletNotFound):
				respondJSON(w, 404, map[string]string{"error": "wallet_not_found"})
			case errors.Is(err, shared.ErrNotAuthorized):
				respondJSON(w, 403, map[string]string{"error": "not_authorized"})
			default:
				respondJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}

		respondJSON(w, 201, map[string]any{"schedule_id": result.ScheduleID})
	}
}

func handleCancelSchedule(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, _ := auth.UserIDFromCtx(r.Context())
		schedID := shared.Id(chi.URLParam(r, "id"))

		var body struct {
			IdempotencyKey shared.Key `json:"idempotency_key"`
		}
		if err := decodeJSON(r, &body); err != nil {
			respondJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}

		err := CancelScheduledTransfer(r.Context(), deps.Wallet, CancelScheduledTransferArgs{
			ScheduleID: schedID,
			CallerID:   callerID,
			Now:        now,
			Key:        body.IdempotencyKey,
		})
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrScheduleNotFound):
				respondJSON(w, 404, map[string]string{"error": "schedule_not_found"})
			case errors.Is(err, shared.ErrAlreadyExecuted):
				respondJSON(w, 409, map[string]string{"error": "already_executed"})
			case errors.Is(err, shared.ErrAlreadyCancelled):
				respondJSON(w, 409, map[string]string{"error": "already_cancelled"})
			case errors.Is(err, shared.ErrNotAuthorized):
				respondJSON(w, 403, map[string]string{"error": "not_authorized"})
			default:
				respondJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}

		w.WriteHeader(204)
	}
}

func handleListSchedules(deps FullDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID, _ := auth.UserIDFromCtx(r.Context())

		schedules, err := deps.Wallet.Schedules.FindPendingBySource(r.Context(), callerID)
		if err != nil {
			respondJSON(w, 500, map[string]string{"error": "internal"})
			return
		}

		type schedView struct {
			ID     shared.Id             `json:"id"`
			Source shared.Id             `json:"source"`
			Dest   shared.Id             `json:"dest"`
			Amount shared.Money          `json:"amount"`
			FireAt time.Time             `json:"fire_at"`
			Status shared.ScheduleStatus `json:"status"`
		}

		views := make([]schedView, 0, len(schedules))
		for _, s := range schedules {
			views = append(views, schedView{
				ID:     s.ID,
				Source: s.Source,
				Dest:   s.Dest,
				Amount: s.Amount,
				FireAt: s.FireAt,
				Status: s.Status,
			})
		}

		respondJSON(w, 200, map[string]any{"schedules": views})
	}
}

// --- Helpers ------------------------------------------------------------------

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}
