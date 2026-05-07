# Coverage checklist

Per-feature, per-endpoint, per-response-variant matrix. A row is checked
when the corresponding scenario lands in the feature's `.hurl` file (and
the `.md` documents any deferred parts).

## Notation

- `[ ]` — not yet covered
- `[x]` — covered (scenario in `.hurl`); add `(✓ <target>)` to mark where it has run green against a generated backend
- `[d]` — documented in `.md`, deferred until harness exists (mock SDK, scheduled-flow runner, etc.)
- `[~]` — partially covered (e.g. happy path runs but a sub-variant is deferred)

---

## auth (`evals/auth/auth.hurl`) — 14 scenarios; ✓ Go target green (PR #45)

| Endpoint                            | Variant                          | Status |
|-------------------------------------|----------------------------------|--------|
| `POST /signup`                      | ok → 201                         | `[x]` (✓ Go) |
|                                     | err WeakPassword (TooShort) → 422 | `[x]` (✓ Go) |
|                                     | err WeakPassword (MissingDigit) → 422 | `[x]` (✓ Go) |
|                                     | err WeakPassword (InBlocklist) → 422 | `[x]` (✓ Go) |
|                                     | err EmailTaken → 409             | `[x]` (✓ Go) |
|                                     | idempotency replay (same key)    | `[x]` (✓ Go) |
| `POST /login`                       | ok → 200                         | `[x]` (✓ Go) |
|                                     | err InvalidCredentials (wrong pw) → 401 | `[x]` (✓ Go) |
|                                     | err InvalidCredentials (no user) → 401 | `[x]` (✓ Go) |
| `POST /logout`                      | ok → 204                         | `[x]` (✓ Go) |
|                                     | replay (already revoked) → 204 (idempotent) | `[x]` (✓ Go) |
|                                     | missing bearer → 401             | `[x]` (✓ Go) |
|                                     | invalid bearer → 401             | `[x]` (✓ Go) |

---

## todo (`evals/todo/todo.hurl`)

Inlines auth + RBAC over todos. Three roles: Admin, Manager, User.

| Endpoint                            | Variant                          | Status |
|-------------------------------------|----------------------------------|--------|
| `POST /signup` / `/login` / `/logout` | (auth coverage as in auth)     | `[x]`  |
| `POST /admin/users/:id/promote`     | ok (Admin promotes) → 200        | `[d]`  |
|                                     | wrong role (User tries) → 403    | `[x]`  |
| `POST /todos`                       | ok → 201                         | `[x]`  |
|                                     | err EmptyText → 422              | `[x]`  |
|                                     | idempotency replay               | `[x]`  |
| `PATCH /todos/:id`                  | ok (owner) → 200                 | `[x]`  |
|                                     | ok (admin) → 200                 | `[d]`  |
|                                     | ok (manager assigned) → 200      | `[d]`  |
|                                     | err NotAuthorized (other user) → 403 | `[x]` |
|                                     | err NotAuthorized (manager not assigned) → 403 | `[x]` |
|                                     | err TodoNotFound → 404           | `[d]`  |
| `POST /todos/:id/toggle`            | ok → 200                         | `[x]`  |
|                                     | err NotAuthorized → 403          | `[x]`  |
| `DELETE /todos/:id`                 | ok (owner deletes own user-todo) → 204 | `[x]` |
|                                     | ok (admin) → 204                 | `[d]`  |
|                                     | err NotAuthorized (owner deletes admin-todo) → 403 | `[d]` |
|                                     | err NotAuthorized (manager) → 403 | `[x]` |
| `POST /admin/todos/:id/assign`      | ok (Admin) → 200                 | `[d]`  |
|                                     | err NotManager (target is User) → 422 | `[d]` |
|                                     | wrong role → 403                 | `[x]`  |
| `GET /todos?filter=All`             | ok (Admin) → 200                 | `[d]`  |
|                                     | err NotAuthorized (User) → 403   | `[x]`  |

---

## airbnb/auth (`evals/airbnb/auth.hurl`) — 25 scenarios

The airbnb-internal auth feature with the role enum (Guest/Host/Admin).

| Endpoint                                 | Variant                               | Status |
|------------------------------------------|---------------------------------------|--------|
| `POST /signup` / `/login` / `/logout`    | (full auth coverage as in auth)       | `[x]`  |
| `POST /me/upgrade-to-host`               | ok (Guest) → 200                      | `[x]`  |
|                                          | err AlreadyHost → 409                 | `[x]`  |
|                                          | wrong role (Admin tries) → 403        | `[x]`  |
| `POST /admin/users/:id/promote`          | ok (Admin) → 200                      | `[x]`  |
|                                          | err InvalidPromotion (Admin → Guest) → 422 | `[x]` |
|                                          | wrong role → 403                      | `[x]`  |
| `POST /admin/users/:id/verify`           | ok → 200                              | `[x]`  |
|                                          | err AlreadyVerified → 409             | `[x]`  |
|                                          | err UserNotFound → 404                | `[x]`  |
|                                          | wrong role (Host tries) → 403         | `[x]`  |
|                                          | idempotency replay                    | `[x]`  |

---

## airbnb/listings (`evals/airbnb/listings.hurl`) — 14 scenarios

Minute-granular slots; HoldSlot/ReleaseSlot exported for the booking saga.

| Endpoint                              | Variant                              | Status |
|---------------------------------------|--------------------------------------|--------|
| `POST /listings`                      | ok (Host) → 201                      | `[x]`  |
|                                       | err InvalidListing → 422             | `[x]`  |
|                                       | wrong role (Guest) → 403             | `[x]`  |
| `PATCH /listings/:id`                 | ok (owner) → 200                     | `[x]`  |
|                                       | err InvalidUpdate → 422              | `[x]`  |
|                                       | err ListingNotFound → 404            | `[x]`  |
|                                       | wrong owner (other Host) → 403       | `[x]`  |
| `POST /listings/:id/publish`          | ok → 200                             | `[x]`  |
|                                       | err AlreadyListed → 409              | `[x]`  |
|                                       | err DraftIncomplete → 422            | `[x]`  |
| `POST /listings/:id/hide`             | ok → 200                             | `[x]`  |
|                                       | err AlreadyHidden → 409              | `[x]`  |
| `GET /listings`                       | ok → 200                             | `[x]`  |
|                                       | filter=AvailableInRange respects holds | `[x]` |

---

## airbnb/booking (`evals/airbnb/booking.hurl`) — saga + schedule

The canonical multi-actor saga: hold → validate coupon → charge → redeem.

| Endpoint                              | Variant                                              | Status |
|---------------------------------------|------------------------------------------------------|--------|
| `POST /bookings`                      | ok (happy path) → 201                                | `[x]`  |
|                                       | err SlotUnavailable → 409                            | `[x]`  |
|                                       | err PaymentDeclined → 402 (compensation: dates released) | `[d]` |
|                                       | err CouponConflict → 422 (compensation: charge refunded, dates released) | `[d]` |
|                                       | idempotency replay                                   | `[x]`  |
| `POST /bookings/with-fallback`        | ok (Stripe primary) → 201                            | `[d]`  |
|                                       | ok (fallback to Polar) → 201                         | `[d]`  |
|                                       | err AllProvidersFailed → 502                         | `[d]`  |
| `POST /bookings/:id/cancel`           | ok Free refund (early) → 200                         | `[~]`  |
|                                       | ok Partial refund → 200                              | `[~]`  |
|                                       | ok NonRefundable → 200 (refund == 0)                 | `[~]`  |
|                                       | err BookingStarted → 409                             | `[~]`  |
|                                       | err BookingNotFound → 404                            | `[x]`  |
| `POST /bookings/:id/complete`         | ok → 200 (transfer fired)                            | `[d]`  |
|                                       | err NotConfirmed → 409                               | `[x]`  |
|                                       | err BookingNotEnded → 409                            | `[x]`  |
| `GET /bookings/:id`                   | ok (guest) → 200                                     | `[x]`  |
|                                       | ok (host) → 200                                      | `[x]`  |
|                                       | err NotAuthorized (other user) → 403                 | `[x]`  |
| `schedule SendBookingReminder`        | reminder fires 60s before checkin                    | `[~]`  |
|                                       | reminder doesn't refire (`reminder_sent` guard)      | `[~]`  |

---

## airbnb/coupons (`evals/airbnb/coupons.hurl`) — 16 scenarios

| Endpoint                              | Variant                          | Status |
|---------------------------------------|----------------------------------|--------|
| `POST /admin/coupons`                 | ok → 201                         | `[x]`  |
|                                       | err InvalidCoupon → 422          | `[x]`  |
|                                       | err CodeTaken → 409              | `[x]`  |
|                                       | wrong role → 403                 | `[x]`  |
| `DELETE /admin/coupons/:id`           | ok → 204                         | `[x]`  |
|                                       | err CouponNotFound → 404         | `[x]`  |
|                                       | err CouponInUse → 409            | `[d]`  |
| `GET /coupons/:code/validate`         | ok → 200                         | `[x]`  |
|                                       | err CouponNotFound → 404         | `[x]`  |
|                                       | err Expired → 410                | `[x]`  |
|                                       | err Exhausted → 410              | `[d]`  |
|                                       | err AlreadyRedeemed → 409        | `[d]`  |

---

## wallet (`evals/wallet/wallet.hurl`) — 35 scenarios; in-flight target Go

Inlined auth; Admin funds, users transact, scheduled transfers fire on a TIME-axis schedule.

| Endpoint                              | Variant                              | Status |
|---------------------------------------|--------------------------------------|--------|
| `POST /signup` / `/login` / `/logout` | (auth coverage)                      | `[x]`  |
| `POST /admin/wallets/:owner/fund`     | ok → 201                             | `[x]`  |
|                                       | err InvalidAmount → 422              | `[x]`  |
|                                       | err WalletNotFound → 404             | `[x]`  |
|                                       | wrong role (User) → 403              | `[x]`  |
| `GET /wallets/me`                     | ok → 200                             | `[x]`  |
| `POST /wallets/me/withdraw`           | ok → 201                             | `[x]`  |
|                                       | err InsufficientFunds → 409          | `[x]`  |
|                                       | err InvalidAmount → 422              | `[x]`  |
|                                       | wrong owner (someone else's) → 403   | `[x]`  |
| `POST /transfers`                     | ok → 201                             | `[x]`  |
|                                       | err InsufficientFunds → 409          | `[x]`  |
|                                       | err SelfTransfer → 422               | `[x]`  |
|                                       | err WalletNotFound → 404             | `[x]`  |
|                                       | err NotAuthorized (other user's wallet) → 403 | `[x]` |
|                                       | replay → same response, no double-debit | `[x]` |
| `POST /transfers/schedule`            | ok → 201                             | `[x]`  |
|                                       | err InvalidAmount → 422              | `[x]`  |
| `POST /transfers/schedule/:id/cancel` | ok → 200                             | `[x]`  |
|                                       | err AlreadyExecuted → 409            | `[x]`  |
|                                       | err NotAuthorized → 403              | `[x]`  |
| `schedule ExecuteScheduledTransfer`   | fires at fire_at, executes transfer  | `[x]`  |
|                                       | doesn't fire if cancelled            | `[x]`  |

---

## billing (`evals/billing/billing.hurl`) — Polar canonical; 60s test plans

Inlined auth (Admin/Customer). Scheduled `ChargeCycle` is the TIME-axis test;
the charge-success/failure paths are deferred until a Polar mock harness lands.

| Endpoint                              | Variant                              | Status |
|---------------------------------------|--------------------------------------|--------|
| `POST /signup` / `/login` / `/logout` | (auth coverage)                      | `[x]`  |
| `POST /admin/plans`                   | ok (Admin, 60s test plan) → 201      | `[x]`  |
|                                       | wrong role → 403                     | `[x]`  |
| `DELETE /admin/plans/:id`             | ok → 204                             | `[x]`  |
| `POST /subscriptions`                 | ok → 201                             | `[x]`  |
|                                       | err InvalidPlan → 422                | `[x]`  |
| `POST /subscriptions/:id/cancel`      | ok → 200                             | `[x]`  |
|                                       | err AlreadyCancelled → 409           | `[x]`  |
| `POST /subscriptions/:id/reactivate`  | ok → 200                             | `[d]`  |
|                                       | err NotSuspended → 409               | `[x]`  |
| `schedule ChargeCycle (60s plan)`     | fires within 60s of next_charge_date | `[~]`  |
|                                       | charge succeeds → status: Active     | `[d]`  |
|                                       | charge fails → status: PastDue       | `[d]`  |
| `schedule RetryDue`                   | retries after 60s test threshold     | `[d]`  |
| `schedule EscalateOverdue`            | suspends after attempts >= 3         | `[d]`  |

---

## notifications (`evals/notifications/notifications.hurl`) — Postmark canonical

Inlined auth (Admin/User); multi-provider rescue chains tested via deferred
worker scenarios. Admin read-only endpoints exercised live.

| Endpoint                                 | Variant                                | Status |
|------------------------------------------|----------------------------------------|--------|
| `POST /signup` / `/login` / `/logout`    | (auth coverage)                        | `[x]`  |
| Worker subscribes UserSignedUp           | dispatches Email[Postmark]             | `[d]`  |
|                                          | Postmark fails → falls back to SendGrid | `[d]` |
|                                          | all providers fail → NotificationFailed | `[d]` |
| Worker subscribes OrderShipped           | dispatches Email + SMS (if phone)      | `[d]`  |
| Webhook: Email Delivered                 | Notification.MarkSent fires            | `[d]`  |
| `GET /admin/notifications/:id`           | ok (Admin) → 200                       | `[x]`  |
|                                          | wrong role → 403                       | `[x]`  |
|                                          | missing bearer → 401                   | `[x]`  |
| `GET /admin/notifications?status=Failed` | ok (Admin) → 200                       | `[x]`  |
|                                          | wrong role → 403                       | `[x]`  |

---

## Summary

| Feature              | Hurl scenarios | `[x]` count | `[d]` count | `[~]` count | Verified target(s) |
|----------------------|----------------|-------------|-------------|-------------|---------------------|
| auth                 | 14             | 13          | 0           | 0           | Go (PR #45)         |
| todo                 | 31             | ~16         | ~9          | 0           | Go in flight        |
| wallet               | 35             | ~22         | 0           | 0           | Go in flight        |
| billing              | 18             | ~10         | ~6          | 1           | —                   |
| notifications        | 12             | ~7          | ~5          | 0           | —                   |
| airbnb/auth          | 25             | 11          | 0           | 0           | —                   |
| airbnb/listings      | 14             | 14          | 0           | 0           | —                   |
| airbnb/booking       | 26             | ~12         | ~7          | ~6          | —                   |
| airbnb/coupons       | 16             | 9           | 3           | 0           | —                   |

`[d]` rows are the documented gap — they require harness work that is
out of scope for v0.1 but documented in each feature's `.md`:

- **Webhook handler tests** (notifications.MarkSent, payments confirmation)
  need a test-mode in the external SDK adapter.
- **Failure-injection compensation tests** (forcing `Payments.Charge` to
  fail to verify `HoldDates` rolls back) need an external mock.
- **Cross-target invariant tests** (the same scenario producing
  byte-identical state across all four targets) come after #17.

When a `[d]` becomes runnable, flip it to `[x]` here and add a scenario
to the relevant `.hurl`.
