// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use serde::{Deserialize, Serialize};
use std::fmt;

/// Opaque id, max 64 chars.
#[derive(Clone, Debug, Eq, PartialEq, Hash, Serialize, Deserialize)]
pub struct Id(pub String);

impl Id {
    pub fn new(s: impl Into<String>) -> Self {
        Id(s.into())
    }
}

impl fmt::Display for Id {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

/// UTC timestamp.
pub type Timestamp = chrono::DateTime<chrono::Utc>;

/// Email address, max 320 chars, rfc5322 format.
#[derive(Clone, Debug, Eq, PartialEq, Hash, Serialize, Deserialize)]
pub struct Email(pub String);

impl Email {
    pub fn parse(s: impl Into<String>) -> Result<Self, String> {
        let s = s.into();
        if s.len() > 320 {
            return Err("email too long".into());
        }
        // basic rfc5322 check: must contain exactly one @
        let at_count = s.chars().filter(|&c| c == '@').count();
        if at_count != 1 {
            return Err("invalid email format".into());
        }
        let parts: Vec<&str> = s.splitn(2, '@').collect();
        if parts[0].is_empty() || parts[1].is_empty() {
            return Err("invalid email format".into());
        }
        Ok(Email(s))
    }
}

impl fmt::Display for Email {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

/// Plaintext password — never persisted, only used transiently.
#[derive(Clone, Debug, Deserialize)]
pub struct Password(pub String);

/// Hashed password — the only form persisted.
#[derive(Clone, Debug)]
pub struct PasswordHash(pub String);

/// Opaque bearer token (JWT), max 256 chars.
#[derive(Clone, Debug, Eq, PartialEq, Hash, Serialize, Deserialize)]
pub struct Token(pub String);

impl fmt::Display for Token {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

/// Idempotency key, max 128 chars.
#[derive(Clone, Debug, Eq, PartialEq, Hash, Serialize, Deserialize)]
pub struct Key(pub String);
