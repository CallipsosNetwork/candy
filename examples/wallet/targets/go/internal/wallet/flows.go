// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package wallet

import (
	"context"
	"fmt"
	"time"

	"github.com/CallipsosNetwork/candy/examples/wallet/targets/go/internal/shared"
	"github.com/segmentio/ksuid"
)

// Deps bundles all wallet-package repositories needed by flows.
type Deps struct {
	Wallets    *WalletRepo
	Schedules  *ScheduledTransferRepo
}

func generateID() shared.Id {
	return shared.Id(ksuid.New().String())
}

// FundWalletArgs matches the spec FundWallet flow.
type FundWalletArgs struct {
	Wallet shared.Id  // the owner id
	Amount shared.Money
	By     shared.Id  // admin user id (self)
	Now    time.Time
	Key    shared.Key
}

// FundWallet is the admin-only credit flow. Validates amount > 0, then calls AdminFund.
func FundWallet(ctx context.Context, deps Deps, a FundWalletArgs) (*shared.JournalEntry, error) {
	// step _ = if amount <= 0 then reject InvalidAmount
	if a.Amount <= 0 {
		return nil, shared.ErrInvalidAmount
	}

	// step w = ask Wallet.findBy(owner: wallet) rescue reject WalletNotFound
	exists, err := deps.Wallets.Exists(ctx, a.Wallet)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, shared.ErrWalletNotFound
	}

	// step entry = ask Wallet(wallet).AdminFund(...)
	entry, err := deps.Wallets.AdminFund(ctx, a.Wallet, a.Amount, a.By, a.Key, a.Now)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// WithdrawArgs matches the spec Withdraw flow.
type WithdrawArgs struct {
	Wallet    shared.Id
	Amount    shared.Money
	CallerID  shared.Id // session.user — WalletOwner enforced here
	Now       time.Time
	Key       shared.Key
}

// Withdraw debits the caller's own wallet.
func Withdraw(ctx context.Context, deps Deps, a WithdrawArgs) (*shared.JournalEntry, error) {
	// WalletOwner policy: caller must own the wallet.
	if a.CallerID != a.Wallet {
		return nil, shared.ErrNotAuthorized
	}

	// step _ = if amount <= 0 then reject InvalidAmount
	if a.Amount <= 0 {
		return nil, shared.ErrInvalidAmount
	}

	// step w = ask Wallet.findBy(owner: wallet) rescue reject WalletNotFound
	exists, err := deps.Wallets.Exists(ctx, a.Wallet)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, shared.ErrWalletNotFound
	}

	// step entry = ask Wallet(wallet).Debit(amount, Withdrawal, none, key, now)
	entry, err := deps.Wallets.Debit(ctx, a.Wallet, a.Amount, shared.EntryKindWithdrawal, nil, a.Key, a.Now)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// TransferArgs matches the spec Transfer flow.
type TransferArgs struct {
	From     shared.Id
	To       shared.Id
	Amount   shared.Money
	CallerID shared.Id // session.user — WalletOwner on source
	Now      time.Time
	Key      shared.Key
}

// TransferResult holds the two journal entries from a successful transfer.
type TransferResult struct {
	Out shared.JournalEntry
	In  shared.JournalEntry
}

// Transfer moves amount from source to destination atomically.
// Policy: WalletOwner (source), TransferAtomicity (debit+credit saga).
func Transfer(ctx context.Context, deps Deps, a TransferArgs) (TransferResult, error) {
	// WalletOwner policy: caller must own the source wallet.
	if a.CallerID != a.From {
		return TransferResult{}, shared.ErrNotAuthorized
	}

	// step _ = if amount <= 0 then reject InvalidAmount
	if a.Amount <= 0 {
		return TransferResult{}, shared.ErrInvalidAmount
	}

	// step _ = if from == to then reject SelfTransfer
	if a.From == a.To {
		return TransferResult{}, shared.ErrSelfTransfer
	}

	// step src = ask Wallet.findBy(owner: from) rescue reject WalletNotFound
	srcExists, err := deps.Wallets.Exists(ctx, a.From)
	if err != nil {
		return TransferResult{}, err
	}
	if !srcExists {
		return TransferResult{}, shared.ErrWalletNotFound
	}

	// step dst = ask Wallet.findBy(owner: to) rescue reject WalletNotFound
	dstExists, err := deps.Wallets.Exists(ctx, a.To)
	if err != nil {
		return TransferResult{}, err
	}
	if !dstExists {
		return TransferResult{}, shared.ErrWalletNotFound
	}

	// Check idempotency: if both out and in entries already exist, return them.
	priorOut, err := deps.Wallets.FindEntry(ctx, a.From, a.Key, shared.EntryKindTransferOut)
	if err != nil {
		return TransferResult{}, err
	}
	priorIn, err := deps.Wallets.FindEntry(ctx, a.To, a.Key, shared.EntryKindTransferIn)
	if err != nil {
		return TransferResult{}, err
	}
	if priorOut != nil && priorIn != nil {
		return TransferResult{Out: *priorOut, In: *priorIn}, nil
	}

	// step out = ask Wallet(from).Debit(amount, TransferOut, to, key, now)
	toID := a.To
	out, err := deps.Wallets.Debit(ctx, a.From, a.Amount, shared.EntryKindTransferOut, &toID, a.Key, a.Now)
	if err != nil {
		return TransferResult{}, err
	}

	// step in = ask Wallet(to).Credit(amount, TransferIn, from, key, now)
	// rescue ask Wallet(from).Credit(amount, Compensation, to, key+"#compensate", now); reject err
	fromID := a.From
	in, err := deps.Wallets.Credit(ctx, a.To, a.Amount, shared.EntryKindTransferIn, &fromID, a.Key, a.Now)
	if err != nil {
		// Compensation: credit back to source with key+"#compensate" and kind Compensation.
		compKey := shared.Key(fmt.Sprintf("%s#compensate", string(a.Key)))
		_, _ = deps.Wallets.Credit(ctx, a.From, a.Amount, shared.EntryKindCompensation, &toID, compKey, a.Now)
		return TransferResult{}, err
	}

	return TransferResult{Out: *out, In: *in}, nil
}

// ScheduleTransferArgs matches the spec ScheduleTransfer flow.
type ScheduleTransferArgs struct {
	From     shared.Id
	To       shared.Id
	Amount   shared.Money
	FireAt   time.Time
	CallerID shared.Id
	Now      time.Time
	Key      shared.Key
}

// ScheduleTransferResult holds the created schedule id.
type ScheduleTransferResult struct {
	ScheduleID shared.Id
}

// ScheduleTransfer queues a future peer-to-peer transfer.
func ScheduleTransfer(ctx context.Context, deps Deps, a ScheduleTransferArgs) (ScheduleTransferResult, error) {
	// WalletOwner policy: caller must own source wallet.
	if a.CallerID != a.From {
		return ScheduleTransferResult{}, shared.ErrNotAuthorized
	}

	// step _ = if amount <= 0 then reject InvalidAmount
	if a.Amount <= 0 {
		return ScheduleTransferResult{}, shared.ErrInvalidAmount
	}

	// step _ = if fire_at <= now then reject InvalidAmount
	if !a.FireAt.After(a.Now) {
		return ScheduleTransferResult{}, shared.ErrInvalidAmount
	}

	// step src = ask Wallet.findBy(owner: from) rescue reject WalletNotFound
	srcExists, err := deps.Wallets.Exists(ctx, a.From)
	if err != nil {
		return ScheduleTransferResult{}, err
	}
	if !srcExists {
		return ScheduleTransferResult{}, shared.ErrWalletNotFound
	}

	// step dst = ask Wallet.findBy(owner: to) rescue reject WalletNotFound
	dstExists, err := deps.Wallets.Exists(ctx, a.To)
	if err != nil {
		return ScheduleTransferResult{}, err
	}
	if !dstExists {
		return ScheduleTransferResult{}, shared.ErrWalletNotFound
	}

	// step sched = ask ScheduledTransferActor.create(...)
	id := generateID()
	sched, err := deps.Schedules.Create(ctx, CreateScheduledTransferArgs{
		ID:      id,
		Source:  a.From,
		Dest:    a.To,
		Amount:  a.Amount,
		FireAt:  a.FireAt,
		Key:     a.Key,
		Status:  shared.ScheduleStatusPending,
		Created: a.Now,
	})
	if err != nil {
		return ScheduleTransferResult{}, err
	}

	return ScheduleTransferResult{ScheduleID: sched.ID}, nil
}

// CancelScheduledTransferArgs matches the spec CancelScheduledTransfer flow.
type CancelScheduledTransferArgs struct {
	ScheduleID shared.Id
	CallerID   shared.Id
	Now        time.Time
	Key        shared.Key
}

// CancelScheduledTransfer cancels a Pending scheduled transfer.
// Authorization is checked before state — NotAuthorized fires even on non-Pending schedules.
func CancelScheduledTransfer(ctx context.Context, deps Deps, a CancelScheduledTransferArgs) error {
	// step sched = ask ScheduledTransferActor.findBy(id: schedule) rescue reject ScheduleNotFound
	sched, err := deps.Schedules.FindByID(ctx, a.ScheduleID)
	if err != nil {
		return err // ErrScheduleNotFound
	}

	// step _ = if sched.source != self then reject NotAuthorized
	// Authorization is checked before state.
	if sched.Source != a.CallerID {
		return shared.ErrNotAuthorized
	}

	// ask ScheduledTransferActor(schedule).MarkCancelled()
	return deps.Schedules.MarkCancelled(ctx, a.ScheduleID)
}

// ExecuteScheduledTransferArgs matches the spec ExecuteScheduledTransfer flow.
type ExecuteScheduledTransferArgs struct {
	ScheduleID shared.Id
	Now        time.Time
	Key        shared.Key // outer idempotency key (generate() from scheduler)
}

// ExecuteScheduledTransfer executes a Pending scheduled transfer.
// Invoked by the scheduler. Delegates to Transfer using the schedule's captured key.
func ExecuteScheduledTransfer(ctx context.Context, deps Deps, a ExecuteScheduledTransferArgs) (TransferResult, error) {
	// step sched = ask ScheduledTransferActor.findBy(id: schedule) rescue reject ScheduleNotFound
	sched, err := deps.Schedules.FindByID(ctx, a.ScheduleID)
	if err != nil {
		return TransferResult{}, err
	}

	// step _ = if sched.status != Pending then reject AlreadyExecuted
	if sched.Status != shared.ScheduleStatusPending {
		return TransferResult{}, shared.ErrAlreadyExecuted
	}

	// step result = Transfer(sched.source, sched.dest, sched.amount, now, sched.key)
	result, err := Transfer(ctx, deps, TransferArgs{
		From:     sched.Source,
		To:       sched.Dest,
		Amount:   sched.Amount,
		CallerID: sched.Source, // scheduled transfer bypasses WalletOwner — source is authoritative
		Now:      a.Now,
		Key:      sched.Key,
	})
	if err != nil {
		return TransferResult{}, err
	}

	// ask ScheduledTransferActor(schedule).MarkExecuted()
	if err := deps.Schedules.MarkExecuted(ctx, a.ScheduleID); err != nil {
		// If already executed, that's fine — it's idempotent at the Transfer level.
		if err != shared.ErrAlreadyExecuted {
			return TransferResult{}, err
		}
	}

	return result, nil
}
