# Auth Go Target — Handoff

This file captures judgment calls made during codegen — places where the
spec was ambiguous, where realisation choices were forced, or where the
implementation diverges from a literal reading of the spec for principled
reasons. The next regeneration / next reviewer should read this before
re-walking the same paths.

---

## 1. Session as a self-contained JWT (not a stateful actor)

**Spec text** (`examples/auth/auth.candy`):

The `prose` block reads, in part:

> "The Token type is opaque to the spec but is realized as a JWT by
> [codegen]. Standard payload: id and role, exp = issued + 7d.
> Session.Validate parses the JWT … JWT semantics for production. No
> session-store lookup on the hot path; the JWT is self-contained.
> Revocation goes through a small … JWT claims."

The same file declares `actor Session(token: Token) { state { user, issued,
expires, revoked: bool = false } }` — i.e., a stateful actor with four
fields.

**Reconciliation.** The two readings (stateful actor vs. self-contained
JWT) split cleanly along which field is read on the hot path:

| Spec field | Realisation                                              |
|------------|----------------------------------------------------------|
| `user`     | JWT `sub` claim. No DB lookup.                           |
| `issued`   | JWT `iat` claim. No DB lookup.                           |
| `expires`  | JWT `exp` claim. No DB lookup.                           |
| `revoked`  | Membership in the `revoked_jtis` table. One small lookup. |

`Session.Revoke()` is `INSERT OR IGNORE INTO revoked_jtis (jti, user_id,
revoked_at) VALUES (?, ?, ?)`. Idempotent by construction.

`Session.Validate()` is parse JWT → verify signature → check exp → check
JTI not in `revoked_jtis`. The first three are O(1) with no DB; the last
is one indexed lookup.

The persistent `sessions` table from the v0.1.0 KSUID-backed
implementation is gone. Tokens themselves are stateless.

## 2. JWT details

| Choice                 | Value                                              |
|------------------------|----------------------------------------------------|
| Signing algorithm      | HS256 (symmetric; `JWT_SECRET` env var holds the secret) |
| `iss`                  | `candy-auth`                                       |
| `sub`                  | the user id (KSUID string)                         |
| `jti`                  | a fresh KSUID per issued token                     |
| `iat`, `exp`           | UTC unix timestamps; `exp - iat = 7d` per spec     |
| TTL constant           | `auth.SessionTTL = 7 * 24 * time.Hour`             |
| Issuer code            | `JWTService.Issue(userID, jti, now)`               |
| Parser code            | `JWTService.Parse(token, now)` — verifies sig + exp; does NOT check revocation |

Revocation is a separate concern, available as `RevokedRepo.IsRevoked(jti)`.
This split lets `LogoutBearerAuth` (§3) skip revocation while
`BearerAuth` enforces it.

## 3. `auth: bearer` — two middlewares, principled split

The spec puts `policies: [BearerAuth]` at prose scope (every
controller/flow gets it) and `auth: bearer` on `POST /logout` and any
future authenticated route. Two interpretation forks:

- **Strict** — `auth: bearer` means "the session must currently be
  live." A revoked JWT → 401.
- **Liberal** — `auth: bearer` means "a bearer token must be present
  and authentic." A revoked JWT for the right user → still let through.

The eval (`evals/auth/auth.hurl`, scenario `logout-replay`) requires
that re-sending a revoked token to `/logout` returns 204 — i.e., logout
must be idempotent at the HTTP boundary, not just the flow boundary. So
strict middleware on `/logout` would conflict with the eval, while strict
middleware on every OTHER bearer route is the right default.

**Resolution.** Two middlewares:

- `BearerAuth`: parse + verify sig + check exp + **check revocation**.
  Used by every authenticated route except `/logout`.
- `LogoutBearerAuth`: parse + verify sig + check exp; **no revocation
  check**. Used only by `/logout`.

When an authenticated route beyond `/logout` lands, it uses `BearerAuth`.

## 4. PasswordStrength — strict to spec

The spec policy is "length ≥ 12, ≥1 letter, ≥1 digit, not blocklisted",
with the example `"correct horse battery staple 9"` (note the trailing
digit) → ok. The hurl fixture (`evals/auth/fixtures.env`) was missing
the digit on `alice_password` and `bob_password`; it has been corrected
to include the digit. The implementation is the literal spec rule with
no passphrase exemption.

## 5. Idempotency replay shape

Spec: `flow Signup(email, password, now, key) -> Result<{ user, token },
WeakPassword | EmailTaken>`. Comment in the flow body: replay returns
the prior `user_id` with a fresh session.

Implementation:

- `signup_idempotency` table holds `(key, user_id)`. The token is **not**
  stored — the JWT issued on first signup has no special role on replay,
  and storing it would let an attacker who steals an idempotency key
  resurrect the original token.
- Replay: look up by key → found → issue a fresh JWT for the stored
  user_id → return `(user_id, fresh_token, 201)`.
- The eval's `signup-idempotency-replay` scenario asserts
  `user_id == alice_user_id` (must be equal to the original) and
  `token isString && token != ""` (must be present, no equality check).

## 6. SessionRevoked event payload

Spec: `event SessionRevoked { payload: { token: Token, user: Id, at:
Timestamp }, delivery: eager }`.

Earlier KSUID implementation dropped `token` from the event payload
under "tokens never log." That conflated two rules: tokens never appear
in **log lines** (slog calls, error responses), but event payloads are
internal data carried between subscribers, not log surfaces. The spec
includes `token` for a reason — downstream subscribers (notifications,
audit) may need to identify the revoked session.

`token` is back in the event. The audit table (`audit_session_revoked`)
records the JTI, not the full JWT, since the JTI is the durable
identifier and the JWT is reconstructible only from the secret.

## 7. Library version pins

| Library                              | Version  | Source                                  |
|--------------------------------------|----------|-----------------------------------------|
| `github.com/go-chi/chi/v5`           | v5.2.5   | `go mod tidy` resolved latest           |
| `github.com/golang-jwt/jwt/v5`       | v5.3.1   | per `preferences.candy` `when need jwt` |
| `github.com/mattn/go-sqlite3`        | v1.14.44 | `go mod tidy` resolved latest           |
| `github.com/segmentio/ksuid`         | v1.0.4   | per `preferences.candy` `when need id`  |
| `golang.org/x/crypto`                | v0.50.0  | `argon2id` per `preferences.candy`      |

## 8. Database schema

| Table                   | Purpose                                                   |
|-------------------------|-----------------------------------------------------------|
| `users`                 | `User` actor's persistent state.                          |
| `revoked_jtis`          | `Session.revoked` realisation. PK = `jti`. INSERT OR IGNORE. |
| `signup_idempotency`    | `(key, user_id)` for replay.                              |
| `audit_user_verified`   | Append-only audit for `User.Verify`.                      |
| `audit_session_revoked` | Append-only audit for the SessionRevoked event.            |

No `sessions` table — JWTs are stateless.

## 9. Reproduction

```sh
# Build
cd examples/auth/targets/go
go build -o /tmp/auth-server ./cmd/server

# Start (fresh DB, free port, env-var secret)
rm -f /tmp/auth-jwt-dev.db
PATH=$HOME/bin:$PATH \
PORT=8080 DB_PATH=/tmp/auth-jwt-dev.db JWT_SECRET=test-secret \
  /tmp/auth-server > /tmp/auth-server.log 2>&1 &
echo $! > /tmp/auth-server.pid
sleep 1

# Run conformance
hurl --variables-file evals/auth/fixtures.env \
     --variable BASE_URL=http://localhost:8080 \
     --test evals/auth/auth.hurl

# Cleanup
kill $(cat /tmp/auth-server.pid)
```

| Variable     | Default                            | Purpose                             |
|--------------|-------------------------------------|-------------------------------------|
| `PORT`       | `8080`                              | HTTP listen port                    |
| `DB_PATH`    | `/tmp/auth-dev.db`                  | SQLite database file                |
| `JWT_SECRET` | `dev-secret-change-in-production`   | HS256 signing secret                |

## 10. Future work

- A `GET /me` (or similar) endpoint backed by `User(id)` would exercise
  `BearerAuth`'s revocation check end-to-end. Today the only bearer
  consumer is `/logout`, which intentionally bypasses revocation.
- Token rotation on the idempotency replay path (currently issues a
  fresh JWT on every replay, which means a leaked replay can issue
  unbounded tokens).
- Argon2id parameters are conservative for dev; production should tune
  time/memory cost per the deployment's hardware budget. Move to a
  config file or env-var-driven knob.
- Rate limiting on `/login` and `/signup` is not declared by the spec
  but is a normal hardening step.
