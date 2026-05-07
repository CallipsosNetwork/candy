// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	authpkg "github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/auth"
	"github.com/CallipsosNetwork/candy/examples/auth/targets/go/internal/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Configuration from environment.
	dbPath := envOr("DB_PATH", "/tmp/auth-dev.db")
	port := envOr("PORT", "8080")
	jwtSecret := []byte(envOr("JWT_SECRET", "dev-secret-change-in-production"))

	// Open SQLite database and run schema.
	db, err := runtime.OpenDB(ctx, dbPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Wire dependencies.
	bus := runtime.NewEventBus()
	deps := authpkg.Deps{
		Users:       authpkg.NewUserRepo(db),
		JWT:         authpkg.NewJWTService(jwtSecret, "candy-auth", authpkg.SessionTTL),
		Revoked:     authpkg.NewRevokedRepo(db),
		Idempotency: authpkg.NewIdempotencyRepo(db),
		EventBus:    bus,
	}

	// Build router.
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	authpkg.MountAuth(r, deps)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", port, "db", dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
