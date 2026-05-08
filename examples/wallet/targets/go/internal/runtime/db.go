// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package runtime

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Open opens (or creates) the SQLite database at path and applies the schema.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer.
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id      TEXT PRIMARY KEY,
    email   TEXT NOT NULL UNIQUE,
    hash    TEXT NOT NULL,
    role    TEXT NOT NULL DEFAULT 'User',
    created TEXT NOT NULL
);

-- JWT revocation list. Membership = revoked. INSERT OR IGNORE makes
-- Logout idempotent.
CREATE TABLE IF NOT EXISTS revoked_jtis (
    jti        TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    revoked_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS wallets (
    owner_id TEXT PRIMARY KEY,
    created  TEXT NOT NULL
);

-- Journal entries are append-only. Balance = sum(delta).
CREATE TABLE IF NOT EXISTS journal (
    id          TEXT PRIMARY KEY,
    owner_id    TEXT NOT NULL,
    kind        TEXT NOT NULL,
    delta       INTEGER NOT NULL,
    counterpart TEXT,
    key         TEXT NOT NULL,
    at          TEXT NOT NULL,
    FOREIGN KEY (owner_id) REFERENCES wallets(owner_id)
);

CREATE INDEX IF NOT EXISTS idx_journal_owner ON journal(owner_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_journal_key_kind ON journal(owner_id, key, kind);

CREATE TABLE IF NOT EXISTS scheduled_transfers (
    id         TEXT PRIMARY KEY,
    source     TEXT NOT NULL,
    dest       TEXT NOT NULL,
    amount     INTEGER NOT NULL,
    fire_at    TEXT NOT NULL,
    key        TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'Pending',
    created    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scheduled_pending ON scheduled_transfers(status, fire_at);
`
