// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use anyhow::Result;
use rusqlite::Connection;
use std::sync::{Arc, Mutex};

pub type DbPool = Arc<Mutex<Connection>>;

pub fn open(path: &str) -> Result<DbPool> {
    let conn = Connection::open(path)?;
    conn.execute_batch("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;")?;
    migrate(&conn)?;
    Ok(Arc::new(Mutex::new(conn)))
}

fn migrate(conn: &Connection) -> Result<()> {
    conn.execute_batch(
        "
        CREATE TABLE IF NOT EXISTS users (
            id         TEXT PRIMARY KEY,
            email      TEXT NOT NULL UNIQUE,
            hash       TEXT NOT NULL,
            created    TEXT NOT NULL,
            verified   INTEGER NOT NULL DEFAULT 0
        );

        CREATE TABLE IF NOT EXISTS revoked_jtis (
            jti        TEXT PRIMARY KEY,
            user_id    TEXT NOT NULL,
            revoked_at TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS signup_idempotency (
            idem_key   TEXT PRIMARY KEY,
            user_id    TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS audit_user_verified (
            id         INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id    TEXT NOT NULL,
            at         TEXT NOT NULL
        );
        ",
    )?;
    Ok(())
}
