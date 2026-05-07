// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

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
	"github.com/segmentio/ksuid"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/auth"
	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/runtime"
	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/wallet"
)

func main() {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "/tmp/wallet.db")
	jwtSecret := []byte(envOr("JWT_SECRET", "dev-secret-change-in-production"))

	db, err := runtime.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}

	// Build dependency graph.
	userRepo := auth.NewUserRepo(db)
	jwtSvc := auth.NewJWTService(jwtSecret, "candy-wallet", auth.SessionTTL)
	revokedRepo := auth.NewRevokedRepo(db)
	authDeps := auth.Deps{Users: userRepo, JWT: jwtSvc, Revoked: revokedRepo}
	walletRepo := wallet.NewWalletRepo(db)
	scheduleRepo := wallet.NewScheduledTransferRepo(db)
	walletDeps := wallet.Deps{Wallets: walletRepo, Schedules: scheduleRepo}
	fullDeps := wallet.FullDeps{Auth: authDeps, Wallet: walletDeps}

	// Eventbus (eager delivery per spec).
	bus := runtime.NewEventBus()
	_ = bus

	// Scheduler — wire ExecuteScheduledTransfers (every 1m per spec).
	sched, err := runtime.NewScheduleRunner(bus)
	if err != nil {
		slog.Error("scheduler init failed", "err", err)
		os.Exit(1)
	}
	if err := sched.RegisterExecuteScheduledTransfers(db, func(ctx context.Context, scheduleID string, now time.Time) error {
		// outer idempotency key generated per spec: generate() in schedule call
		key := shared.Key(ksuid.New().String())
		_, execErr := wallet.ExecuteScheduledTransfer(ctx, walletDeps, wallet.ExecuteScheduledTransferArgs{
			ScheduleID: shared.Id(scheduleID),
			Now:        now,
			Key:        key,
		})
		return execErr
	}); err != nil {
		slog.Error("register schedule failed", "err", err)
		os.Exit(1)
	}
	sched.Start()

	// HTTP router.
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	wallet.Mount(r, fullDeps)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("wallet server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	_ = sched.Stop()
	slog.Info("server stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
