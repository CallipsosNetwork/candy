# airbnb/booking — Scenario narrative

Feature: `booking.candy` — PlaceBooking saga, PlaceBookingWithFallback, CancelBooking,
CompleteBooking, SendBookingReminder schedule.
Hurl: `evals/airbnb/booking.hurl`
Issue: #11 — "canonical multi-actor saga"

PlaceBooking is the centrepiece of the marketplace. It is a four-step saga that touches
three owned actors (Listing's Calendar, Coupon, Booking) and one external network boundary
(Payments). The eval bootstraps all actors inline: admin, host, guest, one listing, and
two coupons. Every scenario is self-contained relative to that shared setup.

---

## Setup

Execution order at the top of the `.hurl` file:

1. **Admin signup** — `admin@candy.local`. Admin bootstrap follows the same
   harness-injection pattern documented in `auth.md`: either a
   `/internal/seed-admin` test-mode endpoint or `--variable admin_token=<value>`
   supplied OOB. Admin is needed to create coupons.
2. **Host signup** — `host@candy.local`. Capture `host_token` and `host_user_id`.
3. **Host self-upgrade** — POST `/me/upgrade-to-host` with `host_token`. Now Host-role.
4. **Guest signup** — `guest@candy.local`. Capture `guest_token` and `guest_user_id`.
5. **Other-user signup** — `other@candy.local`. Capture `other_token`. Used only for
   the GET /bookings/:id authorization check.
6. **Create listing** — POST `/listings` as Host. Body uses fixture values
   (`listing_title`, `listing_price_per_minute`, etc.). Capture `listing_id`.
7. **Publish listing** — POST `/listings/{{listing_id}}/publish` as Host.
8. **Create 50% coupon** — POST `/admin/coupons` as Admin, code `TEST50`, kind `Percent`,
   value `50`, maxUses `100`, expires far in future. Capture `coupon_50_id`.
9. **Create 100% coupon** — POST `/admin/coupons` as Admin, code `FULL100`, kind
   `Percent`, value `100`, maxUses `100`, expires far in future. Capture
   `coupon_100_id`. Used for zero-total booking.

All tokens and ids captured via `[Captures]`.

---

## Time math — runner contract

Hurl 4.x has no native timestamp arithmetic. Booking `dates.from` and `dates.to`
fields must be ISO-8601 timestamps. Three approaches are used in this file:

**Static far-future dates** are used for most booking scenarios. The happy-path
booking uses `dates.from = 2099-01-01T12:00:00Z`, `dates.to = 2099-01-01T12:20:00Z`
(20 minutes). This is a 20-minute block, so `total = pricePerMinute * 20 = 50 * 20 = 1000`
minor units. These dates are fixed and reproducible across runs; they will never be
"in the past" from a business logic standpoint until the year 2099.

**The cancellation window tests** require control over where `now` falls relative to
`dates.from`. Because the cancellation thresholds are 60s (Free) and 30s (Partial/
NonRefundable), we use three separate bookings with progressively tighter dates —
but since static future dates can always be > 60s away, we instead use the harness
`--variable` injection mechanism for the cancellation tests:

> The harness must inject `booking_start_free`, `booking_start_partial`, and
> `booking_start_nonrefundable` as RFC-3339 timestamps relative to test-run time:
> - `booking_start_free`: `now + 120s` (within Free window: > 60s out)
> - `booking_start_partial`: `now + 45s` (within Partial window: 30–60s out)
> - `booking_start_nonrefundable`: `now + 15s` (within NonRefundable: < 30s out)
>
> Each `booking_end_*` is `booking_start_* + 1200s` (20 min later).

Scenarios that depend on injected timestamps are marked `[~]` (partial — runs once
runner injects current time). Static-date scenarios have no such marker.

**The SendBookingReminder schedule test** requires a booking that starts ~90s from
now, then waits for the schedule to fire. This is documented in its own section below.

---

## Scenarios

### Group A — Happy path PlaceBooking

#### A1 — PlaceBooking, no coupon → 201

POST `/bookings` as `guest_token`. Body:

- `listing`: `{{listing_id}}`
- `dates.from`: `2099-01-01T12:00:00Z`
- `dates.to`: `2099-01-01T12:20:00Z` (20 minutes)
- `source`: `{{test_payment_method}}`
- `code`: omitted (no coupon)
- `idempotency_key`: `{{place_booking_key}}`

Expect 201 with `booking_id` (non-empty) and `total: 1000` (50 minor units/min × 20 min).
Capture `booking_id`.

The booking is created in `Pending` status — the ChargeSucceeded webhook has not yet
arrived. Depending on test mode, the backend may auto-confirm synchronously (test Stripe
sandbox) or remain Pending until the webhook fires. The `status` assertion in A4 (GET)
handles both: assert `status` is one of `"pending"` or `"confirmed"`.

#### A2 — PlaceBooking, 50% coupon → 201

POST `/bookings` as `guest_token`. Same listing and dates window
(`2099-06-01T10:00:00Z` to `2099-06-01T10:20:00Z` — different slot so no conflict
with A1). Body adds `code: "TEST50"`. `idempotency_key`: `place-booking-50pct-001`.

Expected: 201, `total: 500` (50% of 1000). Capture `booking_id_50pct`.

The `discount` in the Booking actor state should equal `500`.

#### A3 — PlaceBooking, 100% coupon → 201

POST `/bookings` as `guest_token`. Slot: `2099-07-01T09:00:00Z` to
`2099-07-01T09:20:00Z`. Body: `code: "FULL100"`. `idempotency_key`:
`place-booking-full-001`.

Expected: 201, `total: 0`. A zero-total booking is valid; the saga calls
`Payments.Charge(0, ...)` which should return a no-op charge id in test mode.
Capture `booking_id_full`.

---

### Group B — Saga failures and compensation

#### B1 — SlotUnavailable: double-booking the same slot → 409

POST `/bookings` as `guest_token`. Attempt to book the **same slot** as A1
(`2099-01-01T12:00:00Z` to `2099-01-01T12:20:00Z`). Use idempotency key
`place-booking-slot-conflict-001` (a fresh key — this is a genuine second request,
not a replay).

Expected: 409, `error: "slot_unavailable"`.

State readback: GET `/bookings/{{booking_id}}` should still return 200 — the
first booking is unaffected. The Calendar actor's hold from A1 remains intact.

This is the only compensation scenario testable without a mock: the saga reaches
HoldSlot, which rejects SlotUnavailable immediately (no prior successful step to
compensate). The Calendar state is not mutated for the second attempt — confirmed
by the fact that the A1 booking is still accessible.

#### B2 — PaymentDeclined → 402 `[d]`

**Deferred — requires Payments mock to force decline.**

Scenario: POST `/bookings` with a payment method that the Payments[Stripe] sandbox
will decline (e.g. Stripe test card `pm_card_chargeDeclined`). The saga should:

1. Hold the slot (step 1 succeeds).
2. Charge fails → saga calls `ReleaseSlot` (compensation).
3. Return 402, `error: "payment_declined"`.

Verification: the booking is **not** created (no `booking_id` is returned); the
Calendar hold is released (booking the same slot again should succeed in a
subsequent request). This compensation test cannot be exercised with the default
test payment method, which always succeeds.

> When the harness exposes a mock Payments adapter that can be configured to decline
> a specific source token, remove `[d]` and add the hurl block.

#### B3 — CouponConflict (race: validate succeeds, redeem fails) → 422 `[d]`

**Deferred — requires concurrency simulator or coupon exhaustion trick.**

Scenario: Two guests attempt to use a single-use coupon (`maxUses: 1`) simultaneously.
Guest A validates (step 2 succeeds), guest B validates and redeems first (exhausting
the coupon), then guest A's saga reaches RedeemCoupon which rejects Exhausted.
The saga should:

1. Refund guest A's charge (`Payments[Stripe].Refund` — compensation for step 4).
2. Release guest A's slot hold (compensation for step 1).
3. Return 422, `error: "coupon_conflict"`.

A single-threaded approximation: create a coupon with `maxUses: 1`, book with it
successfully (consuming the one use), then attempt a second booking from a different
guest with the same code. The second booking will fail at **ValidateCoupon** (step 2),
not at RedeemCoupon — but the error returned is still 422 `coupon_conflict` if the
flow treats Exhausted-at-validate as CouponConflict. If ValidateCoupon's Exhausted
maps differently, this becomes a different error code.

The true race (validate succeeds, concurrent redeem exhausts, redeem fails) requires
either a concurrency harness or a test-mode "fail-after-validate" injection.

> When the harness supports either concurrent request firing or step-level failure
> injection, remove `[d]` and add the full hurl block.

---

### Group C — Idempotency

#### C1 — PlaceBooking idempotency replay

Replay the exact A1 request — same body, same `idempotency_key: {{place_booking_key}}`.

Expected: 201 again, same `booking_id` as captured in A1, same `total: 1000`.
No new booking record created. Assert `booking_id` equals the value captured in A1.

---

### Group D — PlaceBookingWithFallback `[d]`

**All three sub-cases deferred — require provider failure injection.**

The `/bookings/with-fallback` endpoint exercises the three-provider rescue chain
(Stripe → Polar → Lemon). All three variants require the harness to configure
which providers succeed or fail:

#### D1 — Stripe primary succeeds → 201 `[d]`

Stripe accepts the charge. Response includes `provider: "stripe"`.

#### D2 — Stripe fails, Polar succeeds (fallback) → 201 `[d]`

Stripe is configured to decline. Polar accepts. Response includes
`provider: "polar"`. The slot hold is preserved throughout (no compensation fires
on a per-provider failure — the rescue chain falls through to the next provider
without releasing the hold).

#### D3 — All providers fail → 502 `[d]`

Stripe, Polar, and Lemon all configured to decline. Saga compensates:
`ReleaseSlot` fires after all three providers fail. Response: 502,
`error: "all_providers_failed"`. Booking is not created.

> Remove `[d]` when the harness exposes provider configuration endpoints or
> environment flags that control per-provider test-mode behavior.

---

### Group E — CancelBooking

Cancellation tests require bookings whose `dates.from` falls in the correct
window relative to `now` at cancel time. Free-window and Partial-window tests
use runner-injected timestamps (marked `[~]`). The BookingStarted and
BookingNotFound tests are static and unconditional.

A note on booking state: the CancelBooking flow requires `status == Pending or
status == Confirmed`. Bookings placed in scenarios A1–A3 may be Pending or
Confirmed depending on whether the test backend auto-confirms. The `cancel`
flow should succeed regardless of which confirmed-or-pending state the booking is in.

#### E1 — Free refund (early cancellation) → 200 `[~]`

**Runner must inject `booking_start_free` = `now + 120s`,
`booking_end_free` = `now + 1320s`.**

1. POST `/bookings` (no coupon, `booking_start_free` / `booking_end_free` dates,
   idempotency key `place-booking-cancel-free-001`). Capture `booking_id_cancel_free`.
2. Immediately POST `/bookings/{{booking_id_cancel_free}}/cancel` with
   `reason: "changed my mind"`, `idempotency_key: {{cancel_booking_key}}`.

Expected: 200, `refund: 1000` (full amount — Free window gives 100% back).

The `dates.from` is 120s in the future (> 60s threshold), so CancellationPolicy
returns `Free`.

#### E2 — Partial refund → 200 `[~]`

**Runner must inject `booking_start_partial` = `now + 45s`,
`booking_end_partial` = `now + 1245s`.**

1. POST `/bookings` (no coupon, partial-window dates, key `place-booking-cancel-partial-001`).
   Capture `booking_id_cancel_partial`.
2. Immediately POST `/bookings/{{booking_id_cancel_partial}}/cancel`,
   key `cancel-booking-partial-001`.

Expected: 200, `refund: 500` (50% of 1000 — Partial window).

The `dates.from` is 45s out (between 30s and 60s thresholds), so the policy
returns `Partial`.

#### E3 — NonRefundable → 200 (refund == 0) `[~]`

**Runner must inject `booking_start_nonrefundable` = `now + 15s`,
`booking_end_nonrefundable` = `now + 1215s`.**

1. POST `/bookings` (no coupon, nonrefundable-window dates, key
   `place-booking-cancel-nr-001`). Capture `booking_id_cancel_nr`.
2. Immediately POST `/bookings/{{booking_id_cancel_nr}}/cancel`,
   key `cancel-booking-nr-001`.

Expected: 200, `refund: 0` — flow succeeds (booking is cancelled), no refund issued.

The `dates.from` is 15s out (< 30s threshold), so the policy returns `NonRefundable`.

#### E4 — BookingStarted (cancel after check-in) → 409 `[~]`

**Runner must inject `booking_start_past` = `now - 10s`,
`booking_end_past` = `now + 1190s`.**

For this scenario we need a booking whose `dates.from` is already in the past.
Place the booking with `dates.from = now - 10s` (check-in started 10s ago).
Attempt to cancel it.

Expected: 409, `error: "booking_started"`.

CancellationPolicy sees `dates.from < now` and returns BookingStarted.

> **Judgment call**: placing a booking whose `dates.from` is in the past may itself
> be rejected by the backend (invalid booking). If `PlaceBooking` validates that
> `dates.from > now` at the time of booking, this scenario needs the runner to place
> the booking with `dates.from = now + 120s` and then sleep 130s before cancelling —
> making it a schedule-dependent test. In that case mark `[d]` and rely on the same
> infrastructure as the SendBookingReminder schedule test.

#### E5 — BookingNotFound → 404

POST `/bookings/00000000-0000-0000-0000-000000000000/cancel` (sentinel non-existent id).
Body: `reason: "no reason"`, `idempotency_key: cancel-not-found-001`.

Expected: 404, `error: "booking_not_found"`.

---

### Group F — CompleteBooking

CompleteBooking requires `status == Confirmed` (not Pending) and `now >= dates.to`
(the stay must have ended). The happy path is deferred because both conditions
require either waiting for real time to pass or harness-level time injection.

#### F1 — Complete happy path → 200 `[d]`

**Deferred — requires harness to advance time past `dates.to` and to have a
Confirmed booking (requires ChargeSucceeded webhook or auto-confirm in test mode).**

Scenario:

1. Place a booking with `dates.to = now + 60s` so the stay ends quickly.
2. Wait for the booking to be Confirmed (via webhook or test-mode auto-confirm).
3. Wait for `now >= dates.to` (sleep 60s+).
4. POST `/bookings/{{booking_id}}/complete` as Admin.

Expected: 200, `transfer_id` present (non-empty). The host payout is 88% of total
(`1000 * 0.88 = 880` minor units).

> When the harness supports time advancement or the backend exposes a
> `/internal/tick-time` endpoint, remove `[d]`.

#### F2 — NotConfirmed → 409

POST `/bookings/{{booking_id}}/complete` as Admin, where `{{booking_id}}` refers to
a booking created in A1 that is still in `Pending` status (webhook not yet delivered).

Expected: 409, `error: "not_confirmed"`.

> **Note**: if the test backend auto-confirms bookings synchronously, this scenario
> is unreachable without a way to place a booking that stays Pending. In that case
> the backend's implementation differs from the spec's async path; document as a
> known deviation.

#### F3 — BookingNotEnded → 409

POST `/bookings/{{booking_id}}/complete` as Admin, where the booking's `dates.to`
is far in the future (e.g. the A1 booking ending `2099-01-01T12:20:00Z`).

Expected: 409, `error: "booking_not_ended"`.

The check is `if now < b.dates.to then reject BookingNotEnded`. Since the year is
2099, this will always fail unless the backend is running in 2099.

---

### Group G — GET /bookings/:id authorization

The `BookingAccess` policy permits: the booking's guest, the booking's host, or any
Admin. Any other authenticated user gets 403.

Uses `booking_id` captured from A1. The A1 booking's guest is `guest_user_id`, its
host is `host_user_id`.

#### G1 — Guest reads own booking → 200

GET `/bookings/{{booking_id}}` with `Authorization: Bearer {{guest_token}}`.

Expected: 200. Response body has `booking_id: {{booking_id}}`, `guest` matching
`guest_user_id`, `listing` matching `listing_id`. Assert `status` is present.

#### G2 — Host reads own booking → 200

GET `/bookings/{{booking_id}}` with `Authorization: Bearer {{host_token}}`.

Expected: 200. Same booking record.

#### G3 — Other user gets 403

GET `/bookings/{{booking_id}}` with `Authorization: Bearer {{other_token}}`.
`other@candy.local` is neither the guest nor the host.

Expected: 403, `error: "not_authorized"`.

---

### Group H — Schedule: SendBookingReminder `[~]`

The schedule predicate fires every minute, picking up Confirmed bookings where
`dates.from - 60s <= now < dates.from` and `reminder_sent == false`.

In test mode (60s threshold), this means a booking starting in 30–60s from now
would be picked up on the next 1-minute schedule tick.

**Runner contract**: The runner must:
1. Inject `booking_start_reminder = now + 90s`, `booking_end_reminder = now + 1290s`.
2. Place a booking at that time (H1) and confirm it if the backend does not
   auto-confirm (H2).
3. Wait 30 seconds — `reminder_sent` should still be false (H3).
4. Wait another 30 seconds (60s total from booking) — the schedule tick fires,
   `reminder_sent` should now be true (H4).

All rows in this group are `[~]` because they require sleep support in the runner.

#### H1 — Place and confirm a near-future booking `[~]`

POST `/bookings` with `dates.from = {{booking_start_reminder}}`. Capture
`booking_id_reminder`. If the backend does not auto-confirm, also POST a mock
`ChargeSucceeded` webhook — or use an `/internal/confirm-booking` test endpoint.

#### H2 — Verify booking is Confirmed before sleeping `[~]`

GET `/bookings/{{booking_id_reminder}}`. Assert `status == "confirmed"` and
`reminder_sent == false`.

#### H3 — Read booking 30s later — reminder_sent still false `[~]`

After sleeping 30s: GET `/bookings/{{booking_id_reminder}}`.
Assert `reminder_sent == false`. The schedule has not yet fired because
`dates.from - 60s` is still 30s in the future.

#### H4 — Read booking 30s more later — reminder_sent is true `[~]`

After sleeping another 30s (60s total elapsed from H1): GET
`/bookings/{{booking_id_reminder}}`. Assert `reminder_sent == true`.

The schedule fired during this window: `dates.from - 60s <= now < dates.from`
is now satisfied. The schedule invoked `SendBookingReminder`, which emitted
`BookingReminderDue` and called `Booking.MarkReminderSent`.

#### H5 — Non-refire guard: reminder_sent remains true `[~]`

Wait another 30s and read the booking again. Assert `reminder_sent == true`
(not changed back to false or double-emitted). The `MarkReminderSent` accept
is idempotent; the schedule predicate's `reminder_sent == false` guard prevents
re-firing.

---

## Coverage summary

| #   | Endpoint                          | Variant                                              | Deferred? |
|-----|-----------------------------------|------------------------------------------------------|-----------|
| A1  | POST /bookings                    | ok (no coupon) → 201                                 |           |
| A2  | POST /bookings                    | ok (50% coupon) → 201, total halved                  |           |
| A3  | POST /bookings                    | ok (100% coupon) → 201, total == 0                   |           |
| B1  | POST /bookings                    | err SlotUnavailable → 409                            |           |
| B2  | POST /bookings                    | err PaymentDeclined → 402 (slot released)            | `[d]`     |
| B3  | POST /bookings                    | err CouponConflict → 422 (charge refunded + slot released) | `[d]` |
| C1  | POST /bookings                    | idempotency replay → 201 (same booking_id)           |           |
| D1  | POST /bookings/with-fallback      | ok (Stripe primary) → 201                            | `[d]`     |
| D2  | POST /bookings/with-fallback      | ok (fallback to Polar) → 201                         | `[d]`     |
| D3  | POST /bookings/with-fallback      | err AllProvidersFailed → 502                         | `[d]`     |
| E1  | POST /bookings/:id/cancel         | ok Free refund → 200                                 | `[~]`     |
| E2  | POST /bookings/:id/cancel         | ok Partial refund → 200                              | `[~]`     |
| E3  | POST /bookings/:id/cancel         | ok NonRefundable → 200 (refund == 0)                 | `[~]`     |
| E4  | POST /bookings/:id/cancel         | err BookingStarted → 409                             | `[~]`     |
| E5  | POST /bookings/:id/cancel         | err BookingNotFound → 404                            |           |
| F1  | POST /bookings/:id/complete       | ok → 200 (transfer fired)                            | `[d]`     |
| F2  | POST /bookings/:id/complete       | err NotConfirmed → 409                               |           |
| F3  | POST /bookings/:id/complete       | err BookingNotEnded → 409                            |           |
| G1  | GET /bookings/:id                 | ok (guest) → 200                                     |           |
| G2  | GET /bookings/:id                 | ok (host) → 200                                      |           |
| G3  | GET /bookings/:id                 | err NotAuthorized (other user) → 403                 |           |
| H1  | schedule SendBookingReminder      | place + confirm near-future booking                  | `[~]`     |
| H2  | schedule SendBookingReminder      | pre-sleep: reminder_sent == false                    | `[~]`     |
| H3  | schedule SendBookingReminder      | 30s in: reminder_sent still false                    | `[~]`     |
| H4  | schedule SendBookingReminder      | 60s in: reminder fires, reminder_sent == true        | `[~]`     |
| H5  | schedule SendBookingReminder      | reminder_sent guard — no refire                      | `[~]`     |

---

## Deferred items

**`[d]` — blocked on missing harness:**

- **B2 PaymentDeclined**: the default test payment method always succeeds. A mock
  Payments adapter or Stripe test card that declines is needed. The compensation path
  (ReleaseSlot) cannot be verified without it.

- **B3 CouponConflict (true race)**: the validate-then-redeem window is too narrow
  to exploit with sequential requests. The single-threaded approximation tests the
  ValidateCoupon failure path (which returns CouponConflict from step 2), not the
  RedeemCoupon failure path (which fires the full compensation chain of Refund +
  ReleaseSlot). A concurrency harness or step-level failure injection is needed for
  the full compensation verification.

- **D1–D3 PlaceBookingWithFallback**: all three cases require per-provider
  accept/decline configuration. Not possible with the default Stripe test sandbox
  which either always accepts or always declines globally.

- **F1 CompleteBooking happy path**: requires `status == Confirmed` (webhook delivery
  or auto-confirm) **and** `now >= dates.to`. Without harness-level time injection,
  the test must sleep until the booking's check-out time, which is impractical for
  CI-scale bookings and not supported in plain hurl.

**`[~]` — partial: runs once runner injects current-time offsets:**

- **E1–E4 cancellation window tests**: the CancellationPolicy thresholds are 60s
  and 30s. Testing each window requires booking dates within seconds of `now`.
  The runner must inject `booking_start_*` and `booking_end_*` variables computed
  as `now + offset`. Without injection, only static far-future dates are available,
  which always fall in the Free window.

- **H1–H5 SendBookingReminder schedule**: requires sleep between requests (30s × 2
  at minimum). Standard hurl does not support sleep steps. The runner must either
  support a `delay` directive or wrap hurl in a shell loop with `sleep`. The schedule
  infrastructure must be running in eval mode (1-minute tick active).

---

## Judgment calls

**Time math approach**: static far-future dates (2099) for stable happy-path and
negative tests; runner-injected variables for window-sensitive tests. This avoids
encoding timestamps in the `.hurl` that will expire or require recomputation on each
run. The tradeoff is that cancellation window tests and the schedule test cannot run
standalone — they need a thin harness wrapper.

**SendBookingReminder schedule test**: marked `[~]` rather than `[d]` because the
test itself is structurally sound — it just needs sleeps and a running scheduler. A
CI job that starts the scheduler and shells out `sleep 30; hurl ...; sleep 30; hurl ...`
can run it today. No mock adapter is needed.

**GET /bookings/:id authorization**: exercised by using three distinct tokens (guest,
host, other). The `other` user is a second guest created in setup. A missing-bearer
check is not separately listed in COVERAGE.md for this endpoint but is covered
implicitly by the auth harness; adding it here would be trivial if required.

**CompleteBooking NotConfirmed (F2)**: this scenario is only reachable if the backend
does not auto-confirm bookings. If the test environment uses a synchronous
Stripe sandbox that fires ChargeSucceeded inline with the Charge call, the Booking
may already be Confirmed by the time the hurl script reads it. The test is present
in the `.hurl` regardless; it may produce a false pass (returns 409 because booking
is already completed, not because it's not confirmed) or fail (booking is Confirmed,
not Pending, so Complete succeeds). The narrative captures this ambiguity.

**SlotUnavailable compensation (B1)**: the only compensation scenario testable without
a mock. No state was mutated before the error, so "compensation" here is that HoldSlot
returns immediately with an error and the Calendar is never touched. The compensation
story for B1 is therefore verified by asserting the first booking is still alive after
the second attempt fails — which the hurl script does.
