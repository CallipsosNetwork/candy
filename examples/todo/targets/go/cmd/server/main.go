// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/auth"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/runtime"
	"github.com/CallipsosNetwork/candy/examples/todo/targets/go/internal/todo"
)

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "/tmp/todo.db")
	jwtSecret := []byte(envOr("JWT_SECRET", "dev-secret-change-me"))

	db, err := runtime.Open(dbPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	bus := runtime.NewEventBus()

	authDeps := buildAuthDeps(db, bus, jwtSecret)
	todoDeps := buildTodoDeps(db, bus, authDeps)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	auth.MountAuth(r, authDeps)
	todo.MountTodo(r, authDeps, todoDeps)

	addr := fmt.Sprintf(":%s", port)
	slog.Info("todo server starting", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func buildAuthDeps(db *sql.DB, bus *runtime.EventBus, secret []byte) auth.Deps {
	return auth.Deps{
		DB:        db,
		Users:     &auth.UserRepo{DB: db},
		Sessions:  &auth.SessionRepo{DB: db},
		Bus:       bus,
		JWTSecret: secret,
	}
}

func buildTodoDeps(db *sql.DB, bus *runtime.EventBus, authDeps auth.Deps) todo.Deps {
	return todo.Deps{
		DB:    db,
		Todos: &todo.TodoRepo{DB: db},
		Users: authDeps.Users,
		Bus:   bus,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
