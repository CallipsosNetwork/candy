// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use thiserror::Error;

/// Reason a password failed PasswordStrength validation.
#[derive(Clone, Debug, Error, PartialEq)]
pub enum PasswordWeaknessReason {
    #[error("too_short")]
    TooShort,
    #[error("missing_digit")]
    MissingDigit,
    #[error("in_blocklist")]
    InBlocklist,
}

/// Declared error variants for auth flows.
#[derive(Debug, Error)]
pub enum AuthError {
    #[error("weak_password")]
    WeakPassword(PasswordWeaknessReason),
    #[error("email_taken")]
    EmailTaken,
    #[error("invalid_credentials")]
    InvalidCredentials,
    #[error("already_verified")]
    AlreadyVerified,
    #[error("session_invalid")]
    SessionInvalid,
    #[error("internal: {0}")]
    Internal(String),
}

impl From<rusqlite::Error> for AuthError {
    fn from(e: rusqlite::Error) -> Self {
        AuthError::Internal(e.to_string())
    }
}
