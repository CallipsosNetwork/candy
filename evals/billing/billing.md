# Billing — scenario narrative

**Feature:** `billing`
**Spec under test:** `examples/billing/billing.candy`
**Hurl file:** `evals/billing/billing.hurl`
**Total scenarios:** 20 (12 runnable, 8 deferred `[d]`)

---

## Setup

Two users are bootstrapped: Admin (role Admin, provisioned out-of-band or via a
direct signup that a seed migration promotes) and Customer. Because codegen
provisions the first-signup Admin role automatically, the Admin signup is the
first request in the file. Both tokens are captured and reused throughout.

After auth setup, Admin creates the 60s test plan. Its id is captured as
`plan_id` and used in all subscription scenarios. The 60s interval and 60s
retry delay make every billing schedule observable in a single test run without
any production-plan changes.

---

## Scenarios

### 1. signup-admin — bootstrap Admin

Sign up the admin account with a fixed idempotency key.

- **Fixtures:** `{{admin_email}}`, `{{admin_password}}`, `{{signup_admin_key}}`
- **Expected:** 201; `user_id` and `token` captured as `admin_user_id` and
  `admin_signup_token`.

---

### 2. signup-customer — bootstrap Customer

Sign up the customer account.

- **Fixtures:** `{{customer_email}}`, `{{customer_password}}`, `{{signup_customer_key}}`
- **Expected:** 201; `user_id` and `token` captured as `customer_user_id` and
  `customer_token`.

---

### 3. login-admin — obtain admin bearer

Log in as admin to obtain the bearer token used for all admin-gated routes.

- **Fixtures:** `{{admin_email}}`, `{{admin_password}}`
- **Expected:** 200; `token` captured as `admin_token`.

---

### 4. logout-customer — minimal auth coverage

Log out the customer token captured at signup. Confirms 204 and that
`POST /logout` requires a bearer (covered implicitly by the happy path here;
the missing/invalid bearer cases are covered exhaustively in the standalone
auth eval and are not repeated here).

- **Expected:** 204.

---

## Plan management

### 5. create-plan — Admin creates 60s test plan

Admin creates a plan with `interval = 60`, `retry_delay = 60`,
`escalation_window = 300`.

- **Fixtures:** `{{plan_name}}`, `{{plan_amount}}` (999), `{{plan_interval_seconds}}`
  (60), `{{plan_retry_delay_seconds}}` (60), `{{plan_escalation_window_seconds}}`
  (300), `{{create_plan_key}}`
- **Authorization:** `Bearer {{admin_token}}`
- **Expected:** 201; body contains `plan_id`; captured as `plan_id`.

---

### 6. create-plan-wrong-role — Customer cannot create plans (403)

Customer attempts `POST /admin/plans`. Policy `AdminGated` must reject.

- **Expected:** 403.

---

### 7. delete-plan — Admin deletes a throwaway plan

Admin creates a second plan (`delete-plan-key`), captures its id, then deletes
it. Verifying the delete path without disturbing the primary `{{plan_id}}`
used by subscription scenarios.

- **Expected:** Create returns 201; delete returns 204.

---

## Subscription lifecycle

### 8. subscribe — Customer subscribes to the 60s plan

Customer posts `POST /subscriptions` with `plan = {{plan_id}}`, `source =
{{test_payment_method}}`, and `{{subscribe_key}}`.

- **Expected:** 201; `subscription_id` captured as `subscription_id`. The
  subscription starts Active; `next_charge_date` is set to `now` so the first
  ChargeCycle tick picks it up immediately.

---

### 9. subscribe-invalid-plan — InvalidPlan (422)

Customer attempts to subscribe to a non-existent plan id.

- **Expected:** 422; `{"error": "invalid_plan"}`.

---

### 10. cancel-subscription — Customer cancels their subscription (200)

Customer posts `POST /subscriptions/{{subscription_id}}/cancel`.

- **Note:** The spec maps `ok(_) -> 204` for CancelSubscription. The cancel
  marks status Cancelled; service continues to `next_charge_date` per spec
  intent but the HTTP response is empty.
- **Expected:** 204.

---

### 11. cancel-subscription-already-cancelled — AlreadyCancelled (409)

Replay the cancel request against the same subscription.

- **Expected:** 409; `{"error": "already_cancelled"}`.

---

### 12. reactivate-not-suspended — NotSuspended (409)

Attempt to reactivate the Cancelled subscription (which is not Suspended).
`Restore()` rejects `NotSuspended` for any status except Suspended.

- **Expected:** 409; `{"error": "not_suspended"}`.

---

### 13. reactivate-suspended — `[d]` Reactivate from Suspended (200)

**Deferred.** Requires a subscription in Suspended state, which can only be
reached after a charge failure path runs through `EscalateOverdue`. That path
requires Polar sandbox forced-decline cards, which are not available in the
current test harness.

**When unblocked:** Subscribe a second customer, force three consecutive charge
failures (or allow `escalation_window` to expire), verify `status == Suspended`,
then call `POST /subscriptions/:id/reactivate` with a fresh payment method.
Assert 200 and that a subsequent readback shows `status == Active`.

---

## Schedule cadence

### 14. schedule-charge-cycle-cadence — `[~]` ChargeCycle fires within 60s of next_charge_date

Subscribe a fresh customer (second subscription) so that `next_charge_date` is
at or before `now`. Sleep `{{schedule_settle_window_seconds}}` (70s) to allow
one full schedule tick plus a small buffer. Read back the subscription.

The assertion is permissive: status must be `Active` or `PastDue`. This proves
ChargeCycle ran — it either succeeded (Active) or attempted and failed (PastDue)
depending on Polar sandbox behavior. The test does **not** pin to one outcome
because the sandbox response for `{{test_payment_method}}` is
environment-specific.

- **Sleep:** 70 seconds (runner-injected; see `schedule_settle_window_seconds`).
- **Asserts (post-sleep readback):**
  - `status` is one of `["Active", "PastDue"]`
  - Either `last_charge` exists (success path) or `last_failed_at` exists
    (failure path) — confirms the schedule did meaningful work, not just a
    no-op skip.

---

### 15. schedule-charge-cycle-success — `[d]` Charge succeeds → status Active, next_charge_date advanced

**Deferred.** Requires Polar test-mode cards that guarantee a successful charge.
Polar's sandbox supports this, but the test card vocabulary (token strings that
force success vs. decline) varies by SDK version and is not yet stabilized in
the test runner environment.

**When unblocked:** Use the Polar test-mode success token as `source`. After the
settle window, assert `status == Active`, `last_charge` is a non-empty string,
`attempts == 0`, and `next_charge_date` equals the original `next_charge_date`
plus `plan.interval` (60s).

---

### 16. schedule-charge-cycle-failure — `[d]` Charge fails → status PastDue

**Deferred.** Requires a Polar test-mode decline token.

**When unblocked:** Use the forced-decline token as `source`. After the settle
window, assert `status == PastDue`, `attempts == 1`, `last_failed_at` is set,
`first_failed_at` is set, and `last_charge` is absent.

---

### 17. schedule-retry-due — `[d]` RetryDue fires after retry_delay

**Deferred.** Requires a subscription already in PastDue state (from scenario
16), then sleeping `{{retry_settle_window_seconds}}` (130s) = 60s retry delay
+ one 1m schedule cadence.

**When unblocked:** After scenario 16 puts the subscription in PastDue, sleep
130s. Read back. If the retry succeeded (forced-success card), assert
`status == Active`, `attempts == 0`. If the retry also failed (forced-decline
card throughout), assert `status == PastDue`, `attempts == 2`.

The test proves RetryDue ran — `updated` timestamp must advance past the
post-scenario-16 checkpoint regardless of outcome.

---

### 18. schedule-escalate-overdue — `[d]` EscalateOverdue suspends after 3 failures

**Deferred.** Requires three consecutive failures in the PastDue window, or
`first_failed_at + escalation_window <= now` (300s). With forced-decline cards
and `retry_delay = 60`, this is observable in under 5 minutes (3 × 130s =
~390s total from first failure, which triggers the attempt-count gate at
`attempts >= 3` before the 300s escalation window, depending on timing).

**When unblocked:** After three forced-decline RetryDue ticks (`attempts == 3`),
sleep `{{escalation_settle_window_seconds}}` (400s) from `first_failed_at`. Read
back. Assert `status == Suspended`, `SubscriptionSuspended` event emitted once.

---

## Polar test mode and charge-result deferral

Polar offers a sandbox / test mode. The sandbox does respond to charge requests
and does emit `ChargeSucceeded` / `ChargeFailed` events. However, the specific
token strings that force a success vs. a decline vary between Polar SDK versions
and are not yet part of the candy test runner contract.

The schedule-cadence test (scenario 14) side-steps this by asserting that the
schedule ran at all rather than what it decided. The charged result is treated
as environment-opaque: either Active (sandbox allowed the card) or PastDue
(sandbox declined it). Both outcomes confirm ChargeCycle fired.

Everything below scenario 14 — success/failure pinning, retry cadence, and
escalation — is `[d]` until the test runner provides:

1. A canonical Polar sandbox test-card vocabulary (force-success token and
   force-decline token) accessible without real API credentials.
2. A mock-card injection mechanism in the hurl runner so `{{test_payment_method}}`
   can be overridden per scenario.

These are the same deferred-harness categories described in `evals/README.md`
under "Failure-injection compensation tests".

---

## Coverage map

| COVERAGE.md row                                         | Scenario                         |
|---------------------------------------------------------|----------------------------------|
| `POST /signup` / `/login` / `/logout` (auth coverage)  | signup-admin, signup-customer, login-admin, logout-customer |
| `POST /admin/plans` ok → 201                            | create-plan                      |
| wrong role → 403                                        | create-plan-wrong-role           |
| `DELETE /admin/plans/:id` ok → 204                      | delete-plan                      |
| `POST /subscriptions` ok → 201                          | subscribe                        |
| err InvalidPlan → 422                                   | subscribe-invalid-plan           |
| `POST /subscriptions/:id/cancel` ok → 204               | cancel-subscription              |
| err AlreadyCancelled → 409                              | cancel-subscription-already-cancelled |
| `POST /subscriptions/:id/reactivate` ok → 200           | reactivate-suspended `[d]`       |
| err NotSuspended → 409                                  | reactivate-not-suspended         |
| `schedule ChargeCycle` fires within 60s                 | schedule-charge-cycle-cadence `[~]` |
| charge succeeds → status Active                         | schedule-charge-cycle-success `[d]` |
| charge fails → status PastDue                           | schedule-charge-cycle-failure `[d]` |
| `schedule RetryDue` retries after 60s                   | schedule-retry-due `[d]`         |
| `schedule EscalateOverdue` suspends after attempts ≥ 3  | schedule-escalate-overdue `[d]`  |

---

## Judgment calls

**CancelSubscription status code:** The spec maps `ok(_) -> 204`. COVERAGE.md
lists the row as "ok → 200". This narrative follows the spec (204); the hurl
asserts HTTP 204. If a target returns 200 that is a codegen bug and the assert
surfaces it.

**schedule-cadence permissive assert:** Pinning `status == Active` would make the
test fragile against sandbox environments where the test card is declined by
default. The `in ["Active", "PastDue"]` assert proves schedule firing without
requiring control over Polar sandbox card behavior. This is intentional and
explicitly documented above as the boundary between runnable and deferred tests.

**Reactivate from Cancelled vs. Suspended:** Scenario 12 (reactivate-not-suspended)
intentionally reactivates the Cancelled subscription, not a non-existent one.
`NotSuspended` is the correct error for any status that is not Suspended —
Cancelled included. This tests the precondition, not the not-found path.

**delete-plan uses a separate plan:** Deleting `{{plan_id}}` mid-file would break
all downstream subscription scenarios. A dedicated throwaway plan is created and
deleted in scenario 7. This keeps the primary plan alive for the full file.

**No idempotency replay tests for CreatePlan / Subscribe in this file:** Both
flows accept `key: Key`. Idempotency replay coverage is exercised exhaustively
in the auth eval (Signup) and wallet eval (Transfer) as the canonical pattern.
Billing does not repeat that coverage to keep the file focused on billing
behavior. If a billing-specific idempotency concern arises (e.g., double-charge
on schedule re-fire), it belongs in the `BillingFrequency` policy section of
the spec, not in this eval.
