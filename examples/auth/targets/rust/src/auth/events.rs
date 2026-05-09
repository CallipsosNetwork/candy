// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use crate::shared::types::{Email, Id, Timestamp, Token};
use serde::{Deserialize, Serialize};

/// event UserSignedUp { payload: { user: Id, email: Email, at: Timestamp }, delivery: eager }
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct UserSignedUp {
    pub user: Id,
    pub email: Email,
    pub at: Timestamp,
}

/// event UserLoggedIn { payload: { user: Id, at: Timestamp }, delivery: eager, order: by user }
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct UserLoggedIn {
    pub user: Id,
    pub at: Timestamp,
}

/// event UserVerified { payload: { user: Id, at: Timestamp }, delivery: eager }
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct UserVerified {
    pub user: Id,
    pub at: Timestamp,
}

/// event SessionRevoked { payload: { token: Token, user: Id, at: Timestamp }, delivery: eager }
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct SessionRevoked {
    pub token: Token,
    pub user: Id,
    pub at: Timestamp,
}

/// Union of all auth events for the in-process bus.
#[derive(Clone, Debug)]
pub enum AuthEvent {
    UserSignedUp(UserSignedUp),
    UserLoggedIn(UserLoggedIn),
    UserVerified(UserVerified),
    SessionRevoked(SessionRevoked),
}
