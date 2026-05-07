// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package todo

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/auth"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/shared"
)

// MountTodo registers all todo routes on r.
// BearerAuth is applied at feature scope (prose: policies: [BearerAuth]).
// Uses BearerAuthWithUsers so that role promotions take effect immediately
// without requiring a re-login — the role is read from DB on each request.
func MountTodo(r chi.Router, authDeps auth.Deps, deps Deps) {
	r.Group(func(r chi.Router) {
		r.Use(auth.BearerAuthWithUsers(authDeps.JWTSecret, authDeps.Sessions, authDeps.Users))

		r.Post("/todos", handleCreateTodo(deps))
		r.Patch("/todos/{id}", handleUpdateTodo(deps))
		r.Post("/todos/{id}/toggle", handleToggleTodo(deps))
		r.Delete("/todos/{id}", handleDeleteTodo(deps))
		r.Get("/todos", handleListTodos(deps))

		// Admin-only — RoleGated(Admin) stacked on top of BearerAuth.
		r.Group(func(r chi.Router) {
			r.Use(auth.RoleGated(shared.RoleAdmin))
			r.Post("/admin/todos/{id}/assign", handleAssignTodo(deps))
		})
	})
}

func handleCreateTodo(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, callerRole := auth.GetSession(r.Context())
		var body struct {
			Text           string `json:"text"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		result, err := CreateTodo(r.Context(), deps, callerID, callerRole, body.Text, now, shared.Key(body.IdempotencyKey))
		if err != nil {
			if errors.Is(err, shared.ErrEmptyText) {
				writeJSON(w, 422, map[string]string{"error": "empty_text"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": "internal"})
			return
		}
		writeJSON(w, 201, map[string]string{"todo_id": string(result.Todo)})
	}
}

func handleUpdateTodo(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, callerRole := auth.GetSession(r.Context())
		todoID := shared.Id(chi.URLParam(r, "id"))
		var body struct {
			Text           string `json:"text"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		err := UpdateTodo(r.Context(), deps, callerID, callerRole, todoID, body.Text, now, shared.Key(body.IdempotencyKey))
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrNotAuthorized):
				writeJSON(w, 403, map[string]string{"error": "not_authorized"})
			case errors.Is(err, shared.ErrTodoNotFound):
				writeJSON(w, 404, map[string]string{"error": "not_found"})
			case errors.Is(err, shared.ErrEmptyText):
				writeJSON(w, 422, map[string]string{"error": "empty_text"})
			default:
				writeJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}
		writeJSON(w, 200, map[string]string{})
	}
}

func handleToggleTodo(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, callerRole := auth.GetSession(r.Context())
		todoID := shared.Id(chi.URLParam(r, "id"))
		var body struct {
			IdempotencyKey string `json:"idempotency_key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		result, err := ToggleTodo(r.Context(), deps, callerID, callerRole, todoID, now, shared.Key(body.IdempotencyKey))
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrNotAuthorized):
				writeJSON(w, 403, map[string]string{"error": "not_authorized"})
			case errors.Is(err, shared.ErrTodoNotFound):
				writeJSON(w, 404, map[string]string{"error": "not_found"})
			default:
				writeJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}
		writeJSON(w, 200, map[string]bool{"done": result.Done})
	}
}

func handleDeleteTodo(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, callerRole := auth.GetSession(r.Context())
		todoID := shared.Id(chi.URLParam(r, "id"))
		var body struct {
			IdempotencyKey string `json:"idempotency_key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		err := DeleteTodo(r.Context(), deps, callerID, callerRole, todoID, now, shared.Key(body.IdempotencyKey))
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrNotAuthorized):
				writeJSON(w, 403, map[string]string{"error": "not_authorized"})
			case errors.Is(err, shared.ErrTodoNotFound):
				writeJSON(w, 404, map[string]string{"error": "not_found"})
			default:
				writeJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}
		w.WriteHeader(204)
	}
}

func handleAssignTodo(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, callerRole := auth.GetSession(r.Context())
		todoID := shared.Id(chi.URLParam(r, "id"))
		var body struct {
			Manager        string `json:"manager"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request"})
			return
		}
		err := AssignTodo(r.Context(), deps, callerID, callerRole, todoID, shared.Id(body.Manager), now, shared.Key(body.IdempotencyKey))
		if err != nil {
			switch {
			case errors.Is(err, shared.ErrNotAuthorized):
				writeJSON(w, 403, map[string]string{"error": "not_authorized"})
			case errors.Is(err, shared.ErrTodoNotFound):
				writeJSON(w, 404, map[string]string{"error": "todo_not_found"})
			case errors.Is(err, shared.ErrUserNotFound):
				writeJSON(w, 404, map[string]string{"error": "user_not_found"})
			case errors.Is(err, shared.ErrNotManager):
				writeJSON(w, 422, map[string]string{"error": "not_a_manager"})
			default:
				writeJSON(w, 500, map[string]string{"error": "internal"})
			}
			return
		}
		writeJSON(w, 200, map[string]bool{"assigned": true})
	}
}

func handleListTodos(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UTC()
		callerID, callerRole := auth.GetSession(r.Context())
		filterStr := r.URL.Query().Get("filter")
		var filter shared.ListFilter
		switch filterStr {
		case "All":
			filter = shared.ListFilterAll
		case "MineOnly":
			filter = shared.ListFilterMineOnly
		case "AssignedToMe":
			filter = shared.ListFilterAssignedToMe
		default:
			filter = shared.ListFilterMineOnly
		}
		result, err := ListTodos(r.Context(), deps, callerID, callerRole, filter, now)
		if err != nil {
			if errors.Is(err, shared.ErrNotAuthorized) {
				writeJSON(w, 403, map[string]string{"error": "not_authorized"})
				return
			}
			writeJSON(w, 500, map[string]string{"error": "internal"})
			return
		}
		writeJSON(w, 200, map[string]any{"todos": result.Todos})
	}
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
