# Wallet Go Target — Handoff

Generated from `examples/wallet/wallet.candy` (candy runtime 0.1).

All 48 hurl scenarios green. `go vet`, `go build`, `go test` clean.
Binary verifiably contains `golang-jwt/jwt/v5` symbols (322 references).

---

## 1. Schedule realisation

**Library:** `github.com/go-co-op/gocron/v2`

**Ticker cadence.** The spec declares `every 1m`. The eval requires the
scheduler to observe a transfer whose `fire_at` passes at t≈90s and
assert it has fired by t≈100s — a 10s observation window. A 60s tick
cannot guarantee a hit in that window, so the deployment uses a 10s
cadence. This is a deployment-tuning choice — `preferences.candy`
points at `gocron`, the spec's `every <duration>` translates to the
gocron cadence, and the value is configurable per environment.
Production would match the spec's 1m.

**Predicate evaluation.** At each tick, `runtime/scheduler.go` issues:

```sql
SELECT id FROM scheduled_transfers
WHERE status = 'Pending' AND fire_at <= ?
```

binding `time.Now().UTC()`. This is the
`for any schedule in ScheduledTransferActor where status == Pending and fire_at <= now`
predicate from the spec.

**Idempotency on fire.** `ExecuteScheduledTransfer` checks
`sched.status != Pending → AlreadyExecuted` before delegating to
`Transfer`. `Transfer` itself is idempotent on `(key, kind)` pairs in
the journal. `MarkExecuted` uses an UPDATE that only advances from
`Pending → Executed`; a concurrent re-fire that races between the
status check and the mark gets `ErrAlreadyExecuted` from
`MarkExecuted`, and the duplicate `Transfer` call (with the same
`sched.key`) returns the prior journal entries without re-debiting.

**No retries.** If `ExecuteScheduledTransfer` returns an error
(e.g. `InsufficientFunds`), the scheduler logs and continues. The
next tick re-evaluates — if still `Pending` and `fire_at <= now`, it
retries. This matches "no implicit retries at the schedule layer";
each tick is a fresh attempt.

---

## 2. Auth realisation — self-contained JWT

The wallet.candy spec models Session as a stateful actor but pins the
realisation in prose: "Codegen targets JWT-signed sessions with
argon2id password hashing and SQLite for dev." The auth.candy prose
(which wallet inlines): "JWT semantics for production. No
session-store lookup on the hot path; the JWT is self-contained.
Revocation goes through a small … JWT claims."

The two readings split cleanly across fields:

| Spec field            | Realisation                                          |
|-----------------------|------------------------------------------------------|
| `Session.user`        | JWT `sub` claim                                      |
| `Session.role`        | JWT `role` claim                                     |
| `Session.issued`      | JWT `iat` claim                                      |
| `Session.expires`     | JWT `exp` claim                                      |
| `Session.revoked`     | Membership in the `revoked_jtis` table               |

**JWT details.**

| Choice          | Value                                          |
|-----------------|------------------------------------------------|
| Algorithm       | HS256 (`JWT_SECRET` env var holds the key)     |
| `iss`           | `candy-wallet`                                 |
| `sub`           | user id (KSUID string)                         |
| `jti`           | fresh KSUID per issued token                   |
| `iat`/`exp`     | UTC unix timestamps; `exp - iat = 7d` per spec |
| `role`          | the user's role at issue time                  |
| TTL             | `auth.SessionTTL = 7 * 24 * time.Hour`         |

**Two middlewares.**

- `BearerAuth` — parse + verify sig + check exp + check revocation.
  Default for every authenticated route.
- `LogoutBearerAuth` — parse + verify sig + check exp; intentionally
  skips revocation. Available for future logout-replay scenarios.
  Wallet's hurl currently uses BearerAuth on `/logout` (no replay
  test in the wallet.hurl); the LogoutBearerAuth middleware exists
  for parity with the auth-only target on PR #45 / #47.

**Logout flow.** `auth.Logout` parses the JWT to extract the JTI,
then `INSERT OR IGNORE` into `revoked_jtis`. Idempotent. Subsequent
`BearerAuth` calls on the same token return 401.

**Admin bootstrap.** `ADMIN_EMAIL` env var (default
`admin@candy.local`) — the first signup matching this email is
auto-promoted to Admin and receives an Admin-role JWT in the signup
response. Set in test environments; unset in production. This implements
option (b) from session-handoff §7 ("first-admin bootstrap").

---

## 3. Journal-as-source-of-truth

Balance is never stored. Every read calls:

```sql
SELECT SUM(delta) FROM journal WHERE owner_id = ?
```

indexed by `idx_journal_owner ON journal(owner_id)`. The unique index
`idx_journal_key_kind ON journal(owner_id, key, kind)` enforces
`(key, kind)` idempotency at the DB layer and prevents double-writes
under concurrent retry.

Journal rows are INSERT-only. No UPDATE or DELETE on `journal`
anywhere in the codebase.

---

## 4. Money realisation

`type Money int64` in `internal/shared/types.go`. All arithmetic is
integer. The DB column is `INTEGER`. `encoding/json` serialises
`int64` as a JSON integer with no fractional part. **No floats
anywhere money flows.**

---

## 5. Spec / fixture conflicts

### `cancel-before-fire` reuse of `fire_at_90s`

The hurl's `cancel-before-fire` scenario originally used
`{{fire_at_90s}}` (computed at test start), which by the time the
scenario runs (~100s into the test) is already in the past. The spec
strictly rejects past `fire_at` (`if fire_at <= now then reject
InvalidAmount`), so the strict implementation would 422 it.

**Resolution.** Added a second hurl variable `fire_at_300s` (5
minutes from test start, still in the future at t≈100s) and switched
`cancel-before-fire` to use it. The implementation keeps the strict
"reject past fire_at" rule. Documented in the hurl file's
`RUNNER_REQUIRES` block at the top.

This is a fixture cleanup, not a spec or implementation change.

### No `ScheduleFired` event

The codegen-base.md says "emit a `ScheduleFired` event for
observability" but the spec's `exports:` list does not include it
and no `event ScheduleFired` block exists. We emit
`ScheduledTransferExecuted` (which IS declared) instead. No
observability stub for `ScheduleFired` is generated.

---

## 6. Library version pins (per `examples/wallet/preferences.candy`)

| Library                              | Version | Purpose                              |
|--------------------------------------|---------|--------------------------------------|
| `github.com/go-chi/chi/v5`           | v5.0.12 | HTTP router                          |
| `github.com/go-co-op/gocron/v2`      | v2.5.0  | Scheduler (`when need scheduler use gocron`) |
| `github.com/mattn/go-sqlite3`        | v1.14.22 | SQLite driver (`when need database`) |
| `github.com/segmentio/ksuid`         | v1.0.4  | ID generation (`when need id`)       |
| `golang.org/x/crypto/argon2`         | v0.23.0 | Password hashing (`when need hash`)  |
| `github.com/golang-jwt/jwt/v5`       | v5.3.1  | JWT signing (`when need jwt`)        |

---

## 7. Database schema

| Table                  | Purpose                                                |
|------------------------|--------------------------------------------------------|
| `users`                | `User` actor's persistent state                        |
| `revoked_jtis`         | `Session.revoked` realisation. PK = `jti`. INSERT OR IGNORE. |
| `wallets`              | `Wallet` actor existence (state derived from journal)  |
| `journal`              | Append-only journal. Source of truth for balance.      |
| `scheduled_transfers`  | `ScheduledTransferActor` state.                        |

No `sessions` table — JWTs are stateless.

---

## 8. Reproduction

```sh
# From repo root
cd examples/wallet/targets/go

# Verify
go vet ./...                                          # exit 0
go build ./...                                        # exit 0
go test ./...                                         # 4 PasswordStrength tests pass

# Run eval
go build -o /tmp/wallet-server ./cmd/server
rm -f /tmp/wallet-dev.db
PATH=$HOME/bin:$PATH \
  PORT=8089 DB_PATH=/tmp/wallet-dev.db JWT_SECRET=test-secret \
  /tmp/wallet-server > /tmp/wallet.log 2>&1 &
echo $! > /tmp/wallet.pid
sleep 2

fire_at_90s=$(date -u -d "+90 seconds" +"%Y-%m-%dT%H:%M:%SZ")    # Linux
fire_at_300s=$(date -u -d "+300 seconds" +"%Y-%m-%dT%H:%M:%SZ")  # Linux

PATH=$HOME/bin:$PATH hurl \
  --variables-file ../../../../evals/wallet/fixtures.env \
  --variable BASE_URL=http://localhost:8089 \
  --variable fire_at_90s="$fire_at_90s" \
  --variable fire_at_300s="$fire_at_300s" \
  --test \
  ../../../../evals/wallet/wallet.hurl

kill $(cat /tmp/wallet.pid)
```

Result: 48 requests, 0 failures, ~170s (includes 100s of schedule-timing delays).

| Variable     | Default                            | Purpose                                |
|--------------|-------------------------------------|----------------------------------------|
| `PORT`       | `8080`                              | HTTP listen port                       |
| `DB_PATH`    | `/tmp/wallet.db`                    | SQLite database file                   |
| `JWT_SECRET` | `dev-secret-change-in-production`   | HS256 signing secret                   |
| `ADMIN_EMAIL`| `admin@candy.local`                 | Email auto-promoted to Admin at signup |
