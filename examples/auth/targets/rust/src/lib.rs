// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

pub mod auth;
pub mod runtime;
pub mod shared;

use crate::auth::actors::{JwtConfig, RevokedJtiRepo, SignupIdempotencyRepo, UserRepo};
use crate::runtime::{db::DbPool, event_bus::EventBus};

/// Dependency container — wired in main.rs, passed as Arc<Deps> through axum state.
pub struct Deps {
    pub user_repo: UserRepo,
    pub idem_repo: SignupIdempotencyRepo,
    pub revoked_repo: RevokedJtiRepo,
    pub jwt: JwtConfig,
    pub bus: EventBus,
}

impl Deps {
    pub fn new(pool: DbPool, jwt_secret: String) -> Self {
        Deps {
            user_repo: UserRepo { pool: pool.clone() },
            idem_repo: SignupIdempotencyRepo { pool: pool.clone() },
            revoked_repo: RevokedJtiRepo { pool },
            jwt: JwtConfig { secret: jwt_secret },
            bus: EventBus::new(),
        }
    }
}
