// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

use auth::{auth::controllers::auth_router, runtime::db, Deps};
use std::{env, net::SocketAddr, sync::Arc};
use tracing::info;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "auth=info,tower_http=info".parse().unwrap()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    let port: u16 = env::var("PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(8080);

    let db_path = env::var("DB_PATH").unwrap_or_else(|_| "/tmp/auth-dev.db".into());
    let jwt_secret = env::var("JWT_SECRET").unwrap_or_else(|_| "dev-secret-change-me".into());

    let pool = db::open(&db_path)?;
    let deps = Arc::new(Deps::new(pool, jwt_secret));

    let app = auth_router(deps);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    info!("listening on {addr}");

    let listener = tokio::net::TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
