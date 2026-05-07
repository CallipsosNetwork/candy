// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package runtime

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Open opens (or creates) the SQLite database at path and runs the schema.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := applySchema(db); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

func applySchema(db *sql.DB) error {
	_, err := db.Exec(`
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS users (
	id         TEXT PRIMARY KEY,
	email      TEXT NOT NULL UNIQUE,
	hash       TEXT NOT NULL,
	role       TEXT NOT NULL DEFAULT 'User',
	verified   INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	jti        TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	role       TEXT NOT NULL,
	issued_at  TEXT NOT NULL,
	expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS revoked_jtis (
	jti        TEXT PRIMARY KEY,
	user_id    TEXT NOT NULL,
	revoked_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS todos (
	id           TEXT PRIMARY KEY,
	text         TEXT NOT NULL,
	done         INTEGER NOT NULL DEFAULT 0,
	owner        TEXT NOT NULL,
	creator_role TEXT NOT NULL,
	assigned_to  TEXT,
	created_at   TEXT NOT NULL,
	updated_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
	key_val     TEXT NOT NULL,
	user_id     TEXT NOT NULL,
	flow        TEXT NOT NULL,
	result_json TEXT NOT NULL,
	created_at  TEXT NOT NULL,
	PRIMARY KEY (key_val, flow)
);
`)
	return err
}
