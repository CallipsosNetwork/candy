# airbnb/auth — Scenario narrative

Feature: `auth.candy` — User, Session, signup/login/logout, role management.
Hurl: `evals/airbnb/auth.hurl`
Issue: #10 — "FULL incl. role verification"

Roles rank `Guest < Host < Admin`. Guests sign up by default; a Guest
self-promotes to Host via `/me/upgrade-to-host`; an Admin promotes any
user to any role (with guard: Admin → Guest demotion is rejected) via
`/admin/users/:id/promote`. Admin-scope routes also include
`/admin/users/:id/verify` to mark an account as email-verified.

---

## Setup

The `.hurl` bootstraps all actors inline — no external seed data is
required beyond `fixtures.env`. Execution order:

1. Sign up `guest@candy.local` (Guest, default role).
2. Sign up `host@candy.local` (Guest initially; self-upgrades to Host
   later in the script).
3. Bootstrap admin — see "Deferred: admin bootstrap" below.

All tokens and user ids captured via `[Captures]` from prior responses.

---

## Deferred: admin bootstrap

**Problem.** The first Admin must exist before any
`/admin/users/:id/promote` call can succeed. Admin is not a self-service
role — only an existing Admin can promote another user to Admin. This is
the same chicken-and-egg problem as the todo eval.

**Resolution in this file.** The `.hurl` uses a placeholder variable
`{{first_admin_token}}` that the harness must inject before running. The
narrative below describes the expectation:

> The test harness seeds one admin user out-of-band (direct DB insert or
> a privileged `/internal/seed-admin` endpoint that the backend exposes
> only in test mode) and supplies its bearer token via
> `--variable first_admin_token=<value>`. This is a harness concern, not
> a spec concern; the candy spec itself is correct — the first admin is a
> deployment-time actor, not a signup-time one.

Until that harness endpoint ships, the admin-dependent scenarios are
**documented here and present in the `.hurl`** but will fail against a
blank-state backend with no OOB admin injection. Mark them `[d]` in
`COVERAGE.md` only if the harness cannot be wired; the current plan is
that they run once the seed mechanism exists.

---

## Scenarios

### 1 — Signup: happy path (Guest)

Sign up `guest@candy.local` with a valid password and idempotency key
`{{signup_guest_key}}`. Expect 201 with `user_id` and `token`. Capture
both. Confirm the response body contains no plaintext password.

### 2 — Signup: WeakPassword — TooShort

Attempt signup with password `"short1"` (< 12 chars). Expect 422,
`error: "weak_password"`, `reason: "too_short"` (or equivalent variant
string — assert shape, not exact string value).

### 3 — Signup: WeakPassword — MissingDigit

Attempt signup with password `"alllowercaseonly"` (no digit). Expect
422, `error: "weak_password"`, `reason: "missing_digit"`.

### 4 — Signup: WeakPassword — InBlocklist

Attempt signup with password `"password123"` (blocklisted). Expect 422,
`error: "weak_password"`, `reason: "in_blocklist"`.

### 5 — Signup: EmailTaken

Re-attempt signup with `guest@candy.local` (already registered).
Expect 409, `error: "email_taken"`.

### 6 — Signup: idempotency replay

Replay the exact same signup request for `guest@candy.local` with
`idempotency_key: {{signup_guest_key}}`. The spec declares Signup
idempotent on key. Expect 201 again with the **same** `user_id` and
`token` as the first signup (no new record created, no new event
emitted). Assert `user_id` equals the captured value from scenario 1.

### 7 — Signup: host account (bootstrap)

Sign up `host@candy.local` with `{{signup_host_key}}`. Capture
`host_user_id` and `host_token`. Used in later scenarios.

### 8 — Login: happy path

Login with `guest@candy.local` / correct password. Expect 200 with
`user_id`, `role: "guest"`, and `token`. Capture `guest_login_token`.
Assert `role == "guest"` (confirming default role propagates to session).

### 9 — Login: wrong password

Login with `guest@candy.local` and an incorrect password. Expect 401,
`error: "invalid_credentials"`. The error must not reveal whether the
email or the password was wrong.

### 10 — Login: unknown email

Login with `nobody@candy.local` (not registered). Expect 401,
`error: "invalid_credentials"`. Same opaque shape as scenario 9.

### 11 — Logout: happy path

Logout using the token captured in scenario 8 (`guest_login_token`).
Expect 204 (no body). Verify the session is revoked by attempting a
second authenticated call with the same token and confirming 401.

### 12 — Logout: missing bearer

POST `/logout` with no `Authorization` header. Expect 401.

### 13 — Logout: invalid bearer

POST `/logout` with `Authorization: Bearer not-a-real-token`. Expect 401.

### 14 — Logout: idempotency (already revoked)

Replay the logout from scenario 11 — same token, same idempotency key.
The spec says `Session.Revoke()` is idempotent (re-revoking is a no-op).
Expect 204 again (not 401), confirming the idempotent path resolves
before the bearer check on revoked sessions.

> **Note.** If the backend validates the bearer token before resolving
> idempotency, this scenario becomes a 401. The candy spec's intent
> ("re-revoking is a no-op") implies the idempotent path wins, but the
> harness must enforce this ordering. Document as a known ambiguity if a
> target backend disagrees.

### 15 — Self-upgrade to Host (Guest → Host)

Using `host_token` from scenario 7, POST `/me/upgrade-to-host`. Expect
200, `role: "host"`. Confirm by logging in again as `host@candy.local`
and asserting the new login response carries `role: "host"`.

### 16 — Self-upgrade: AlreadyHost

Replay `/me/upgrade-to-host` with the same `host_token` (now a Host).
Expect 409, `error: "already_host"`.

### 17 — Self-upgrade: wrong role (Admin tries)

Using `{{first_admin_token}}`, POST `/me/upgrade-to-host`. The route is
`RoleGated(Guest)` — only callers with exactly Guest role may use it.
Admin rank is above Guest, so the policy rejects with 403.

> **Judgment call.** `RoleGated(Guest)` means "minimum role = Guest",
> which under the ranking `Guest < Host < Admin` would normally let any
> role through. But the candy spec's intent for `/me/upgrade-to-host` is
> clearly "Guest-only self-promotion" — an Admin self-downgrading via
> this route makes no sense and the COVERAGE.md explicitly lists "wrong
> role (Admin tries) → 403". We treat `RoleGated` here as an exact-match
> guard for Guest, not a minimum-rank guard. If the codegen treats it as
> minimum-rank, the route needs a separate `OnlyGuest` policy — flag this
> as a codegen decision point.

### 18 — Admin promote: Guest → Host (happy path)

Using `{{first_admin_token}}`, POST `/admin/users/{{guest_user_id}}/promote`
with body `{ role: "host" }`. Expect 200, `promoted: true`. Confirm by
logging in as guest and asserting `role: "host"` in the response.

### 19 — Admin promote: InvalidPromotion (Admin → Guest demotion)

Sign up a second guest (`admin2@candy.local`, captured as
`admin2_user_id`). Promote them to Admin first (using
`{{first_admin_token}}`). Then attempt to promote `admin2_user_id` to
`Guest` (a demotion). The spec guard is:

```
require: not (role == Admin and to == Guest)  rescue reject InvalidPromotion
```

Expect 422, `error: "invalid_promotion"`.

### 20 — Admin promote: wrong role (non-Admin tries)

Using `guest_token` (a Guest), attempt
`POST /admin/users/{{host_user_id}}/promote`. Expect 403 (insufficient
role for Admin-gated route).

### 21 — Admin verify: happy path

Using `{{first_admin_token}}`, POST
`/admin/users/{{guest_user_id}}/verify` with a fresh idempotency key.
Expect 200, `verified: true`.

### 22 — Admin verify: AlreadyVerified

Replay the verify from scenario 21 with a **different** idempotency key
(i.e., not a replay — a genuine second attempt after the user is already
verified). Expect 409, `error: "already_verified"`.

### 23 — Admin verify: UserNotFound

Attempt to verify a non-existent user id (`00000000-0000-0000-0000-000000000000`
or similar sentinel). Expect 404, `error: "user_not_found"`.

### 24 — Admin verify: wrong role (non-Admin tries)

Using `host_token`, attempt `/admin/users/{{guest_user_id}}/verify`.
Expect 403.

### 25 — Admin verify: idempotency replay

Replay the exact verify from scenario 21 (same idempotency key). Expect
200, `verified: true` again — same response, no duplicate state change.

---

## Coverage summary

| Scenario | Endpoint                        | Variant                              |
|----------|---------------------------------|--------------------------------------|
| 1        | POST /signup                    | ok → 201                             |
| 2        | POST /signup                    | err WeakPassword TooShort → 422      |
| 3        | POST /signup                    | err WeakPassword MissingDigit → 422  |
| 4        | POST /signup                    | err WeakPassword InBlocklist → 422   |
| 5        | POST /signup                    | err EmailTaken → 409                 |
| 6        | POST /signup                    | idempotency replay → 201 (same ids)  |
| 7        | POST /signup                    | host bootstrap                       |
| 8        | POST /login                     | ok → 200 (role in response)          |
| 9        | POST /login                     | err InvalidCredentials (wrong pw)    |
| 10       | POST /login                     | err InvalidCredentials (no user)     |
| 11       | POST /logout                    | ok → 204                             |
| 12       | POST /logout                    | missing bearer → 401                 |
| 13       | POST /logout                    | invalid bearer → 401                 |
| 14       | POST /logout                    | idempotent replay → 204              |
| 15       | POST /me/upgrade-to-host        | ok (Guest) → 200                     |
| 16       | POST /me/upgrade-to-host        | err AlreadyHost → 409                |
| 17       | POST /me/upgrade-to-host        | wrong role (Admin) → 403             |
| 18       | POST /admin/users/:id/promote   | ok (Admin promotes) → 200            |
| 19       | POST /admin/users/:id/promote   | err InvalidPromotion → 422           |
| 20       | POST /admin/users/:id/promote   | wrong role (Guest) → 403             |
| 21       | POST /admin/users/:id/verify    | ok → 200                             |
| 22       | POST /admin/users/:id/verify    | err AlreadyVerified → 409            |
| 23       | POST /admin/users/:id/verify    | err UserNotFound → 404               |
| 24       | POST /admin/users/:id/verify    | wrong role (Host) → 403              |
| 25       | POST /admin/users/:id/verify    | idempotency replay → 200             |

---

## Deferred items

- **Admin bootstrap**: first admin must be injected OOB by the harness
  (`--variable first_admin_token=<value>`). Scenarios 17–25 depend on
  this. The `.hurl` file is written and ready; execution requires the
  harness seed mechanism (direct DB insert or test-mode `/internal/seed-admin`
  endpoint).

- **Session readback after logout**: scenario 11 verifies revocation by
  retrying with the revoked token, which is a valid black-box check. A
  deeper "list active sessions" readback would require a sessions endpoint
  not present in this spec.

- **Logout idempotency ambiguity** (scenario 14): if a target backend
  validates the bearer before resolving idempotency, the replay of a
  revoked-token logout returns 401 instead of 204. This is a codegen
  ordering decision; flagged for discussion, not blocked.
