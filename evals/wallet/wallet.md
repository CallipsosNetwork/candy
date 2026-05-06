# Wallet — scenario narrative

**Feature:** `wallet` (standalone, inlined auth)
**Spec under test:** `examples/wallet/wallet.candy`
**Hurl file:** `evals/wallet/wallet.hurl`
**Total scenarios:** 32
**Coverage issues:** #11, #28

---

## Setup

One admin and four users are bootstrapped in order: admin, alice, bob, charlie,
dave. After signup, all five log in to obtain bearer tokens. The admin then
promotes itself to role Admin (signup creates all accounts as role User). Once
the admin token carries the Admin role, the admin funds Alice's wallet and Bob's
wallet — those credits are the starting balances for the rest of the scenarios.

No teardown — the backend starts with empty state.

**Starting balances after setup:**
- Alice: `{{fund_amount}}` (50 000 minor units)
- Bob: `{{fund_amount}}` (50 000 minor units)
- Charlie: 0 (no funding, used for InsufficientFunds)
- Dave: 0 (no funding, destination only)

---

## Scenarios

### Group 1 — Auth basics

These are kept minimal. The standalone `auth` eval covers every password-strength
variant and opaque InvalidCredentials parity. Here we only confirm that each actor
can reach a valid session token and that the token works on bearer-gated routes.

#### 1. signup-admin
Signup the admin account. Captures `admin_user_id`.

#### 2. signup-alice
Signup Alice. Captures `alice_user_id`.

#### 3. signup-bob
Signup Bob. Captures `bob_user_id`.

#### 4. signup-charlie
Signup Charlie. Captures `charlie_user_id`.

#### 5. signup-dave
Signup Dave. Captures `dave_user_id`.

#### 6. login-admin
Login admin. Captures `admin_token`. The admin is role User at this point;
promotion in scenario 7 upgrades the role for all subsequent calls.

#### 7. promote-admin
`POST /admin/users/:id/promote` with `{ role: "Admin" }`. At this point the
admin is still role User, so this call must fail — 403. This is intentional:
the admin must be promoted by an existing Admin. Since there is no pre-seeded
admin, the spec requires that the first Admin is bootstrapped via the
`POST /admin/users/:id/promote` endpoint once a User with role Admin already
exists, OR that the first admin is seeded out-of-band.

**Deferral note:** In a fresh-state backend there is no initial Admin. The
canonical resolution is a bootstrap seed (the backend starts with one admin
seeded from deployment config) or an env-gated "first admin" endpoint. Until
codegen defines that bootstrap path this scenario documents the gap.
The wallet hurl works around this by relying on a `ADMIN_TOKEN` variable
injected by the test runner (see runner contract below). If `ADMIN_TOKEN` is not
provided, the runner must seed the admin account via out-of-band fixture injection
before the hurl file runs.

**Runner contract for admin bootstrap:**
The test runner must either:
- Start the backend with a seeded admin account matching `{{admin_email}}` /
  `{{admin_password}}`, so that `POST /login` with those credentials returns a
  token with role Admin; or
- Inject `admin_token` as a hurl variable pointing to a pre-issued Admin session.

The hurl file assumes the first approach: it logs in with admin credentials and
expects the returned token to carry role Admin.

#### 8. login-alice / login-bob / login-charlie / login-dave
Capture `alice_token`, `bob_token`, `charlie_token`, `dave_token`.

#### 9. logout-alice (auth basics)
`POST /logout` with `alice_token`. Expects 204. Then login Alice again to restore
her token for the rest of the file.

---

### Group 2 — Admin funding

#### 10. fund-alice — happy path
`POST /admin/wallets/{{alice_user_id}}/fund` with `{{fund_amount}}` and
`{{fund_alice_key}}`. Expects 201; response body contains `entry` with
`kind == "Fund"` and `delta == {{fund_amount}}`.

#### 11. fund-bob — happy path
`POST /admin/wallets/{{bob_user_id}}/fund` with `{{fund_amount}}` and
`{{fund_bob_key}}`. Same assertions as scenario 10.

#### 12. fund-invalid-amount — InvalidAmount
`POST /admin/wallets/{{alice_user_id}}/fund` with `amount: {{invalid_amount}}`
(`-1000`). Expects 422, `{"error": "invalid_amount"}`.

#### 13. fund-wallet-not-found — WalletNotFound
`POST /admin/wallets/unknown-wallet-id-999/fund` with `{{fund_amount}}`.
Expects 404, `{"error": "wallet_not_found"}`.

#### 14. fund-wrong-role — User calls admin route
`POST /admin/wallets/{{alice_user_id}}/fund` authenticated as Alice (role User).
Expects 403, `{"error": "not_authorized"}`. This exercises AdminGated rejecting
a non-admin caller.

---

### Group 3 — Wallet reads

#### 15. get-balance-alice — happy path
`GET /wallets/me` as Alice. Expects 200, `{"balance": {{fund_amount}}}`.

---

### Group 4 — User self-actions (Withdraw)

WalletOwner policy: `session.user == wallet.owner`. Admin role does NOT bypass
this — admins can fund wallets but cannot drain them.

#### 16. withdraw-alice — happy path
`POST /wallets/me/withdraw` as Alice with `{{withdraw_amount}}` and
`{{withdraw_alice_key}}`. Expects 201; entry `kind == "Withdrawal"`,
`delta == -{{withdraw_amount}}`.

Alice's balance after: `fund_amount - withdraw_amount` = 40 000.

#### 17. withdraw-insufficient-funds — InsufficientFunds
`POST /wallets/me/withdraw` as Charlie (balance 0) with `{{withdraw_amount}}`.
Expects 409, `{"error": "insufficient_funds"}`.

#### 18. withdraw-invalid-amount — InvalidAmount
`POST /wallets/me/withdraw` as Alice with `amount: {{invalid_amount}}` (`-1000`).
Expects 422, `{"error": "invalid_amount"}`.

#### 19. withdraw-wrong-owner — WalletOwner violation
The `POST /wallets/me/withdraw` route binds `wallet = self` (the caller's own
wallet), so there is no way to pass a different wallet id via this route. The
wrong-owner case is tested at the transfer layer (scenario 22) where the `from`
field is explicit, and via `POST /admin/wallets/:owner/fund` (scenario 14) where
a User caller is rejected by AdminGated. Direct WalletOwner on Withdraw is
structurally enforced by the route shape.

**Coverage note:** The spec's WalletOwner policy says `caller id != wallet.owner
→ err(NotAuthorized)`. The `/wallets/me/withdraw` route always sets
`wallet = self`, making a cross-owner withdraw impossible at the HTTP level.
A backend that exposes a different route shape (e.g. `/wallets/:id/withdraw`)
would need a direct wrong-owner test. This is documented here but the scenario
is marked `[~]` in COVERAGE.md — covered by route-level enforcement, not a
separate negative test.

---

### Group 5 — Peer-to-peer Transfer

Transfer uses WalletOwner on the source leg (`from = self`) and TransferAtomicity
for the two-leg saga. Every request includes an idempotency key.

#### 20. transfer-alice-to-bob — happy path
`POST /transfers` as Alice with `to: {{bob_user_id}}`, `{{transfer_amount}}`,
`{{transfer_alice_to_bob_key}}`. Expects 201; body has `out.kind == "TransferOut"`,
`in.kind == "TransferIn"`, both entries non-empty. Captures `transfer_out_id` and
`transfer_in_id`.

Balances after:
- Alice: 40 000 − 5 000 = 35 000
- Bob: 50 000 + 5 000 = 55 000

#### 21. transfer-insufficient-funds — InsufficientFunds
`POST /transfers` as Charlie (balance 0) with `to: {{dave_user_id}}`,
`{{transfer_amount}}`. Expects 409, `{"error": "insufficient_funds"}`.

#### 22. transfer-self — SelfTransfer
`POST /transfers` as Alice with `to: {{alice_user_id}}` (same as caller).
Expects 422, `{"error": "self_transfer"}`. No journal entry appended.

#### 23. transfer-wallet-not-found — WalletNotFound
`POST /transfers` as Alice with `to: "unknown-user-id-999"`.
Expects 404, `{"error": "wallet_not_found"}`. TransferAtomicity: Wallet(from).Debit
is not issued until after both wallets are found, so Alice's balance is unchanged.

#### 24. transfer-wrong-owner — WalletOwner violation
`POST /transfers` as Bob with `to: {{dave_user_id}}` but using `from` set to
Alice's wallet id (i.e., Bob tries to drain Alice). Expects 403,
`{"error": "not_authorized"}`.

**Note on route shape:** The `/transfers` controller maps `from = self`, so a
caller cannot specify a `from` that differs from their session user. If the route
enforces this at the binding layer, the wrong-owner case produces 403 because
WalletOwner always compares `session.user` to the source wallet's owner. This
scenario sends `from: {{alice_user_id}}` in the body while authenticated as Bob,
expecting the backend to either ignore the field (using `self` from the session)
and then reject on WalletOwner, or reject the body field directly. Either way the
expected status is 403.

#### 25. transfer-replay — idempotency + no double-debit

This is the canonical replay readback test.

1. `GET /wallets/me` as Alice — capture `alice_balance_before`.
2. `GET /wallets/me` as Bob — capture `bob_balance_before` (authenticated as Bob).
3. `POST /transfers` as Alice with `{{transfer_alice_to_bob_key}}` (same key as
   scenario 20). This is the replay. Expects 201; response body must have
   `out.id == {{transfer_out_id}}` and `in.id == {{transfer_in_id}}` — the
   identical journal entries from scenario 20, not new ones.
4. `GET /wallets/me` as Alice — assert `balance == alice_balance_before`
   (unchanged, not decremented again).
5. `GET /wallets/me` as Bob — assert `balance == bob_balance_before`
   (unchanged, not incremented again).

This proves idempotency: same key → same observable state. The response-body
check and the balance readback together are the double-debit guard.

---

### Group 6 — Scheduled transfers

#### 26. schedule-charlie-to-dave — happy path
`POST /transfers/schedule` as Charlie with `to: {{dave_user_id}}`,
`{{schedule_amount}}`, and `fire_at = now + {{schedule_offset_seconds}}` (90s).
Expects 201, body has `schedule_id` (non-empty). Captures `schedule_id`.

**Time math:** Hurl 4.x has no built-in arithmetic on captured timestamps.
The `fire_at` value is injected by the test runner as the variable `fire_at_90s`.

**Runner contract:** Before running the hurl file, the test runner must compute:

```sh
fire_at_90s=$(date -u -d "+90 seconds" +"%Y-%m-%dT%H:%M:%SZ")
hurl --variable fire_at_90s="$fire_at_90s" ...
```

On macOS: `date -u -v+90S +"%Y-%m-%dT%H:%M:%SZ"`.

The hurl file uses `{{fire_at_90s}}` directly in the request body. If the runner
does not inject this variable the test fails at the variable-resolution step, not
at an HTTP assertion — making the omission visible and diagnosable.

#### 27. schedule-invalid-amount — InvalidAmount
`POST /transfers/schedule` as Charlie with `amount: {{invalid_amount}}` (`-1000`).
Expects 422, `{"error": "invalid_amount"}`.

#### 28. cancel-schedule — happy path
`POST /transfers/schedule/{{schedule_id}}/cancel` as Charlie. Expects 204.

**Note:** This cancels the schedule created in scenario 26, before it fires.
The "schedule doesn't fire if cancelled" sub-case of the schedule-fires group
(scenario 32) creates its own fresh schedule to cancel, so there is no ordering
conflict here.

#### 29. cancel-schedule-already-executed — AlreadyExecuted
To create a schedule in Executed state we need to wait for it to fire, which is
part of the time-based group (scenarios 30–32). Instead, this scenario uses a
different approach: create a schedule, wait for it to execute (scenarios 30–31),
then attempt to cancel it.

This scenario is therefore ordered after scenario 31 in the hurl file. See
`cancel-after-executed` in the hurl for the actual block placement.

#### 30. cancel-not-authorized — NotAuthorized
`POST /transfers/schedule/{{schedule_id}}/cancel` as Dave (not the schedule source).
The schedule was created by Charlie, so Dave is not the source owner.
Expects 403, `{"error": "not_authorized"}`.

**Note:** This is tested against the already-cancelled schedule from scenario 28.
The cancel endpoint must check authorization before state — a NotAuthorized should
fire even on a non-Pending schedule. If the backend checks state first, Dave would
get 409 (AlreadyExecuted/Cancelled) instead of 403. The hurl asserts 403.

**Judgment call:** The spec's CancelScheduledTransfer flow checks `sched.source !=
self → reject NotAuthorized` before calling `MarkCancelled()`, so authorization
is always evaluated first. The assert encodes this ordering.

---

### Group 7 — Schedule fires

These scenarios exercise the `schedule ExecuteScheduledTransfer every 1m`
declaration. The test requires real wall-clock time. Hurl 4.x supports
`[Options] delay: <ms>` to introduce per-entry waits, but there is no
cross-entry sleep primitive. The runner must use `--delay` at the CLI level or
a custom sleep entry.

**Runner contract for sleep:**
The hurl file uses a dummy `GET /wallets/me` entry with `[Options] delay: 70000`
and `delay: 30000` to approximate the two sleep phases. These are hurl 4.x
per-entry delays (milliseconds). The backend receives the request after the delay;
the request itself is valid and the response is used for balance readback.

If the runner disables delays (e.g. `--no-delay`), the schedule-fires scenarios
will fail the balance assertions — that is the expected failure mode for a runner
that cannot wait. Mark as `[~]` in COVERAGE.md if the CI runner strips delays.

#### 31. schedule-fires — setup
`POST /transfers/schedule` as Alice with `to: {{bob_user_id}}`,
`{{schedule_amount}}` (2500), `fire_at = {{fire_at_90s}}`.
Captures `fired_schedule_id`. Alice's balance must be >= 2500 at this point
(she has 35 000 after earlier scenarios).

#### 32. schedule-fires — verify pending at 70s
`GET /wallets/me` as Alice with `[Options] delay: 70000` (sleep 70s before
sending). At this point only 70s have elapsed since schedule creation. The
schedule fires at 90s; the schedule runner fires every 60s. The earliest possible
execution is at the 90s mark after a runner cycle. Verify:
- Alice's balance == pre-schedule balance (no debit yet).
- Bob's balance == pre-schedule balance (no credit yet).
- `GET /transfers/schedule` as Alice lists `fired_schedule_id` with
  `status == "Pending"`.

#### 33. schedule-fires — verify executed at 100s (total)
`GET /wallets/me` as Alice with `[Options] delay: 30000` (sleep 30s more;
total ~100s since schedule creation). By now fire_at has passed and at least one
60s runner cycle has elapsed after the fire_at threshold. Verify:
- Alice's balance == (pre-schedule balance − schedule_amount).
- Bob's balance == (pre-schedule balance + schedule_amount).
- `GET /transfers/schedule/:id` (or journal readback) shows
  `status == "Executed"`.

#### 34. cancel-before-fire — schedule doesn't fire if cancelled
`POST /transfers/schedule` as Bob with `to: {{alice_user_id}}`,
`{{schedule_amount}}`, `fire_at = {{fire_at_90s}}` (fresh computation by runner).
Capture `cancel_before_fire_id`. Cancel it immediately via
`POST /transfers/schedule/{{cancel_before_fire_id}}/cancel`.
Sleep past fire_at (another `delay: 70000` entry). Verify Bob's balance is
unchanged — the cancelled schedule was not executed.

**This is ordered last** because it requires another 70s wait. Runners with
`--no-delay` will skip balance verification here; the cancel itself (204) is still
asserted.

---

## Replay readback — canonical technique

The Transfer replay test (scenario 25) uses three layers of evidence:

1. **Response body equality:** `out.id` and `in.id` in the replay response match
   the captured ids from the first call. New journal entries would have new ids.
2. **Alice balance unchanged:** `GET /wallets/me` confirms Alice's balance is the
   same before and after the replay — not decremented twice.
3. **Bob balance unchanged:** Same for Bob — not incremented twice.

Layer 1 alone proves no new entries were written. Layers 2 and 3 are redundant
but add defense against a backend that returns old ids while still debiting.
All three layers are asserted in the hurl.

---

## Time math — fire_at handling

Hurl 4.x has no date arithmetic. `fire_at_90s` must be injected by the runner.
The canonical runner snippet (bash):

```sh
fire_at_90s=$(date -u -d "+90 seconds" +"%Y-%m-%dT%H:%M:%SZ")
hurl \
  --variables-file evals/wallet/fixtures.env \
  --variable BASE_URL=http://localhost:8080 \
  --variable fire_at_90s="$fire_at_90s" \
  evals/wallet/wallet.hurl
```

The 90s offset matches `schedule_offset_seconds=90` in fixtures.env. The offset
is set deliberately short (90s >> 60s schedule cadence) so the schedule fires
within two runner cycles of its fire_at.

---

## Admin role bootstrap

The spec creates all users with role User. The `/admin/wallets/:owner/fund` and
`/admin/users/:id/promote` routes require role Admin. There is no "first admin"
endpoint in the spec.

The wallet hurl assumes the backend is started with a seeded Admin whose
credentials match `{{admin_email}}` / `{{admin_password}}`. The login call in
the setup group must return a token with `role: "Admin"`. If it returns
`role: "User"`, the admin-gated scenarios will all return 403 and the test will
fail immediately — the failure is easy to diagnose.

---

## Coverage map

| COVERAGE.md row                                                     | Scenario                        |
|---------------------------------------------------------------------|---------------------------------|
| `POST /signup` ok → 201                                             | signup-admin/alice/bob/charlie/dave |
| `POST /login` ok → 200                                              | login-admin/alice/bob/charlie/dave |
| `POST /logout` ok → 204                                             | logout-alice                    |
| `POST /admin/wallets/:owner/fund` ok → 201                          | fund-alice, fund-bob            |
| err InvalidAmount → 422                                             | fund-invalid-amount             |
| err WalletNotFound → 404                                            | fund-wallet-not-found           |
| wrong role (User) → 403                                             | fund-wrong-role                 |
| `GET /wallets/me` ok → 200                                          | get-balance-alice               |
| `POST /wallets/me/withdraw` ok → 201                                | withdraw-alice                  |
| err InsufficientFunds → 409                                         | withdraw-insufficient-funds     |
| err InvalidAmount → 422                                             | withdraw-invalid-amount         |
| wrong owner → 403                                                   | [~] route-enforced (see note)   |
| `POST /transfers` ok → 201                                          | transfer-alice-to-bob           |
| err InsufficientFunds → 409                                         | transfer-insufficient-funds     |
| err SelfTransfer → 422                                              | transfer-self                   |
| err WalletNotFound → 404                                            | transfer-wallet-not-found       |
| replay → same response, no double-debit                             | transfer-replay                 |
| `POST /transfers/schedule` ok → 201                                 | schedule-charlie-to-dave        |
| err InvalidAmount → 422                                             | schedule-invalid-amount         |
| `POST /transfers/schedule/:id/cancel` ok → 204                      | cancel-schedule                 |
| err AlreadyExecuted → 409                                           | cancel-after-executed           |
| err NotAuthorized → 403                                             | cancel-not-authorized           |
| `schedule ExecuteScheduledTransfer` fires at fire_at                | schedule-fires (scenarios 31–33)|
| doesn't fire if cancelled                                           | cancel-before-fire (scenario 34)|

---

## Deferred items

None. All COVERAGE.md rows for this feature are directly testable via HTTP.
The two runner-contract dependencies (admin bootstrap seed, `fire_at_90s`
injection) are documented above and are runner configuration, not missing
test coverage.

---

## Judgment calls

**Admin bootstrap:** The spec has no "first admin" endpoint. The hurl assumes a
seeded admin at startup. This is the only practical option without an out-of-band
fixture injection API. The failure mode if the seed is missing is immediate and
obvious (all admin-gated calls return 403).

**WalletOwner on Withdraw:** The route binds `wallet = self`, so a different
`wallet_owner` id cannot be injected via the HTTP interface. Coverage is marked
`[~]`. A backend exposing a `/wallets/:id/withdraw` shape would need a direct
test; the current spec does not.

**Wrong-owner on Transfer (scenario 24):** The route maps `from = self`. If the
backend ignores any `from` field in the body and always uses the session user,
the test still achieves coverage because Bob calling the transfer endpoint will be
using his own wallet as source, transferring to Dave — which is a valid transfer,
not a 403. The "wrong owner" case must be crafted differently: Bob calls the
endpoint with Alice's `user_id` as an explicit `from` field. If the backend
accepts `from` in the body and checks WalletOwner against it, it will return 403.
If the backend ignores `from` in the body (always uses `self`), the test will
return 201 and the assertion will fail — which is the correct failure for a backend
that doesn't enforce this. The hurl asserts 403.

**fire_at_90s injection:** Hurl has no date math. Documenting the runner contract
in the `.md` is the right approach; adding a `# RUNNER_REQUIRES: fire_at_90s`
comment at the top of the `.hurl` makes the dependency machine-readable.

**Schedule sleep phases:** Per-entry `delay:` in hurl 4.x applies before the
request is sent, not after. The delay value is on the balance-readback GET, so
the 70s sleep happens before the balance check — correct behavior. The total
wall-clock cost of the schedule-fires group is ~100s (70s + 30s); runners with
a hard test timeout shorter than 120s should increase their limit for this file.

**cancel-not-authorized ordering:** The hurl tests NotAuthorized against the
already-cancelled schedule (scenario 28's target). Per the spec, authorization is
checked before state (`sched.source != self → reject NotAuthorized` precedes
`MarkCancelled()`). If a backend checks state first, Dave gets 409 instead of 403
and the assert fails — this is intentional, surfacing a spec-ordering violation.
