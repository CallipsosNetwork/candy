// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package shared

import "errors"

// Branded scalar types — opaque wrappers prevent accidental mixing.
type Id string
type Token string
type Key string
type Email string
type Password string
type PasswordHash string

// Role enum.
type Role string

const (
	RoleAdmin   Role = "Admin"
	RoleManager Role = "Manager"
	RoleUser    Role = "User"
)

// RoleLevel returns a numeric level for hierarchy comparison.
// User < Manager < Admin.
func RoleLevel(r Role) int {
	switch r {
	case RoleAdmin:
		return 2
	case RoleManager:
		return 1
	default:
		return 0
	}
}

// TodoStatus enum.
type TodoStatus string

const (
	TodoStatusOpen TodoStatus = "Open"
	TodoStatusDone TodoStatus = "Done"
)

// ListFilter enum.
type ListFilter string

const (
	ListFilterAll          ListFilter = "All"
	ListFilterMineOnly     ListFilter = "MineOnly"
	ListFilterAssignedToMe ListFilter = "AssignedToMe"
)

// Declared error variants — one sentinel per spec variant.
var (
	ErrWeakPassword      = errors.New("weak_password")
	ErrMissingDigit      = errors.New("missing_digit")
	ErrTooShort          = errors.New("too_short")
	ErrInBlocklist       = errors.New("in_blocklist")
	ErrEmailTaken        = errors.New("email_taken")
	ErrInvalidCredentials = errors.New("invalid_credentials")
	ErrSessionInvalid    = errors.New("session_invalid")
	ErrUnauthenticated   = errors.New("unauthenticated")
	ErrAlreadyVerified   = errors.New("already_verified")
	ErrInvalidPromotion  = errors.New("invalid_promotion")
	ErrNotAuthorized     = errors.New("not_authorized")
	ErrTodoNotFound      = errors.New("todo_not_found")
	ErrUserNotFound      = errors.New("user_not_found")
	ErrNotManager        = errors.New("not_a_manager")
	ErrEmptyText         = errors.New("empty_text")
)

// WeakPasswordErr carries the sub-reason for PasswordStrength failures.
type WeakPasswordErr struct {
	Reason error
}

func (e *WeakPasswordErr) Error() string { return "weak_password: " + e.Reason.Error() }
func (e *WeakPasswordErr) Unwrap() error { return ErrWeakPassword }
