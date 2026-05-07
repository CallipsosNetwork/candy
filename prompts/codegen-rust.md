# candy codegen — Rust overlay

Apply on top of `codegen-base.md`. Before emitting any Rust, **load the
`rust-best-practices` skill** for idiom guidance (ownership, error types,
async runtime patterns, trait design). This overlay specifies only the
candy-to-Rust bindings.

Default framework: `axum` (HTTP). Default async runtime: `tokio`.
Default DB: per `preferences.candy`; typical is `rusqlite` for examples,
swappable to `sqlx` + Postgres in production.

---

## Project layout

```
targets/rust/
  Cargo.toml
  src/
    main.rs                     — tokio runtime, DI, server bootstrap.
    lib.rs                      — re-exports.
    <feature>/
      mod.rs
      actors.rs
      flows.rs
      controllers.rs
      policies.rs
      events.rs
    shared/
      types.rs                  — branded types.
      events.rs                 — cross-feature events.
      errors.rs                 — declared error variants.
    runtime/
      db.rs
      scheduler.rs
      event_bus.rs
      webhooks.rs
  tests/
    integration.rs              — hurl runner harness.
```

Crate name: `<project_name>`. Edition 2021.

---

## Block-by-block bindings

### `actor`

```rust
pub struct Booking {
    pub id: BookingId,
    pub status: BookingStatus,
    // one field per state entry.
}

pub struct BookingRepo { pool: sqlx::Pool<sqlx::Sqlite> }

impl BookingRepo {
    pub async fn create(&self, init: BookingInit) -> Result<Booking, ActorError> { ... }
    pub async fn find_by_id(&self, id: BookingId) -> Result<Booking, ActorError> { ... }
    pub async fn confirm(&self, id: BookingId, args: ConfirmArgs, now: Timestamp, key: IdempotencyKey)
        -> Result<ConfirmOk, ConfirmErr> { ... }
}
```

- One module per feature.
- The `<Actor>Repo` owns reads/writes for that actor's table only.
- `derive` accessors are `impl` methods returning the computed value.
- `invariant` predicates: enforce in the repo function before commit;
  return a typed error variant on violation.
- `audit` tables: append-only, no `UPDATE`.

### `external actor`

```rust
#[async_trait]
pub trait Payments: Send + Sync {
    async fn charge(&self, amt: Money, src: PaymentMethod, key: IdempotencyKey)
        -> Result<ChargeId, PaymentError>;
}

pub struct StripePayments { client: stripe::Client }

#[async_trait]
impl Payments for StripePayments { ... }
```

- Multi-provider: trait + one impl per tag. A registry HashMap maps
  tag → `Arc<dyn Payments>`.
- Webhook routes: `POST /webhooks/<provider>/<event>`. Verify
  signature using the provider's library, map payload to the declared
  `emits` event, dispatch.

### `flow`

```rust
pub async fn place_booking(
    deps: &Deps,
    args: PlaceBookingArgs,
    now: Timestamp,
    key: IdempotencyKey,
) -> Result<BookingConfirmed, BookingError> {
    let booking_id = BookingId::generate();              // pre-generated; base §5.
    let held = deps.listings.hold_dates(args.listing_id, args.range, booking_id, now)
        .await
        .map_err(|_| BookingError::HoldFailed)?;
    let paid = match deps.payments.charge(args.amount, args.source, key).await {
        Ok(p) => p,
        Err(_) => {
            let _ = deps.listings.release_dates(held).await;     // compensate.
            return Err(BookingError::PaymentDeclined);
        }
    };
    Ok(BookingConfirmed { id: booking_id, charge: paid })
}
```

- `tell` (fire-and-forget): `tokio::spawn` the call; do not await.
- Compensation: explicit; on rejection, run named-step compensators in
  reverse, then return the error variant.
- Dependencies: pass `&Deps` (borrowed) or `Arc<Deps>` if cloned across
  threads.

### `controller`

```rust
pub fn auth_router(deps: Arc<Deps>) -> Router {
    Router::new()
        .route("/signup", post(handle_signup))
        .route("/logout", post(handle_logout).layer(BearerAuth::layer(deps.clone())))
        .with_state(deps)
}

async fn handle_signup(
    State(deps): State<Arc<Deps>>,
    headers: HeaderMap,
    Json(body): Json<SignupBody>,
) -> Response {
    let now = Timestamp::now_utc();
    let key = read_idem_key(&headers);
    match signup(&deps, body, now, key).await {
        Ok(out) => (StatusCode::CREATED, Json(out)).into_response(),
        Err(SignupError::WeakPassword) => bad_request("weak_password"),
        Err(SignupError::EmailTaken)   => conflict("email_taken"),
    }
}
```

- One async function per route.
- `Json(body)` validates via `serde` + `validator` if a `body` shape
  declares constraints.
- `auth: bearer` → an extractor or middleware layer that yields the
  authenticated principal id; 401 on missing/invalid.
- Map every declared `ok(...)` and `err(Variant)` to its declared
  status with its declared body shape.

### `policy`

Pure function; preconditions return `Result<(), PolicyError>`:

```rust
pub fn password_strength(p: &Password) -> Result<(), AuthError> {
    if p.0.len() < 12 { return Err(AuthError::WeakPassword); }
    Ok(())
}
```

`controller`-scope policies become `tower` middleware layers.
`actor`/`flow` scope: explicit call before the protected op.
`prose` scope: apply at every controller and flow in the feature.

### `event`

```rust
#[derive(Clone, Debug)]
pub struct ChargeSucceeded {
    pub charge: ChargeId,
    pub at: Timestamp,
}

pub trait EventBus: Send + Sync {
    fn publish(&self, ev: BoxedEvent) -> impl Future<Output = Result<(), BusError>> + Send;
    fn subscribe<E: 'static>(&self, h: impl Fn(E) -> BoxFuture<'static, Result<(), BusError>> + Send + Sync + 'static);
}
```

- `delivery: strict` → transactional outbox via the same DB
  connection.
- `delivery: eager` → at-least-once channel (`tokio::sync::broadcast`
  for in-process; out-of-process needs a queue per preferences).
- `delivery: weak` → fire-and-forget `tokio::spawn`.

### `type` and `enum`

```rust
#[derive(Clone, Copy, Debug, Eq, PartialEq, Hash)]
pub struct Money(pub i64);                         // unit: minor; currency carried separately.

#[derive(Clone, Debug)]
pub struct Email(String);

impl Email {
    pub fn parse(s: impl Into<String>) -> Result<Self, AuthError> { /* validate format/max */ }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BookingStatus { Pending, Confirmed, Cancelled }

pub type BookingResult = Result<BookingConfirmed, BookingError>;

#[derive(thiserror::Error, Debug)]
pub enum BookingError {
    #[error("hold_failed")]    HoldFailed,
    #[error("payment_declined")] PaymentDeclined,
}
```

- Branded types: newtype struct. `Eq`/`Hash` where the inner allows.
- `Money`/integer minor units: `i64`.
- `decimal` types: `rust_decimal::Decimal`.
- `instant { tz: utc }`: `chrono::DateTime<chrono::Utc>` or
  `time::OffsetDateTime` (per preferences).
- Sum errors: `enum` deriving `thiserror::Error`.

### `schedule`

```rust
async fn run_charge_cycle(deps: Arc<Deps>, now: Timestamp) {
    let subs = deps.subscriptions.query_active().await.unwrap_or_default();
    for sub in subs {
        let _ = charge_cycle(&deps, sub, now).await;
    }
}

// Spawn a tokio::time::interval ticker per declared schedule cadence.
```

- One-shot schedules: store firing time in a `scheduled_jobs` table;
  the runtime sweeps it on the smallest declared interval.

---

## Errors

`thiserror` for declared variants. `anyhow` only at the binary's outer
edge (logging unexpected internal failures). Spec-declared variants are
always typed and exhaustive.

---

## Runtime substrate

- HTTP: `axum` 0.7 + `tower-http`.
- Async: `tokio` (multi-thread).
- DB: per preferences. Examples default to `rusqlite` (sync, simple);
  real apps use `sqlx` async.
- JWT: per preferences. Default: `jsonwebtoken`.
- Hash: per preferences. Default: `argon2` crate.
- ID: per preferences. Default: `uuid` v7 or `ksuid`.
- Logging: `tracing` + `tracing-subscriber`.

---

## Conventions

- Module per feature. `snake_case` modules, `UpperCamelCase` types,
  `snake_case` functions/fields.
- Spec identifier `BookingId` → Rust type `BookingId`; spec field
  `booking_id` → Rust field `booking_id`.
- `cargo fmt` clean. `cargo clippy -- -D warnings` clean.
- No `unwrap()` outside `main.rs` startup or test code. Spec-declared
  error variants are exhaustive.

---

## Verification before reporting done

```sh
cd targets/rust
cargo fmt --all -- --check
cargo clippy --all -- -D warnings
cargo build --release
cargo test
# server up, then:
hurl --variables-file ../../evals/<feature>/fixtures.env \
     --variable BASE_URL=http://localhost:3000 \
     ../../evals/<feature>/<feature>.hurl
```

All five must pass. Don't edit hurl files to make them green; the spec
mapping is the bug.
