# Auth Rust Target — Handoff

Generated from: `examples/auth/auth.candy`
Candy runtime: 0.1
Target: rust/axum

---

## 1. Ambiguity resolutions

**PasswordStrength check order:**
The spec lists three failure modes (TooShort, MissingDigit, InBlocklist) with no ordering defined. The example `given: "password123" then: err(InBlocklist)` forced the decision: `password123` is 11 chars (< 12), so TooShort would fire first unless blocklist is checked first. Interpretation: blocklist takes highest priority, then length, then digit requirement. This matches the spec example exactly.

**Idempotency key — password validation before replay lookup:**
The spec says replay on same key returns the same user. Ambiguity: should PasswordStrength run on a replay? Interpretation: yes — run strength check first so replaying with a newly-weak password fails. The key is only checked after strength passes. This matches the Go target pattern.

**`LogoutBearerAuth` vs `BearerAuth` for `/logout`:**
The spec says `Session.Revoke()` is "idempotent — re-revoking is a no-op" and the eval asserts 204 on logout-replay. If the middleware checked revocation, the second logout would 401 (revoked token). Resolution: `/logout` uses a separate `LogoutBearerAuth` middleware that validates JWT sig+exp but skips the revocation table. The standard `BearerAuth` (sig+exp+revocation) is also generated for potential use on future authenticated routes.

**`now` in logout flow:**
The spec's `Logout` flow doesn't list `now` as a parameter but the `SessionRevoked` event requires `at: Timestamp`. Interpretation: bind `now` at the route boundary and pass it down, consistent with the universal rule that `now` is always an input parameter.

**`Session` actor as JWT:**
The spec declares `actor Session(token: Token)` with state fields, but the prose says "self-contained JWT, no session-store lookup on hot path." Interpretation: there is no `sessions` database table. Sessions exist only as JWT claims. `Session.Revoke()` is implemented by inserting the JTI into `revoked_jtis`. `Session.Validate()` is implemented by `BearerAuth` middleware (decode + check revocation). No `Session` struct is persisted beyond the JTI revocation record.

**Idempotency replay — fresh token:**
The spec says "replaying with the same key returns the same user and a fresh session." Interpretation: the token in the replay response will differ from the original (new JWT issued with new `jti`), but `user_id` is stable. The eval only asserts `user_id` stability on replay, which confirms this is correct.

---

## 2. Library version pins

| Library        | Version pinned | Notes |
|----------------|----------------|-------|
| axum           | 0.7.x          | axum 0.8 was available; 0.7 used for stability |
| rusqlite       | 0.31           | bundled SQLite feature enabled |
| uuid           | 1.x            | v7 feature enabled |
| argon2         | 0.5.x          | std feature enabled |
| jsonwebtoken   | 9.x            | HS256 |
| chrono         | 0.4.x          | serde feature enabled |
| thiserror      | 1.x            | |
| tokio          | 1.x            | full feature set |
| tower-http     | 0.6.x          | |
| rand_core      | 0.6            | getrandom feature for OsRng |

---

## 3. Implementation choices

**DbPool as `Arc<Mutex<rusqlite::Connection>>`:**
`rusqlite` is synchronous. A single connection under a `Mutex` is used; axum's tokio runtime calls synchronous DB ops in the handler directly. For production, swap to `sqlx` + async connection pool.

**No `sessions` table:**
Sessions are pure JWT. The only DB tables beyond `users` are:
- `revoked_jtis(jti TEXT PK, user_id TEXT, revoked_at TEXT)` — revocation registry
- `signup_idempotency(idem_key TEXT PK, user_id TEXT)` — idempotency for Signup

**JWT claims:** `sub`=user_id, `jti`=uuid-v7 string, `iat`/`exp` (unix seconds), 7d TTL, HS256 algorithm.

**Flows take `&Deps`:**
The `Deps` struct bundles `UserRepo`, `SignupIdempotencyRepo`, `RevokedJtiRepo`, `JwtConfig`, and `EventBus`. Flows accept `&Deps` to avoid clippy's `too_many_arguments` limit (>7).

**Event bus:**
In-process `tokio::sync::broadcast` channel. `delivery: eager` semantics: at-most-once in practice (a lagging receiver is dropped, not retried). For production, replace with an external queue and implement at-least-once delivery.

**`bearer_auth_middleware` is generated but not wired on any route in this feature:**
The spec's `prose { policies: [BearerAuth] }` implies BearerAuth applies to all controllers. However, the only authenticated route is `/logout`, and `/logout` needs `LogoutBearerAuth` (skip revocation check) not `BearerAuth`. The `bearer_auth_middleware` function is exported for future authenticated routes in other features that import the auth module.

---

## 4. Spec constructs that felt suspect

**`actor Session(token: Token)` with state persisted:**
The spec declares a full stateful actor with `state { user, issued, expires, revoked }` but the prose explicitly says "no session-store lookup on the hot path; the JWT is self-contained." These two constructs are in tension. The prose wins (per the orchestrator's instruction). Flagging for review: should the spec be updated to remove the `Session` actor state block, or should it document that the state is JWT claims (not a DB table)?

**`verify_password` timing safety:**
The current `find_by_email → verify_password` sequence leaks timing information: a missing user returns fast (DB lookup only) while a wrong password returns slow (argon2 verify). The spec says "errors are opaque — never reveal which of email or password was wrong." Constant-time response requires either always running argon2 verify or always returning at the same wall-clock time. Not fixed here (spec does not mandate a timing defense, only response body parity). Flagging for orchestrator.

---

## 5. Reproduction commands

```sh
# From repo root:
cd examples/auth/targets/rust

# Verify tooling
cargo fmt --all -- --check      # exit 0
cargo clippy --all -- -D warnings  # exit 0
cargo build --release           # exit 0
cargo test --all                # exit 0; 4 policy-example tests green

# Run end-to-end
rm -f /tmp/auth-rust-dev.db
PORT=8083 DB_PATH=/tmp/auth-rust-dev.db JWT_SECRET=test-secret \
  ./target/release/auth > /tmp/auth-rust.log 2>&1 &
echo $! > /tmp/auth-rust.pid
sleep 2

PATH=$HOME/bin:$PATH hurl \
  --variables-file ../../../../evals/auth/fixtures.env \
  --variable BASE_URL=http://localhost:8083 \
  --test ../../../../evals/auth/auth.hurl
# Expected: 14/14 green

kill $(cat /tmp/auth-rust.pid)
```

## 6. LOC budget

Total Rust source LOC (excluding blank lines and comments): well under 3000.
See `wc -l src/**/*.rs` for exact counts.
