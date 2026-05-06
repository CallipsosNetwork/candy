# Auth — scenario narrative

**Feature:** `auth` (standalone)
**Spec under test:** `examples/auth/auth.candy`
**Hurl file:** `evals/auth/auth.hurl`
**Total scenarios:** 13

---

## Setup

Two users are bootstrapped in order: Alice is signed up first (the
idempotency-replay scenario re-sends her exact signup key to confirm
idempotency), then Bob is signed up to enable the EmailTaken test.
Alice is also logged in during setup to produce the bearer token used
across all logout scenarios. No teardown — the backend starts empty.

---

## Scenarios

### 1. signup-alice — happy path

Sign up Alice with a strong password and a fixed idempotency key.

- **Fixtures:** `{{alice_email}}`, `{{alice_password}}`, `{{signup_alice_key}}`
- **Expected:** 201; body contains `user_id` (non-empty) and `token` (non-empty).
  Both values are captured for use in later scenarios.

---

### 2. signup-weak-too-short — WeakPassword / TooShort

Attempt signup with `{{weak_short}}` (`"short1"`, 6 chars).

- **Fixtures:** `{{alice_email}}` (email doesn't matter for this path), `{{weak_short}}`
- **Expected:** 422; `{"error": "weak_password", "reason": "too_short"}` (or the
  codegen-canonical casing of `TooShort`).

---

### 3. signup-weak-missing-digit — WeakPassword / MissingDigit

Attempt signup with `{{weak_no_digit}}` (`"alllowercaseletters"`).

- **Fixtures:** `{{weak_no_digit}}`
- **Expected:** 422; `{"error": "weak_password", "reason": "missing_digit"}`.

---

### 4. signup-weak-blocklisted — WeakPassword / InBlocklist

Attempt signup with `{{weak_blocklisted}}` (`"password123"`).

- **Fixtures:** `{{weak_blocklisted}}`
- **Expected:** 422; `{"error": "weak_password", "reason": "in_blocklist"}`.

---

### 5. signup-bob — second user bootstrap

Sign up Bob. Required so that scenario 6 (EmailTaken) can attempt to re-register
Alice's already-used email.

- **Fixtures:** `{{bob_email}}`, `{{bob_password}}`, `{{signup_bob_key}}`
- **Expected:** 201; `user_id` and `token` present. Bob's user id is captured.

---

### 6. signup-email-taken — EmailTaken

Attempt signup with Alice's email (`{{alice_email}}`), which is already registered.

- **Fixtures:** `{{alice_email}}`, `{{bob_password}}` (any strong password)
- **Expected:** 409; `{"error": "email_taken"}`.

---

### 7. signup-idempotency-replay — idempotency replay

Replay Alice's exact signup request (same email, password, and idempotency key
`{{signup_alice_key}}`).

- **Fixtures:** all of Alice's signup fixtures
- **Expected:** 201; `user_id` equals the `alice_user_id` captured in scenario 1;
  `token` may differ (spec says "fresh session") but the user identity must be
  stable. The `user_id` assert confirms no duplicate record was created.

---

### 8. login-alice — happy path

Log in Alice with correct credentials.

- **Fixtures:** `{{alice_email}}`, `{{alice_password}}`
- **Expected:** 200; `user_id` matches `{{alice_user_id}}`; `token` is non-empty.
  Captures `alice_token` for logout scenarios.

---

### 9. login-wrong-password — InvalidCredentials (wrong password)

Attempt login with Alice's email but an incorrect password.

- **Fixtures:** `{{alice_email}}`; wrong password inline (`"wrong-password-99"`)
- **Expected:** 401; `{"error": "invalid_credentials"}`. The body must not
  distinguish password error from email error — same opaque shape either way.

---

### 10. login-no-such-email — InvalidCredentials (no such user)

Attempt login with an email that was never registered.

- **Fixtures:** email inline (`"nobody@candy.local"`), any strong password
- **Expected:** 401; `{"error": "invalid_credentials"}`. Same opaque shape as
  scenario 9 — this is the conformance assertion that the implementation
  does not leak which field failed.

---

### 11. logout-alice — happy path

Revoke Alice's session using the token captured in scenario 8.

- **Fixtures:** `{{alice_token}}` (from capture)
- **Expected:** 204 with empty body.

---

### 12. logout-replay — idempotent re-revoke

Send the same logout request again (same bearer token that was already revoked).
`Session.Revoke()` is declared idempotent in the spec.

- **Fixtures:** `{{alice_token}}` (same token as scenario 11)
- **Expected:** 204. No error — re-revoking must be a no-op.

---

### 13. logout-missing-bearer — 401 on missing Authorization header

Send `POST /logout` with no `Authorization` header.

- **Expected:** 401.

---

### 14. logout-invalid-bearer — 401 on bogus token

Send `POST /logout` with `Authorization: Bearer bogus-token-xyz`.

- **Expected:** 401.

---

## Coverage map

| COVERAGE.md row                                      | Scenario                       |
|------------------------------------------------------|--------------------------------|
| `POST /signup` ok → 201                              | signup-alice                   |
| err WeakPassword (TooShort) → 422                    | signup-weak-too-short          |
| err WeakPassword (MissingDigit) → 422                | signup-weak-missing-digit      |
| err WeakPassword (InBlocklist) → 422                 | signup-weak-blocklisted        |
| err EmailTaken → 409                                 | signup-email-taken             |
| idempotency replay (same key)                        | signup-idempotency-replay      |
| `POST /login` ok → 200                               | login-alice                    |
| err InvalidCredentials (wrong pw) → 401              | login-wrong-password           |
| err InvalidCredentials (no user) → 401               | login-no-such-email            |
| `POST /logout` ok → 204                              | logout-alice                   |
| replay (already revoked) → 204 (idempotent)          | logout-replay                  |
| missing bearer → 401                                 | logout-missing-bearer          |
| invalid bearer → 401                                 | logout-invalid-bearer          |

---

## Judgment calls

**Opaque InvalidCredentials parity (scenarios 9 and 10):** Both wrong-password and
no-such-email must return 401 with `{"error": "invalid_credentials"}`. The hurl
asserts are identical for both; the two scenarios together form the conformance
check that no extra field (e.g. `"field": "email"`) leaks into the error body.

**WeakPassword reason casing:** The spec declares `TooShort`, `MissingDigit`,
`InBlocklist` as variant names. Codegen will snake_case these to `too_short`,
`missing_digit`, `in_blocklist` in the JSON body. The hurl asserts use the
snake_case form; if a target emits PascalCase the assert will fail and that
is intentional — it surfaces a codegen inconsistency.

**Logout idempotency (scenario 12):** The spec says `Revoke()` is "idempotent —
re-revoking is a no-op" and the controller maps `ok(_) -> 204`. A 401 on replay
would mean the backend is re-validating the session token after revocation,
which contradicts the spec. The assert enforces 204.

**No deferred scenarios in this file.** All 13 coverage rows are directly
testable via HTTP without external stubs or webhook injection.
