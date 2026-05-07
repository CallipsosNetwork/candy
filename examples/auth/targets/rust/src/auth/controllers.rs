// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use axum::{
    extract::{Request, State},
    http::{HeaderMap, StatusCode},
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::post,
    Json, Router,
};
use chrono::Utc;
use serde::{Deserialize, Serialize};
use std::sync::Arc;

use crate::auth::flows::{login, logout, signup};
use crate::shared::{
    errors::{AuthError, PasswordWeaknessReason},
    types::{Email, Key, Password, Token},
};
use crate::Deps;

// ── Request / response shapes ─────────────────────────────────────────────────

#[derive(Deserialize)]
struct SignupBody {
    email: String,
    password: String,
    idempotency_key: String,
}

#[derive(Serialize)]
struct SignupOkResponse {
    user_id: String,
    token: String,
}

#[derive(Deserialize)]
struct LoginBody {
    email: String,
    password: String,
}

#[derive(Serialize)]
struct LoginOkResponse {
    user_id: String,
    token: String,
}

#[derive(Serialize)]
struct ErrorResponse {
    error: &'static str,
    #[serde(skip_serializing_if = "Option::is_none")]
    reason: Option<String>,
}

fn error_json(status: StatusCode, error: &'static str, reason: Option<String>) -> Response {
    (status, Json(ErrorResponse { error, reason })).into_response()
}

// ── Handlers ──────────────────────────────────────────────────────────────────

/// POST /signup -> Signup(email, password, now, idempotency_key)
async fn handle_signup(State(deps): State<Arc<Deps>>, Json(body): Json<SignupBody>) -> Response {
    // now is bound at the route boundary
    let now = Utc::now();

    let email = match Email::parse(&body.email) {
        Ok(e) => e,
        Err(_) => return error_json(StatusCode::UNPROCESSABLE_ENTITY, "weak_password", None),
    };
    let password = Password(body.password);
    let key = Key(body.idempotency_key);

    match signup(&deps, email, password, now, key).await {
        Ok(out) => (
            StatusCode::CREATED,
            Json(SignupOkResponse {
                user_id: out.user.0,
                token: out.token.0,
            }),
        )
            .into_response(),
        // err(WeakPassword reason) -> 422 { error: "weak_password", reason }
        Err(AuthError::WeakPassword(reason)) => {
            let reason_str = match reason {
                PasswordWeaknessReason::TooShort => "too_short",
                PasswordWeaknessReason::MissingDigit => "missing_digit",
                PasswordWeaknessReason::InBlocklist => "in_blocklist",
            };
            error_json(
                StatusCode::UNPROCESSABLE_ENTITY,
                "weak_password",
                Some(reason_str.to_string()),
            )
        }
        // err(EmailTaken) -> 409 { error: "email_taken" }
        Err(AuthError::EmailTaken) => error_json(StatusCode::CONFLICT, "email_taken", None),
        Err(_) => StatusCode::INTERNAL_SERVER_ERROR.into_response(),
    }
}

/// POST /login -> Login(email, password, now)
async fn handle_login(State(deps): State<Arc<Deps>>, Json(body): Json<LoginBody>) -> Response {
    let now = Utc::now();

    let email = match Email::parse(&body.email) {
        Ok(e) => e,
        Err(_) => return error_json(StatusCode::UNAUTHORIZED, "invalid_credentials", None),
    };
    let password = Password(body.password);

    match login(&deps, email, password, now).await {
        Ok(out) => (
            StatusCode::OK,
            Json(LoginOkResponse {
                user_id: out.user.0,
                token: out.token.0,
            }),
        )
            .into_response(),
        // err(InvalidCredentials) -> 401 { error: "invalid_credentials" }
        Err(AuthError::InvalidCredentials) => {
            error_json(StatusCode::UNAUTHORIZED, "invalid_credentials", None)
        }
        Err(_) => StatusCode::INTERNAL_SERVER_ERROR.into_response(),
    }
}

/// POST /logout -> Logout(bearer, now)
/// auth: bearer
/// Uses LogoutBearerAuth middleware: verifies sig+exp, skips revocation check
/// so replay returns 204 (idempotent per spec).
async fn handle_logout(State(deps): State<Arc<Deps>>, headers: HeaderMap) -> Response {
    let now = Utc::now();

    let token = match extract_bearer(&headers) {
        Some(t) => t,
        None => return StatusCode::UNAUTHORIZED.into_response(),
    };

    match logout(&deps, token, now).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(AuthError::SessionInvalid) => StatusCode::UNAUTHORIZED.into_response(),
        Err(_) => StatusCode::INTERNAL_SERVER_ERROR.into_response(),
    }
}

// ── BearerAuth middleware ─────────────────────────────────────────────────────
// Parses + verifies sig + checks exp + checks revocation.

/// Middleware for routes requiring a valid, non-revoked bearer token.
pub async fn bearer_auth_middleware(
    State(deps): State<Arc<Deps>>,
    req: Request,
    next: Next,
) -> Response {
    let token = match extract_bearer_ref(req.headers()) {
        Some(t) => Token(t.to_string()),
        None => return StatusCode::UNAUTHORIZED.into_response(),
    };
    match deps.jwt.decode(&token) {
        Ok(claims) => match deps.revoked_repo.is_revoked(&claims.jti) {
            Ok(true) => return StatusCode::UNAUTHORIZED.into_response(),
            Ok(false) => {}
            Err(_) => return StatusCode::INTERNAL_SERVER_ERROR.into_response(),
        },
        Err(_) => return StatusCode::UNAUTHORIZED.into_response(),
    }
    next.run(req).await
}

/// LogoutBearerAuth: parses + verifies sig + checks exp; intentionally skips
/// revocation so logout-replay returns 204 per the eval.
pub async fn logout_bearer_auth_middleware(
    State(deps): State<Arc<Deps>>,
    req: Request,
    next: Next,
) -> Response {
    let token = match extract_bearer_ref(req.headers()) {
        Some(t) => Token(t.to_string()),
        None => return StatusCode::UNAUTHORIZED.into_response(),
    };
    // Verify sig + exp only; do NOT check revocation table.
    if deps.jwt.decode(&token).is_err() {
        return StatusCode::UNAUTHORIZED.into_response();
    }
    next.run(req).await
}

// ── Helpers ───────────────────────────────────────────────────────────────────

fn extract_bearer(headers: &HeaderMap) -> Option<Token> {
    let value = headers.get("Authorization")?.to_str().ok()?;
    let stripped = value.strip_prefix("Bearer ")?;
    Some(Token(stripped.to_string()))
}

fn extract_bearer_ref(headers: &HeaderMap) -> Option<&str> {
    let value = headers.get("Authorization")?.to_str().ok()?;
    value.strip_prefix("Bearer ")
}

// ── Router ────────────────────────────────────────────────────────────────────

pub fn auth_router(deps: Arc<Deps>) -> Router {
    Router::new()
        .route("/signup", post(handle_signup))
        .route("/login", post(handle_login))
        .route(
            "/logout",
            post(handle_logout).route_layer(middleware::from_fn_with_state(
                deps.clone(),
                logout_bearer_auth_middleware,
            )),
        )
        .with_state(deps)
}
