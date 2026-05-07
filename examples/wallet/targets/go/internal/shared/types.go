// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package shared

import (
	"errors"
	"time"
)

// Branded primitive types from the spec.

type Id string
type Email string
type Password string
type PasswordHash string
type Token string
type Key string

// Money is integer minor units (cents, USD). Never float64.
type Money int64

type Timestamp = time.Time

// EntryKind tags each journal entry.
type EntryKind string

const (
	EntryKindFund        EntryKind = "Fund"
	EntryKindWithdrawal  EntryKind = "Withdrawal"
	EntryKindTransferOut EntryKind = "TransferOut"
	EntryKindTransferIn  EntryKind = "TransferIn"
	EntryKindCompensation EntryKind = "Compensation"
)

// ScheduleStatus is the lifecycle state of a ScheduledTransfer.
type ScheduleStatus string

const (
	ScheduleStatusPending   ScheduleStatus = "Pending"
	ScheduleStatusExecuted  ScheduleStatus = "Executed"
	ScheduleStatusCancelled ScheduleStatus = "Cancelled"
)

// Role represents a user's permission level.
type Role string

const (
	RoleAdmin Role = "Admin"
	RoleUser  Role = "User"
)

// JournalEntry is an immutable record of every balance change.
type JournalEntry struct {
	ID          Id        `json:"id"`
	Kind        EntryKind `json:"kind"`
	Delta       Money     `json:"delta"`
	Counterpart *Id       `json:"counterpart,omitempty"`
	Key         Key       `json:"key"`
	At          time.Time `json:"at"`
}

// Sentinel errors — one per declared error variant.
var (
	ErrWeakPassword      = errors.New("weak_password")
	ErrTooShort          = errors.New("too_short")
	ErrMissingDigit      = errors.New("missing_digit")
	ErrInBlocklist       = errors.New("in_blocklist")
	ErrEmailTaken        = errors.New("email_taken")
	ErrInvalidCredentials = errors.New("invalid_credentials")
	ErrSessionInvalid    = errors.New("session_invalid")
	ErrUnauthenticated   = errors.New("unauthenticated")
	ErrNotAuthorized     = errors.New("not_authorized")
	ErrInsufficientFunds = errors.New("insufficient_funds")
	ErrInvalidAmount     = errors.New("invalid_amount")
	ErrWalletNotFound    = errors.New("wallet_not_found")
	ErrSelfTransfer      = errors.New("self_transfer")
	ErrReplayMismatch    = errors.New("replay_mismatch")
	ErrScheduleNotFound  = errors.New("schedule_not_found")
	ErrAlreadyExecuted   = errors.New("already_executed")
	ErrAlreadyCancelled  = errors.New("already_cancelled")
	ErrNotFound          = errors.New("not_found")
)

// WeakPasswordErr carries the reason for password rejection.
type WeakPasswordErr struct {
	Reason string
}

func (e *WeakPasswordErr) Error() string { return "weak_password: " + e.Reason }
func (e *WeakPasswordErr) Is(target error) bool {
	return target == ErrWeakPassword
}
