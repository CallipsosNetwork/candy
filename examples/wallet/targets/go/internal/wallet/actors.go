// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package wallet

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
	"github.com/segmentio/ksuid"
)

// WalletRepo manages the wallets and journal tables.
type WalletRepo struct{ db *sql.DB }

func NewWalletRepo(db *sql.DB) *WalletRepo { return &WalletRepo{db: db} }

// Create creates a wallet for an owner. Idempotent (INSERT OR IGNORE).
func (r *WalletRepo) Create(ctx context.Context, ownerID shared.Id, now time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO wallets(owner_id, created) VALUES(?,?)`,
		string(ownerID), now.UTC().Format(time.RFC3339),
	)
	return err
}

// Exists returns true if a wallet for ownerID exists.
func (r *WalletRepo) Exists(ctx context.Context, ownerID shared.Id) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM wallets WHERE owner_id=?`, string(ownerID)).Scan(&n)
	return n > 0, err
}

// Balance returns sum(delta) for the wallet — journal as source of truth.
func (r *WalletRepo) Balance(ctx context.Context, ownerID shared.Id) (shared.Money, error) {
	var sum sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT SUM(delta) FROM journal WHERE owner_id=?`, string(ownerID)).Scan(&sum)
	if err != nil {
		return 0, err
	}
	return shared.Money(sum.Int64), nil
}

// Journal returns all journal entries for the wallet.
func (r *WalletRepo) Journal(ctx context.Context, ownerID shared.Id) ([]shared.JournalEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, kind, delta, counterpart, key, at FROM journal WHERE owner_id=? ORDER BY at ASC`,
		string(ownerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []shared.JournalEntry
	for rows.Next() {
		var e shared.JournalEntry
		var id, kind, key, at string
		var counterpart sql.NullString
		var delta int64
		if err := rows.Scan(&id, &kind, &delta, &counterpart, &key, &at); err != nil {
			return nil, err
		}
		e.ID = shared.Id(id)
		e.Kind = shared.EntryKind(kind)
		e.Delta = shared.Money(delta)
		e.Key = shared.Key(key)
		if counterpart.Valid {
			cp := shared.Id(counterpart.String)
			e.Counterpart = &cp
		}
		t, _ := time.Parse(time.RFC3339, at)
		e.At = t
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// FindEntry looks for a journal entry by owner, key, and kind — for idempotency.
func (r *WalletRepo) FindEntry(ctx context.Context, ownerID shared.Id, key shared.Key, kind shared.EntryKind) (*shared.JournalEntry, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, kind, delta, counterpart, key, at FROM journal WHERE owner_id=? AND key=? AND kind=?`,
		string(ownerID), string(key), string(kind))
	var e shared.JournalEntry
	var id, ekind, ekey, at string
	var counterpart sql.NullString
	var delta int64
	err := row.Scan(&id, &ekind, &delta, &counterpart, &ekey, &at)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find entry: %w", err)
	}
	e.ID = shared.Id(id)
	e.Kind = shared.EntryKind(ekind)
	e.Delta = shared.Money(delta)
	e.Key = shared.Key(ekey)
	if counterpart.Valid {
		cp := shared.Id(counterpart.String)
		e.Counterpart = &cp
	}
	t, _ := time.Parse(time.RFC3339, at)
	e.At = t
	return &e, nil
}

// AppendEntry inserts a new journal entry. Append-only — no UPDATE on journal rows.
func (r *WalletRepo) AppendEntry(ctx context.Context, ownerID shared.Id, e shared.JournalEntry) error {
	var counterpart interface{}
	if e.Counterpart != nil {
		counterpart = string(*e.Counterpart)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO journal(id, owner_id, kind, delta, counterpart, key, at) VALUES(?,?,?,?,?,?,?)`,
		string(e.ID), string(ownerID), string(e.Kind), int64(e.Delta),
		counterpart, string(e.Key), e.At.UTC().Format(time.RFC3339),
	)
	return err
}

// AdminFund appends a Fund journal entry. Idempotent on (key, Fund).
// Invariant: balance >= 0 is preserved because Fund is always positive.
func (r *WalletRepo) AdminFund(ctx context.Context, ownerID shared.Id, amount shared.Money, by shared.Id, key shared.Key, now time.Time) (*shared.JournalEntry, error) {
	// Idempotency check on (key, Fund).
	prior, err := r.FindEntry(ctx, ownerID, key, shared.EntryKindFund)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		if prior.Delta != amount {
			return nil, shared.ErrReplayMismatch
		}
		return prior, nil
	}
	entry := shared.JournalEntry{
		ID:          shared.Id(ksuid.New().String()),
		Kind:        shared.EntryKindFund,
		Delta:       amount,
		Counterpart: &by,
		Key:         key,
		At:          now,
	}
	if err := r.AppendEntry(ctx, ownerID, entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Credit appends a positive journal entry (TransferIn or Compensation). Idempotent on (key, kind).
func (r *WalletRepo) Credit(ctx context.Context, ownerID shared.Id, amount shared.Money, kind shared.EntryKind, counterpart *shared.Id, key shared.Key, now time.Time) (*shared.JournalEntry, error) {
	prior, err := r.FindEntry(ctx, ownerID, key, kind)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		if prior.Delta != amount {
			return nil, shared.ErrReplayMismatch
		}
		if counterpartMismatch(prior.Counterpart, counterpart) {
			return nil, shared.ErrReplayMismatch
		}
		return prior, nil
	}
	entry := shared.JournalEntry{
		ID:          shared.Id(ksuid.New().String()),
		Kind:        kind,
		Delta:       amount,
		Counterpart: counterpart,
		Key:         key,
		At:          now,
	}
	if err := r.AppendEntry(ctx, ownerID, entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Debit appends a negative journal entry. Idempotent on (key, kind). Enforces balance >= amount.
func (r *WalletRepo) Debit(ctx context.Context, ownerID shared.Id, amount shared.Money, kind shared.EntryKind, counterpart *shared.Id, key shared.Key, now time.Time) (*shared.JournalEntry, error) {
	prior, err := r.FindEntry(ctx, ownerID, key, kind)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		if prior.Delta != -amount {
			return nil, shared.ErrReplayMismatch
		}
		if counterpartMismatch(prior.Counterpart, counterpart) {
			return nil, shared.ErrReplayMismatch
		}
		return prior, nil
	}

	// invariant: balance >= amount
	balance, err := r.Balance(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if balance < amount {
		return nil, shared.ErrInsufficientFunds
	}

	entry := shared.JournalEntry{
		ID:          shared.Id(ksuid.New().String()),
		Kind:        kind,
		Delta:       -amount,
		Counterpart: counterpart,
		Key:         key,
		At:          now,
	}
	if err := r.AppendEntry(ctx, ownerID, entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func counterpartMismatch(a, b *shared.Id) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return *a != *b
}

// ScheduledTransferRepo manages the scheduled_transfers table.
type ScheduledTransferRepo struct{ db *sql.DB }

func NewScheduledTransferRepo(db *sql.DB) *ScheduledTransferRepo {
	return &ScheduledTransferRepo{db: db}
}

type ScheduledTransfer struct {
	ID      shared.Id
	Source  shared.Id
	Dest    shared.Id
	Amount  shared.Money
	FireAt  time.Time
	Key     shared.Key
	Status  shared.ScheduleStatus
	Created time.Time
}

type CreateScheduledTransferArgs struct {
	ID      shared.Id
	Source  shared.Id
	Dest    shared.Id
	Amount  shared.Money
	FireAt  time.Time
	Key     shared.Key
	Status  shared.ScheduleStatus
	Created time.Time
}

func (r *ScheduledTransferRepo) Create(ctx context.Context, a CreateScheduledTransferArgs) (*ScheduledTransfer, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO scheduled_transfers(id, source, dest, amount, fire_at, key, status, created) VALUES(?,?,?,?,?,?,?,?)`,
		string(a.ID), string(a.Source), string(a.Dest), int64(a.Amount),
		a.FireAt.UTC().Format(time.RFC3339), string(a.Key), string(a.Status),
		a.Created.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &ScheduledTransfer{
		ID: a.ID, Source: a.Source, Dest: a.Dest, Amount: a.Amount,
		FireAt: a.FireAt, Key: a.Key, Status: a.Status, Created: a.Created,
	}, nil
}

func (r *ScheduledTransferRepo) FindByID(ctx context.Context, id shared.Id) (*ScheduledTransfer, error) {
	return r.scan(r.db.QueryRowContext(ctx,
		`SELECT id, source, dest, amount, fire_at, key, status, created FROM scheduled_transfers WHERE id=?`,
		string(id)))
}

func (r *ScheduledTransferRepo) FindPendingBySource(ctx context.Context, source shared.Id) ([]ScheduledTransfer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, source, dest, amount, fire_at, key, status, created FROM scheduled_transfers WHERE source=? AND status='Pending'`,
		string(source))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScheduledTransfer
	for rows.Next() {
		var s ScheduledTransfer
		if err := scanScheduled(rows, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ScheduledTransferRepo) MarkExecuted(ctx context.Context, id shared.Id) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE scheduled_transfers SET status='Executed' WHERE id=? AND status='Pending'`,
		string(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Was not Pending — check actual status for the right error.
		st, err2 := r.getStatus(ctx, id)
		if err2 != nil {
			return err2
		}
		if st == shared.ScheduleStatusExecuted {
			return shared.ErrAlreadyExecuted
		}
		return shared.ErrAlreadyCancelled
	}
	return nil
}

func (r *ScheduledTransferRepo) MarkCancelled(ctx context.Context, id shared.Id) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE scheduled_transfers SET status='Cancelled' WHERE id=? AND status='Pending'`,
		string(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		st, err2 := r.getStatus(ctx, id)
		if err2 != nil {
			return err2
		}
		if st == shared.ScheduleStatusExecuted {
			return shared.ErrAlreadyExecuted
		}
		return shared.ErrAlreadyCancelled
	}
	return nil
}

func (r *ScheduledTransferRepo) getStatus(ctx context.Context, id shared.Id) (shared.ScheduleStatus, error) {
	var status string
	err := r.db.QueryRowContext(ctx,
		`SELECT status FROM scheduled_transfers WHERE id=?`, string(id)).Scan(&status)
	if err == sql.ErrNoRows {
		return "", shared.ErrScheduleNotFound
	}
	return shared.ScheduleStatus(status), err
}

func (r *ScheduledTransferRepo) scan(row *sql.Row) (*ScheduledTransfer, error) {
	var s ScheduledTransfer
	if err := scanScheduled(row, &s); err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrScheduleNotFound
		}
		return nil, err
	}
	return &s, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanScheduled(row rowScanner, s *ScheduledTransfer) error {
	var id, source, dest, key, status, created, fireAt string
	var amount int64
	if err := row.Scan(&id, &source, &dest, &amount, &fireAt, &key, &status, &created); err != nil {
		return err
	}
	tFireAt, _ := time.Parse(time.RFC3339, fireAt)
	tCreated, _ := time.Parse(time.RFC3339, created)
	s.ID = shared.Id(id)
	s.Source = shared.Id(source)
	s.Dest = shared.Id(dest)
	s.Amount = shared.Money(amount)
	s.FireAt = tFireAt
	s.Key = shared.Key(key)
	s.Status = shared.ScheduleStatus(status)
	s.Created = tCreated
	return nil
}
