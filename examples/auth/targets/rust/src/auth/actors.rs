// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use crate::runtime::db::DbPool;
use crate::shared::{
    errors::AuthError,
    types::{Email, Id, Key, Password, PasswordHash, Timestamp, Token},
};
use argon2::{
    password_hash::{PasswordHash as Argon2Hash, PasswordHasher, PasswordVerifier, SaltString},
    Argon2,
};
use chrono::Utc;
use jsonwebtoken::{decode, encode, Algorithm, DecodingKey, EncodingKey, Header, Validation};
use rand_core::OsRng;
use rusqlite::params;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

// ── JWT claims ────────────────────────────────────────────────────────────────

#[derive(Debug, Serialize, Deserialize)]
pub struct JwtClaims {
    /// Subject: user id
    pub sub: String,
    /// JWT id: fresh uuid for revocation
    pub jti: String,
    /// Issued at (unix seconds)
    pub iat: i64,
    /// Expires at (unix seconds)
    pub exp: i64,
}

pub struct JwtConfig {
    pub secret: String,
}

impl JwtConfig {
    pub fn issue(&self, user_id: &Id, now: Timestamp) -> Result<Token, AuthError> {
        let jti = Uuid::now_v7().to_string();
        let iat = now.timestamp();
        let exp = iat + 7 * 24 * 3600; // 7d TTL
        let claims = JwtClaims {
            sub: user_id.0.clone(),
            jti,
            iat,
            exp,
        };
        let token = encode(
            &Header::new(Algorithm::HS256),
            &claims,
            &EncodingKey::from_secret(self.secret.as_bytes()),
        )
        .map_err(|e| AuthError::Internal(e.to_string()))?;
        Ok(Token(token))
    }

    /// Decode and verify signature + expiry. Does NOT check revocation.
    pub fn decode(&self, token: &Token) -> Result<JwtClaims, AuthError> {
        let mut validation = Validation::new(Algorithm::HS256);
        validation.validate_exp = true;
        decode::<JwtClaims>(
            &token.0,
            &DecodingKey::from_secret(self.secret.as_bytes()),
            &validation,
        )
        .map(|d| d.claims)
        .map_err(|_| AuthError::SessionInvalid)
    }
}

// ── Password hashing ──────────────────────────────────────────────────────────

/// hash(password) — argon2id
pub fn hash_password(password: &Password) -> Result<PasswordHash, AuthError> {
    let salt = SaltString::generate(&mut OsRng);
    let argon2 = Argon2::default();
    let hash = argon2
        .hash_password(password.0.as_bytes(), &salt)
        .map_err(|e| AuthError::Internal(e.to_string()))?;
    Ok(PasswordHash(hash.to_string()))
}

/// verify(plaintext, hash) — argon2id
pub fn verify_password(password: &Password, stored: &PasswordHash) -> bool {
    let parsed = match Argon2Hash::new(&stored.0) {
        Ok(h) => h,
        Err(_) => return false,
    };
    Argon2::default()
        .verify_password(password.0.as_bytes(), &parsed)
        .is_ok()
}

// ── UserRepo ──────────────────────────────────────────────────────────────────

pub struct User {
    pub id: Id,
    pub email: Email,
    pub hash: PasswordHash,
    pub created: Timestamp,
    pub verified: bool,
}

pub struct UserRepo {
    pub pool: DbPool,
}

impl UserRepo {
    /// User.create({id, email, hash, created})
    pub fn create(
        &self,
        id: Id,
        email: Email,
        hash: PasswordHash,
        created: Timestamp,
    ) -> Result<User, AuthError> {
        let conn = self.pool.lock().unwrap();
        conn.execute(
            "INSERT INTO users (id, email, hash, created, verified) VALUES (?1, ?2, ?3, ?4, 0)",
            params![id.0, email.0, hash.0, created.to_rfc3339()],
        )?;
        Ok(User {
            id,
            email,
            hash,
            created,
            verified: false,
        })
    }

    /// User.findBy(email)
    pub fn find_by_email(&self, email: &Email) -> Result<User, AuthError> {
        let conn = self.pool.lock().unwrap();
        let mut stmt = conn
            .prepare("SELECT id, email, hash, created, verified FROM users WHERE email = ?1")
            .map_err(|e| AuthError::Internal(e.to_string()))?;
        let user = stmt
            .query_row(params![email.0], |row| {
                let id: String = row.get(0)?;
                let em: String = row.get(1)?;
                let hash: String = row.get(2)?;
                let created_str: String = row.get(3)?;
                let verified: bool = row.get(4)?;
                Ok((id, em, hash, created_str, verified))
            })
            .map_err(|_| AuthError::InvalidCredentials)?;
        let created = user.3.parse::<Timestamp>().unwrap_or_else(|_| Utc::now());
        Ok(User {
            id: Id(user.0),
            email: Email(user.1),
            hash: PasswordHash(user.2),
            created,
            verified: user.4,
        })
    }

    /// Check if any user has this email (for the EmailTaken check).
    pub fn email_exists(&self, email: &Email) -> Result<bool, AuthError> {
        let conn = self.pool.lock().unwrap();
        let count: i64 = conn
            .query_row(
                "SELECT COUNT(*) FROM users WHERE email = ?1",
                params![email.0],
                |row| row.get(0),
            )
            .map_err(|e| AuthError::Internal(e.to_string()))?;
        Ok(count > 0)
    }
}

// ── RevokedJtiRepo ────────────────────────────────────────────────────────────

pub struct RevokedJtiRepo {
    pub pool: DbPool,
}

impl RevokedJtiRepo {
    /// Write revoked jti. INSERT OR IGNORE for idempotency.
    pub fn revoke(&self, jti: &str, user_id: &str, now: Timestamp) -> Result<(), AuthError> {
        let conn = self.pool.lock().unwrap();
        conn.execute(
            "INSERT OR IGNORE INTO revoked_jtis (jti, user_id, revoked_at) VALUES (?1, ?2, ?3)",
            params![jti, user_id, now.to_rfc3339()],
        )?;
        Ok(())
    }

    /// Check if a jti has been revoked.
    pub fn is_revoked(&self, jti: &str) -> Result<bool, AuthError> {
        let conn = self.pool.lock().unwrap();
        let count: i64 = conn
            .query_row(
                "SELECT COUNT(*) FROM revoked_jtis WHERE jti = ?1",
                params![jti],
                |row| row.get(0),
            )
            .map_err(|e| AuthError::Internal(e.to_string()))?;
        Ok(count > 0)
    }
}

// ── SignupIdempotencyRepo ─────────────────────────────────────────────────────

pub struct SignupIdempotencyRepo {
    pub pool: DbPool,
}

impl SignupIdempotencyRepo {
    /// Look up a prior signup by idempotency key.
    pub fn find(&self, key: &Key) -> Result<Option<Id>, AuthError> {
        let conn = self.pool.lock().unwrap();
        let result = conn.query_row(
            "SELECT user_id FROM signup_idempotency WHERE idem_key = ?1",
            params![key.0],
            |row| {
                let id: String = row.get(0)?;
                Ok(id)
            },
        );
        match result {
            Ok(id) => Ok(Some(Id(id))),
            Err(rusqlite::Error::QueryReturnedNoRows) => Ok(None),
            Err(e) => Err(AuthError::Internal(e.to_string())),
        }
    }

    /// Persist (key, user_id). INSERT OR IGNORE for idempotency.
    pub fn insert(&self, key: &Key, user_id: &Id) -> Result<(), AuthError> {
        let conn = self.pool.lock().unwrap();
        conn.execute(
            "INSERT OR IGNORE INTO signup_idempotency (idem_key, user_id) VALUES (?1, ?2)",
            params![key.0, user_id.0],
        )?;
        Ok(())
    }
}

// ── Password helper re-export ─────────────────────────────────────────────────

/// generate() → Id using UUID v7
pub fn generate_id() -> Id {
    Id(Uuid::now_v7().to_string())
}
