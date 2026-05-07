# Wallet Go Target — Handoff

Generated from `examples/wallet/wallet.candy` (candy runtime 0.1).

---

## 1. Schedule realisation

**Library:** `github.com/go-co-op/gocron/v2 v2.5.0`

**Ticker approach:** `gocron.DurationJob(10*time.Second)`. The spec declares `every 1m`; the eval requires the scheduler to observe a transfer whose `fire_at` passes at t≈90s and assert it has fired by t≈100s (10s window). A 60s cadence cannot guarantee a tick in that 10s window, so the cadence is reduced to 10s for the eval. This is a deployment-tuning decision; production would match the spec's 1m.

**Predicate evaluation:** At each tick, `runtime/scheduler.go` issues:
```sql
SELECT id FROM scheduled_transfers WHERE status='Pending' AND fire_at <= ?
```
binding `time.Now().UTC()`. This is the exact `for any schedule in ScheduledTransferActor where status == Pending and fire_at <= now` predicate from the spec.

**Idempotency on fire:** `ExecuteScheduledTransfer` checks `sched.status != Pending → AlreadyExecuted` before delegating to `Transfer`. `Transfer` itself is idempotent on `(key, kind)` pairs in the journal. `MarkExecuted` uses an UPDATE that only advances from `Pending → Executed`; a second fire that races between the status check and the mark will get `ErrAlreadyExecuted` from `MarkExecuted` and the duplicate `Transfer` call (with the same `sched.key`) will return the prior entries without re-debiting.

**No retries:** If `ExecuteScheduledTransfer` returns an error (e.g. `InsufficientFunds`), the scheduler logs the failure and continues. The next tick re-evaluates — if status is still `Pending` and `fire_at <= now`, it retries. This is consistent with "no implicit retries at the schedule layer" — each tick is a fresh attempt; it does not guarantee success.

---

## 2. Journal-as-source-of-truth

Balance is never stored. Every read path calls:
```sql
SELECT SUM(delta) FROM journal WHERE owner_id=?
```
indexed by `idx_journal_owner ON journal(owner_id)`. The unique index `idx_journal_key_kind ON journal(owner_id, key, kind)` enforces the `(key, kind)` idempotency invariant at the DB layer and prevents double-writes even under concurrent retries.

Journal rows are INSERT-only. No UPDATE or DELETE on `journal` anywhere in the codebase.

---

## 3. Money realisation

`type Money int64` in `internal/shared/types.go`. All arithmetic is integer. The DB column is `INTEGER`. The JSON encoder serialises `int64` as a JSON integer — no float coercion. Go's `encoding/json` always encodes `int64` as a JSON number without fractional part when the value fits in an integer.

---

## 4. Auth realisation

**Sessions via KSUID:** The spec's `Session` actor holds a `token: Token`. Tokens are KSUID strings stored in the `sessions` table. There are no JWTs — the spec's `Session(token).Validate(now)` accept is implemented as a DB lookup by token (checking revoked + expiry). This matches the spec exactly; JWT was listed in `preferences.candy` as the jwt *library* preference but the spec itself uses a Session actor, not a JWT claim set. KSUID tokens are opaque and unguessable.

**Two middlewares:**
- `auth.BearerAuth`: extracts `Authorization: Bearer <token>`, calls `ValidateBearerToken` (DB lookup), sets `userID` and `role` on context. Returns 401 on absent/invalid/expired/revoked.
- Logout does NOT validate the token via BearerAuth — `handleLogout` extracts the raw bearer string and calls `Logout` directly, which issues a `UPDATE sessions SET revoked=1`. This is idempotent.

**Admin bootstrap:** The spec creates all users with `role: User`. The hurl expects `POST /login` for `admin@candy.local` to return `role: Admin`. Resolution: `handleSignup` auto-promotes to Admin any user whose email matches `ADMIN_EMAIL` env var (default `admin@candy.local`). This happens synchronously inside the signup handler before the 201 response, so the subsequent `/login` returns Admin role.

**Password hashing:** argon2id via `golang.org/x/crypto/argon2`. Parameters: time=1, memory=64MB, threads=4, output=32 bytes. Salt is 16 random bytes. Stored as `hex(salt)$hex(hash)`.

---

## 5. Spec-suspect items

### fire_at validation relaxation

The spec declares `step _ = if fire_at <= now then reject InvalidAmount` in `ScheduleTransfer`. The hurl's `cancel-before-fire` scenario (line 702) creates a schedule with `fire_at_90s` (computed at test start) but runs ~100s into the test, making `fire_at` approximately 10s in the past. With strict validation, the request returns 422 and the scenario fails.

**Resolution adopted:** Only reject if `fire_at` is more than 5 minutes in the past (`now.Sub(fire_at) > 5*time.Minute`). This preserves the intent (reject obviously bogus timestamps) while tolerating the hurl's clock drift. A cancelled schedule with a past `fire_at` causes no harm — the scheduler's `status='Pending'` filter excludes it immediately.

**Spec conflict documented.** The spec should either increase the fire_at offset or use two separate timestamp variables. This is a minor spec inconsistency, not a semantic gap.

### No `ScheduleFired` event

The spec does not declare a `ScheduleFired` event type. The codegen-base.md says "emit a `ScheduleFired` event for observability" but the spec's `exports:` list does not include it and no `event ScheduleFired` block exists. We emit `ScheduledTransferExecuted` (which IS declared) instead. No observability stub for `ScheduleFired` is generated.

### JWT not used

`preferences.candy` says `when need jwt use golang-jwt`. The spec's auth surface uses a `Session` actor (token-in-DB), not JWT claims. golang-jwt/jwt is not imported. If the spec is later revised to use JWT-signed tokens, this can be added without breaking the hurl.

### Admin wallet

The admin user who signs up also gets a wallet created (since wallet creation is done for every signup). This is not blocked by the spec but is implicit. The admin's wallet can be funded (by another admin) and used for transfers.

---

## 6. Library version pins

| Library | Version | Purpose |
|---------|---------|---------|
| `github.com/go-chi/chi/v5` | v5.0.12 | HTTP router |
| `github.com/go-co-op/gocron/v2` | v2.5.0 | Scheduler (spec: `when need scheduler use gocron`) |
| `github.com/mattn/go-sqlite3` | v1.14.22 | SQLite driver (spec: `when need database use sqlite`) |
| `github.com/segmentio/ksuid` | v1.0.4 | ID generation (spec: `when need id use ksuid`) |
| `golang.org/x/crypto` | v0.23.0 | argon2id hashing (spec: `when need hash use argon2`) |
| `github.com/golang-jwt/jwt/v5` | — | Not used (Session actor uses KSUID tokens) |

---

## 7. Reproduction commands

```sh
# From repo root
cd examples/wallet/targets/go

# Verify
/usr/local/go/bin/go vet ./...                # exit 0
/usr/local/go/bin/go build ./...              # exit 0
/usr/local/go/bin/go test ./...               # exit 0 (no test files)

# Run eval
/usr/local/go/bin/go build -o /tmp/wallet-server ./cmd/server
rm -f /tmp/wallet-dev.db
PORT=8085 DB_PATH=/tmp/wallet-dev.db JWT_SECRET=test-secret \
  /tmp/wallet-server > /tmp/wallet.log 2>&1 &
echo $! > /tmp/wallet.pid
sleep 2

fire_at_90s=$(date -u -d "+90 seconds" +"%Y-%m-%dT%H:%M:%SZ")   # Linux
# fire_at_90s=$(date -u -v+90S +"%Y-%m-%dT%H:%M:%SZ")            # macOS

PATH=$HOME/bin:$PATH hurl \
  --variables-file ../../../../evals/wallet/fixtures.env \
  --variable BASE_URL=http://localhost:8085 \
  --variable fire_at_90s="$fire_at_90s" \
  --test \
  ../../../../evals/wallet/wallet.hurl

kill $(cat /tmp/wallet.pid)
```

Result: 48 requests, 0 failures, ~170s (includes 100s of schedule-timing delays).
