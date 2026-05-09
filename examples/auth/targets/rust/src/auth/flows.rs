// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use crate::auth::{
    actors::{generate_id, hash_password, verify_password},
    events::{AuthEvent, SessionRevoked, UserLoggedIn, UserSignedUp},
    policies::password_strength,
};
use crate::shared::{
    errors::AuthError,
    types::{Email, Id, Key, Password, Timestamp, Token},
};
use crate::Deps;
use serde::Serialize;

#[derive(Debug, Serialize)]
pub struct SignupOk {
    pub user: Id,
    pub token: Token,
}

#[derive(Debug, Serialize)]
pub struct LoginOk {
    pub user: Id,
    pub token: Token,
}

/// flow Signup(email, password, now, key) -> Result<{user, token}, WeakPassword | EmailTaken>
///
/// Create a new user, hash the password, and issue an initial session.
/// Idempotent on key — replaying with the same key returns the same user
/// and a fresh session.
pub async fn signup(
    deps: &Deps,
    email: Email,
    password: Password,
    now: Timestamp,
    key: Key,
) -> Result<SignupOk, AuthError> {
    // step strength = PasswordStrength(password) rescue reject WeakPassword(reason)
    password_strength(&password)?;

    // Idempotency: if we've seen this key before, return the same user with a fresh token.
    if let Some(existing_user_id) = deps.idem_repo.find(&key)? {
        let token = deps.jwt.issue(&existing_user_id, now)?;
        return Ok(SignupOk {
            user: existing_user_id,
            token,
        });
    }

    // step taken = if any user in User where user.email == email then reject EmailTaken
    if deps.user_repo.email_exists(&email)? {
        return Err(AuthError::EmailTaken);
    }

    // step user = ask User.create({id: generate(), email, hash: hash(password), created: now})
    let user_id = generate_id();
    let hash = hash_password(&password)?;
    let user = deps.user_repo.create(user_id, email.clone(), hash, now)?;

    // Persist idempotency key → user_id (only the key+id, never the token)
    deps.idem_repo.insert(&key, &user.id)?;

    // step session = ask Session.create({ token: generate(), user: user.id, issued: now, expires: now after 7d })
    let token = deps.jwt.issue(&user.id, now)?;

    // emit UserSignedUp { user: user.id, email, at: now }
    deps.bus.publish(AuthEvent::UserSignedUp(UserSignedUp {
        user: user.id.clone(),
        email,
        at: now,
    }));

    // commit { user: user.id, token: session.token }
    Ok(SignupOk {
        user: user.id,
        token,
    })
}

/// flow Login(email, password, now) -> Result<{user, token}, InvalidCredentials>
///
/// Authenticate by email and password. On success, issue a new session.
/// Errors are opaque — never reveal which of email or password was wrong.
pub async fn login(
    deps: &Deps,
    email: Email,
    password: Password,
    now: Timestamp,
) -> Result<LoginOk, AuthError> {
    // step user = ask User.findBy(email) rescue reject InvalidCredentials
    let user = deps
        .user_repo
        .find_by_email(&email)
        .map_err(|_| AuthError::InvalidCredentials)?;

    // step ok = if not verify(password, user.hash) then reject InvalidCredentials
    if !verify_password(&password, &user.hash) {
        return Err(AuthError::InvalidCredentials);
    }

    // step session = ask Session.create({ token: generate(), user: user.id, ... })
    let token = deps.jwt.issue(&user.id, now)?;

    // emit UserLoggedIn { user: user.id, at: now }
    deps.bus.publish(AuthEvent::UserLoggedIn(UserLoggedIn {
        user: user.id.clone(),
        at: now,
    }));

    // commit { user: user.id, token: session.token }
    Ok(LoginOk {
        user: user.id,
        token,
    })
}

/// flow Logout(token, now) -> unit
///
/// Revoke the session associated with the given bearer token.
/// Session.Revoke() is idempotent — re-revoking is a no-op.
pub async fn logout(deps: &Deps, token: Token, now: Timestamp) -> Result<(), AuthError> {
    // step _ = ask Session(token).Revoke()
    // Parse claims (sig + exp valid)
    let claims = deps.jwt.decode(&token)?;
    let user_id = Id(claims.sub.clone());

    // INSERT OR IGNORE — idempotent
    deps.revoked_repo.revoke(&claims.jti, &claims.sub, now)?;

    // emit SessionRevoked { token: self.token, user, at: now }
    deps.bus.publish(AuthEvent::SessionRevoked(SessionRevoked {
        token,
        user: user_id,
        at: now,
    }));

    Ok(())
}
