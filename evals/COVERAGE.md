# Coverage checklist

Per-feature, per-endpoint, per-response-variant matrix. A row is checked
when the corresponding scenario lands in the feature's `.hurl` file (and
the `.md` documents any deferred parts).

## Notation

- `[ ]` — not yet covered
- `[x]` — covered (scenario in `.hurl`)
- `[d]` — documented in `.md`, deferred until harness exists
- `[~]` — partially covered

---

## auth (`evals/auth/auth.hurl`)

| Endpoint                            | Variant                          | Status |
|-------------------------------------|----------------------------------|--------|
| `POST /signup`                      | ok → 201                         | `[ ]`  |
|                                     | err WeakPassword (TooShort) → 422 | `[ ]`  |
|                                     | err WeakPassword (MissingDigit) → 422 | `[ ]` |
|                                     | err WeakPassword (InBlocklist) → 422 | `[ ]` |
|                                     | err EmailTaken → 409             | `[ ]`  |
|                                     | idempotency replay (same key)    | `[ ]`  |
| `POST /login`                       | ok → 200                         | `[ ]`  |
|                                     | err InvalidCredentials (wrong pw) → 401 | `[ ]` |
|                                     | err InvalidCredentials (no user) → 401 | `[ ]` |
| `POST /logout`                      | ok → 204                         | `[ ]`  |
|                                     | replay (already revoked) → 204 (idempotent) | `[ ]` |
|                                     | missing bearer → 401             | `[ ]`  |
|                                     | invalid bearer → 401             | `[ ]`  |

---

## todo (`evals/todo/todo.hurl`)

Inlines auth + RBAC over todos. Three roles: Admin, Manager, User.

| Endpoint                            | Variant                          | Status |
|-------------------------------------|----------------------------------|--------|
| `POST /signup` / `/login` / `/logout` | (auth coverage as in auth)     | `[ ]`  |
| `POST /admin/users/:id/promote`     | ok (Admin promotes) → 200        | `[ ]`  |
|                                     | wrong role (User tries) → 403    | `[ ]`  |
| `POST /todos`                       | ok → 201                         | `[ ]`  |
|                                     | err EmptyText → 422              | `[ ]`  |
|                                     | idempotency replay               | `[ ]`  |
| `PATCH /todos/:id`                  | ok (owner) → 200                 | `[ ]`  |
|                                     | ok (admin) → 200                 | `[ ]`  |
|                                     | ok (manager assigned) → 200      | `[ ]`  |
|                                     | err NotAuthorized (other user) → 403 | `[ ]` |
|                                     | err NotAuthorized (manager not assigned) → 403 | `[ ]` |
|                                     | err TodoNotFound → 404           | `[ ]`  |
| `POST /todos/:id/toggle`            | ok → 200                         | `[ ]`  |
|                                     | err NotAuthorized → 403          | `[ ]`  |
| `DELETE /todos/:id`                 | ok (owner deletes own user-todo) → 204 | `[ ]` |
|                                     | ok (admin) → 204                 | `[ ]`  |
|                                     | err NotAuthorized (owner deletes admin-todo) → 403 | `[ ]` |
|                                     | err NotAuthorized (manager) → 403 | `[ ]` |
| `POST /admin/todos/:id/assign`      | ok (Admin) → 200                 | `[ ]`  |
|                                     | err NotManager (target is User) → 422 | `[ ]` |
|                                     | wrong role → 403                 | `[ ]`  |
| `GET /todos?filter=All`             | ok (Admin) → 200                 | `[ ]`  |
|                                     | err NotAuthorized (User) → 403   | `[ ]`  |

---

## airbnb/auth (`evals/airbnb/auth.hurl`) — issue #10

The airbnb-internal auth feature with the role enum (Guest/Host/Admin).

| Endpoint                                 | Variant                               | Status |
|------------------------------------------|---------------------------------------|--------|
| `POST /signup` / `/login` / `/logout`    | (full auth coverage as in auth)       | `[ ]`  |
| `POST /me/upgrade-to-host`               | ok (Guest) → 200                      | `[ ]`  |
|                                          | err AlreadyHost → 409                 | `[ ]`  |
|                                          | wrong role (Admin tries) → 403        | `[ ]`  |
| `POST /admin/users/:id/promote`          | ok (Admin) → 200                      | `[ ]`  |
|                                          | err InvalidPromotion (Admin → Guest) → 422 | `[ ]` |
|                                          | wrong role → 403                      | `[ ]`  |
| `POST /admin/users/:id/verify`           | ok → 200                              | `[ ]`  |
|                                          | err AlreadyVerified → 409             | `[ ]`  |
|                                          | err UserNotFound → 404                | `[ ]`  |

---

## airbnb/listings (`evals/airbnb/listings.hurl`) — issue #11

Minute-granular slots; HoldSlot/ReleaseSlot exported for the booking saga.

| Endpoint                              | Variant                              | Status |
|---------------------------------------|--------------------------------------|--------|
| `POST /listings`                      | ok (Host) → 201                      | `[ ]`  |
|                                       | err InvalidListing → 422             | `[ ]`  |
|                                       | wrong role (Guest) → 403             | `[ ]`  |
| `PATCH /listings/:id`                 | ok (owner) → 200                     | `[ ]`  |
|                                       | err InvalidUpdate → 422              | `[ ]`  |
|                                       | err ListingNotFound → 404            | `[ ]`  |
|                                       | wrong owner (other Host) → 403       | `[ ]`  |
| `POST /listings/:id/publish`          | ok → 200                             | `[ ]`  |
|                                       | err AlreadyListed → 409              | `[ ]`  |
|                                       | err DraftIncomplete → 422            | `[ ]`  |
| `POST /listings/:id/hide`             | ok → 200                             | `[ ]`  |
|                                       | err AlreadyHidden → 409              | `[ ]`  |
| `GET /listings`                       | ok → 200                             | `[ ]`  |
|                                       | filter=AvailableInRange respects holds | `[ ]` |

---

## airbnb/booking (`evals/airbnb/booking.hurl`) — issue #11

The canonical multi-actor saga: hold → validate coupon → charge → redeem.

| Endpoint                              | Variant                                              | Status |
|---------------------------------------|------------------------------------------------------|--------|
| `POST /bookings`                      | ok (happy path) → 201                                | `[ ]`  |
|                                       | err SlotUnavailable → 409                            | `[ ]`  |
|                                       | err PaymentDeclined → 402 (compensation: dates released) | `[d]` |
|                                       | err CouponConflict → 422 (compensation: charge refunded, dates released) | `[d]` |
|                                       | idempotency replay                                   | `[ ]`  |
| `POST /bookings/with-fallback`        | ok (Stripe primary) → 201                            | `[d]`  |
|                                       | ok (fallback to Polar) → 201                         | `[d]`  |
|                                       | err AllProvidersFailed → 502                         | `[d]`  |
| `POST /bookings/:id/cancel`           | ok Free refund (early) → 200                         | `[ ]`  |
|                                       | ok Partial refund → 200                              | `[ ]`  |
|                                       | ok NonRefundable → 200 (refund == 0)                 | `[ ]`  |
|                                       | err BookingStarted → 409                             | `[ ]`  |
|                                       | err BookingNotFound → 404                            | `[ ]`  |
| `POST /bookings/:id/complete`         | ok → 200 (transfer fired)                            | `[d]`  |
|                                       | err NotConfirmed → 409                               | `[ ]`  |
|                                       | err BookingNotEnded → 409                            | `[ ]`  |
| `GET /bookings/:id`                   | ok (guest) → 200                                     | `[ ]`  |
|                                       | ok (host) → 200                                      | `[ ]`  |
|                                       | err NotAuthorized (other user) → 403                 | `[ ]`  |
| `schedule SendBookingReminder`        | reminder fires 60s before checkin                    | `[ ]`  |
|                                       | reminder doesn't refire (`reminder_sent` guard)      | `[ ]`  |

---

## airbnb/coupons (`evals/airbnb/coupons.hurl`) — issue #11

| Endpoint                              | Variant                          | Status |
|---------------------------------------|----------------------------------|--------|
| `POST /admin/coupons`                 | ok → 201                         | `[ ]`  |
|                                       | err InvalidCoupon → 422          | `[ ]`  |
|                                       | err CodeTaken → 409              | `[ ]`  |
|                                       | wrong role → 403                 | `[ ]`  |
| `DELETE /admin/coupons/:id`           | ok → 204                         | `[ ]`  |
|                                       | err CouponNotFound → 404         | `[ ]`  |
|                                       | err CouponInUse → 409            | `[ ]`  |
| `GET /coupons/:code/validate`         | ok → 200                         | `[ ]`  |
|                                       | err CouponNotFound → 404         | `[ ]`  |
|                                       | err Expired → 410                | `[ ]`  |
|                                       | err Exhausted → 410              | `[ ]`  |
|                                       | err AlreadyRedeemed → 409        | `[ ]`  |

---

## wallet (`evals/wallet/wallet.hurl`) — issue #11 + #28

Inlined auth; Admin funds, users transact, scheduled transfers.

| Endpoint                              | Variant                              | Status |
|---------------------------------------|--------------------------------------|--------|
| `POST /signup` / `/login` / `/logout` | (auth coverage)                      | `[ ]`  |
| `POST /admin/wallets/:owner/fund`     | ok → 201                             | `[ ]`  |
|                                       | err InvalidAmount → 422              | `[ ]`  |
|                                       | err WalletNotFound → 404             | `[ ]`  |
|                                       | wrong role (User) → 403              | `[ ]`  |
| `GET /wallets/me`                     | ok → 200                             | `[ ]`  |
| `POST /wallets/me/withdraw`           | ok → 201                             | `[ ]`  |
|                                       | err InsufficientFunds → 409          | `[ ]`  |
|                                       | err InvalidAmount → 422              | `[ ]`  |
|                                       | wrong owner (someone else's) → 403   | `[ ]`  |
| `POST /transfers`                     | ok → 201                             | `[ ]`  |
|                                       | err InsufficientFunds → 409          | `[ ]`  |
|                                       | err SelfTransfer → 422               | `[ ]`  |
|                                       | err WalletNotFound → 404             | `[ ]`  |
|                                       | replay → same response, no double-debit (state readback) | `[ ]` |
| `POST /transfers/schedule`            | ok → 201                             | `[ ]`  |
|                                       | err InvalidAmount → 422              | `[ ]`  |
| `POST /transfers/schedule/:id/cancel` | ok → 200                             | `[ ]`  |
|                                       | err AlreadyExecuted → 409            | `[ ]`  |
|                                       | err NotAuthorized → 403              | `[ ]`  |
| `schedule ExecuteScheduledTransfer`   | fires at fire_at, executes transfer  | `[ ]`  |
|                                       | doesn't fire if cancelled            | `[ ]`  |

---

## billing (`evals/billing/billing.hurl`) — issue #28

Inlined auth (Admin/Customer); Polar canonical; 60s test plans.

| Endpoint                              | Variant                              | Status |
|---------------------------------------|--------------------------------------|--------|
| `POST /signup` / `/login` / `/logout` | (auth coverage)                      | `[ ]`  |
| `POST /admin/plans`                   | ok (Admin, 60s test plan) → 201      | `[ ]`  |
|                                       | wrong role → 403                     | `[ ]`  |
| `DELETE /admin/plans/:id`             | ok → 204                             | `[ ]`  |
| `POST /subscriptions`                 | ok → 201                             | `[ ]`  |
|                                       | err InvalidPlan → 422                | `[ ]`  |
| `POST /subscriptions/:id/cancel`      | ok → 200                             | `[ ]`  |
|                                       | err AlreadyCancelled → 409           | `[ ]`  |
| `POST /subscriptions/:id/reactivate`  | ok → 200                             | `[ ]`  |
|                                       | err NotSuspended → 409               | `[ ]`  |
| `schedule ChargeCycle (60s plan)`     | fires within 60s of next_charge_date | `[ ]`  |
|                                       | charge succeeds → status: Active     | `[d]`  |
|                                       | charge fails → status: PastDue       | `[d]`  |
| `schedule RetryDue`                   | retries after 60s test threshold     | `[d]`  |
| `schedule EscalateOverdue`            | suspends after attempts >= 3         | `[d]`  |

---

## notifications (`evals/notifications/notifications.hurl`) — issue #28

Inlined auth (Admin/User); Postmark canonical; multi-provider rescue chains.

| Endpoint                              | Variant                              | Status |
|---------------------------------------|--------------------------------------|--------|
| `POST /signup` / `/login` / `/logout` | (auth coverage)                      | `[ ]`  |
| Worker subscribes UserSignedUp        | dispatches Email[Postmark]           | `[d]`  |
|                                       | Postmark fails → falls back to SendGrid | `[d]` |
|                                       | all providers fail → NotificationFailed | `[d]` |
| Worker subscribes OrderShipped        | dispatches Email + SMS (if phone)    | `[d]`  |
| Webhook: Email Delivered              | Notification.MarkSent fires          | `[d]`  |
| `GET /admin/notifications/:id`        | ok (Admin) → 200                     | `[ ]`  |
|                                       | wrong role → 403                     | `[ ]`  |
| `GET /admin/notifications?status=Failed` | ok (Admin) → 200                  | `[ ]`  |
