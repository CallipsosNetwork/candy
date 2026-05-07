// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package runtime

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// OpenDB opens the SQLite database at path and runs the schema migration.
func OpenDB(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("migrate db: %w", err)
	}
	return db, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id       TEXT PRIMARY KEY,
	email    TEXT NOT NULL UNIQUE,
	hash     TEXT NOT NULL,
	created  TEXT NOT NULL,
	verified INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
	token   TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	issued  TEXT NOT NULL,
	expires TEXT NOT NULL,
	revoked INTEGER NOT NULL DEFAULT 0
);

-- idempotency store for Signup
CREATE TABLE IF NOT EXISTS signup_idempotency (
	key     TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	token   TEXT NOT NULL
);

-- append-only audit for UserVerified
CREATE TABLE IF NOT EXISTS audit_user_verified (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	at      TEXT NOT NULL
);

-- append-only audit for SessionRevoked
CREATE TABLE IF NOT EXISTS audit_session_revoked (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	token   TEXT NOT NULL,
	user_id TEXT NOT NULL,
	at      TEXT NOT NULL
);
`

func migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schema)
	return err
}
