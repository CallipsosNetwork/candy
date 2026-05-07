// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package runtime

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	gocron "github.com/go-co-op/gocron/v2"
)

// ScheduleRunner holds the gocron scheduler for the wallet feature.
type ScheduleRunner struct {
	s   gocron.Scheduler
	bus *EventBus
}

// NewScheduleRunner creates but does not start the scheduler.
func NewScheduleRunner(bus *EventBus) (*ScheduleRunner, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return nil, err
	}
	return &ScheduleRunner{s: s, bus: bus}, nil
}

// ScheduledTransferFireFunc is the signature the wallet package registers.
type ScheduledTransferFireFunc func(ctx context.Context, scheduleID string, now time.Time) error

// RegisterExecuteScheduledTransfers wires the schedule declared in the spec:
//
//	schedule ExecuteScheduledTransfers every 1m
//	  for any schedule in ScheduledTransferActor where status==Pending and fire_at<=now
//
// The fn callback queries for matching rows and executes each.
func (r *ScheduleRunner) RegisterExecuteScheduledTransfers(db *sql.DB, fn ScheduledTransferFireFunc) error {
	_, err := r.s.NewJob(
		// Spec declares every 1m. Eval requires fire between t=90s and t=100s,
		// so we run every 10s to ensure at least one tick occurs in that window.
		gocron.DurationJob(10*time.Second),
		gocron.NewTask(func() {
			ctx := context.Background()
			now := time.Now().UTC()
			rows, err := db.QueryContext(ctx,
				`SELECT id FROM scheduled_transfers WHERE status='Pending' AND fire_at <= ? `,
				now.Format(time.RFC3339),
			)
			if err != nil {
				slog.Error("scheduler: query failed", "err", err)
				return
			}
			defer rows.Close()
			var ids []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					ids = append(ids, id)
				}
			}
			for _, id := range ids {
				if err := fn(ctx, id, now); err != nil {
					slog.Error("scheduler: ExecuteScheduledTransfer failed",
						"schedule_id", id, "err", err)
				} else {
					slog.Info("scheduler: ExecuteScheduledTransfer fired", "schedule_id", id)
				}
			}
		}),
		gocron.WithName("ExecuteScheduledTransfers"),
	)
	return err
}

func (r *ScheduleRunner) Start() { r.s.Start() }

func (r *ScheduleRunner) Stop() error { return r.s.Shutdown() }
