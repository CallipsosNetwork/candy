# airbnb — full marketplace example

A short-stay rental marketplace: hosts list properties, guests book stays
on time ranges, coupons reduce price, payments settle the booking, and
the platform takes a cut. This is the largest example in the repo — a
multi-feature project with cross-feature event subscriptions, an external
payment provider with multiple providers configured, and a canonical
multi-actor saga (`PlaceBooking`) that demonstrates compensation across
network boundaries.

Wallet is *not* in airbnb — it's its own standalone example. Booking
settles directly through the external `Payments` actor (charge guest,
transfer to host).

## What this exercises

- **`prose` block** in every feature — intent, exports, uses, policies (GRAMMAR.md "prose").
- **Single-file features** (one `.candy` per feature, flat layout) per the project decision.
- **Cross-feature `uses:`** including the `feature X for event Y` form (subscribers).
- **Multi-actor saga** with `compensate` across network boundaries (booking).
- **External actor with multiple providers** (`Payments` with `providers: [Stripe, Polar, Lemon]`) and `Actor[Tag]` selection (GRAMMAR.md "external actor", "Multiple providers").
- **Webhook subscriptions** (`subscribe ChargeSucceeded -> ConfirmBooking`).
- **Policy attachment** at type-, actor-, flow-, controller-, and feature-scope (GRAMMAR.md "Policy attachment").
- **Idempotency keys** on every replayable flow.
- **Branded `Money`** and minor-units arithmetic; conservation invariants.
- **Role-gated controllers** (admin endpoints for coupon management, host endpoints for listings).
- **`schedule` + TIME-axis pattern** — `SendBookingReminder` fires every minute and picks up Confirmed bookings whose start is within 60 seconds, demonstrating the schedule predicate form from GRAMMAR.md alongside the webhook subscribe pattern already exercised by billing.

## Test-mode design choices

### Minute-granular slots instead of day-granular dates

Real Airbnb books by day: a guest selects a check-in date and a check-out date, and the listing's calendar blocks those days. This example books by minute instead. A `TimeRange` is a pair of `Timestamp` values (UTC, minute or finer resolution). A booking might be 5 minutes long, 20 minutes long, or 60 minutes long.

**Why:** eval cycles should complete in seconds, not days. If bookings were day-granular, an eval run that books a listing, waits for the booking to start, and then tests the reminder flow would need to wait 24+ hours. By making every bookable slot a minute, the same eval run can:

1. List a property with `pricePerMinute`.
2. Book it for the next 5–60 minutes.
3. Watch the `SendBookingReminder` schedule fire at `T - 60s`.
4. Confirm the `BookingReminderDue` event was emitted.
5. Cancel or complete the booking.

All within a single minute-long test run.

### Pricing: `pricePerMinute` instead of `pricePerNight`

`Listing.pricePerMinute: Money` replaces the day-granular `pricePerNight`. The booking total is computed as `pricePerMinute * (duration / 60)` where `duration` is the number of seconds between `dates.from` and `dates.to`. This is intentionally simple — it does not pro-rate partial minutes, and Money's `round: nearest` handles the integer truncation.

In production you would rename back to `pricePerNight` and use a day-count multiplier; the spec structure is otherwise identical.

### Cancellation thresholds

The `CancellationPolicy` windows are scaled to seconds for the same reason:

| Window          | Test (this file)         | Production equivalent |
|-----------------|--------------------------|----------------------|
| Free            | > 60s before check-in   | > 48h before         |
| Partial         | 30s – 60s before         | 24h – 48h before     |
| NonRefundable   | ≤ 30s before / started   | ≤ 24h / started      |

### Pre-booking reminder via `schedule`

`SendBookingReminder` fires every 1 minute and emits `BookingReminderDue` for each Confirmed booking whose `dates.from` is within the next 60 seconds and whose `reminder_sent` flag is false. After emitting, it calls `Booking.MarkReminderSent` to prevent duplicate delivery.

`BookingReminderDue` is a feature-local event in `booking.candy`. In a multi-feature project, the notifications feature subscribes to it:

```
uses: feature booking for event BookingReminderDue
```

This keeps reminder logic in booking and delivery logic (Postmark, SendGrid, etc.) in notifications — neither feature depends on the other's internals.

## Project structure

```
examples/airbnb/
  candy.toml          ← manifest
  preferences.candy   ← per-target library bindings (incl. payment providers)
  README.md           ← this file (project scope + symbol contract)

  types.candy         ← shared types (#2)
  events.candy        ← cross-feature events (#3)
  invariants.candy    ← system-wide truths (#4)
  externals.candy     ← Payments external actor (#20)

  auth.candy          ← User, Session, Signup/Login/Logout (#5)
  listings.candy      ← Listing, Calendar, CRUD + HoldSlot/ReleaseSlot (#7)
  coupons.candy       ← Coupon, Validate/Redeem/Restore, admin CRUD (#9)
  booking.candy       ← Booking, PlaceBooking saga, CancelBooking, SendBookingReminder schedule (#8)
```

Conformance evals live at `evals/airbnb/{auth,listings,booking,coupons}.hurl` (#10, #11).

## Symbol contract

This section is the source of truth for cross-feature names. Every spec
file in this project must use these exact identifiers and shapes.

### Shared types (defined in `types.candy`)

```
type Id          opaque  { max: 64 }
type Timestamp   instant { tz: utc }
type TimeRange   { from: Timestamp, to: Timestamp }
type Email       string  { max: 320, format: rfc5322 }
type Password    string  { policies: [PasswordStrength] }
type PasswordHash opaque
type Token       opaque  { max: 256 }
type Key         opaque  { max: 128 }
type Money       int     { unit: minor, currency: USD, round: nearest }
type CouponCode  string  { max: 32 }

type PaymentMethod opaque { max: 256 }
type ChargeId      opaque { max: 128 }
type RefundId      opaque { max: 128 }
type TransferId    opaque { max: 128 }

enum Role           { Guest, Host, Admin }
enum BookingStatus  { Pending, Confirmed, Cancelled, Completed }
enum ListingStatus  { Draft, Listed, Hidden }
enum CouponKind     { Percent, FixedAmount }
```

`Date` and `DateRange` have been removed. All time-ranged fields now use
`TimeRange { from: Timestamp, to: Timestamp }`.

### Shared events (defined in `events.candy`)

All events use `delivery: eager` unless noted; ordering is `by` the most
relevant identifier so subscribers can dedupe.

```
event UserSignedUp    { payload: { user: Id, email: Email, at: Timestamp }, delivery: eager }
event UserVerified    { payload: { user: Id, at: Timestamp },               delivery: eager }
event UserLoggedIn    { payload: { user: Id, at: Timestamp },               delivery: eager, order: by user }
event SessionRevoked  { payload: { token: Token, user: Id, at: Timestamp }, delivery: eager }

event ListingCreated  { payload: { listing: Id, host: Id, at: Timestamp }, delivery: eager, order: by listing }
event ListingUpdated  { payload: { listing: Id, at: Timestamp },           delivery: eager, order: by listing }
event ListingHidden   { payload: { listing: Id, at: Timestamp },           delivery: eager, order: by listing }

event CouponCreated   { payload: { coupon: Id, code: CouponCode, at: Timestamp }, delivery: eager }
event CouponRedeemed  { payload: { coupon: Id, user: Id, booking: Id, at: Timestamp }, delivery: eager, order: by coupon }
event CouponRestored  { payload: { coupon: Id, booking: Id, at: Timestamp }, delivery: eager, order: by coupon }

event BookingPlaced     { payload: { booking: Id, listing: Id, guest: Id, dates: TimeRange, total: Money, at: Timestamp }, delivery: eager, order: by booking }
event BookingConfirmed  { payload: { booking: Id, charge: ChargeId, at: Timestamp }, delivery: eager, order: by booking }
event BookingCancelled  { payload: { booking: Id, reason: string, at: Timestamp }, delivery: eager, order: by booking }
```

**Note for orchestrator:** `events.candy` still declares `BookingPlaced` with
`dates: DateRange`. That file needs a follow-up edit to change `DateRange` →
`TimeRange` in the `BookingPlaced` payload. No other events in `events.candy`
reference `DateRange` or `Date`.

### System-wide invariants (defined in `invariants.candy`)

```
invariant SlotIntegrity:
  "no two Confirmed bookings share a (listing, slot) overlap"

invariant CouponSingleUse:
  "a coupon is redeemed at most once per (coupon, user) pair"

invariant BookingHostMatchesListing:
  "for any Booking b: b.host == Listing(b.listing).host"

invariant SessionUserConsistency:
  "for any active Session s: User(s.user) exists and is verified"
```

`invariants.candy` still uses `(listing, date) tuple` wording. The orchestrator
should update it to `(listing, slot) overlap` to match TimeRange semantics.

### External actor (defined in `externals.candy`)

```candy
external actor Payments {
  providers: [Stripe, Polar, Lemon]

  config Stripe: api_key: secret "STRIPE_KEY"
                 webhook_secret: secret "STRIPE_WEBHOOK_SECRET"
  config Polar:  api_key: secret "POLAR_KEY"
                 webhook_secret: secret "POLAR_WEBHOOK_SECRET"
  config Lemon:  api_key: secret "LEMONSQUEEZY_KEY"
                 webhook_secret: secret "LEMONSQUEEZY_WEBHOOK_SECRET"

  accepts Charge(amount: Money, source: PaymentMethod, key: Key)
    -> Result<ChargeId, PaymentDeclined | NetworkError | RateLimited>

  accepts Refund(charge: ChargeId, amount: Money?, key: Key)
    -> Result<RefundId, RefundError | NetworkError>

  accepts Transfer(to: Id, amount: Money, key: Key)
    -> Result<TransferId, TransferFailed | NetworkError>

  emits ChargeSucceeded { charge: ChargeId, booking: Id, at: Timestamp }
  emits ChargeFailed    { charge: ChargeId, booking: Id, reason: string, at: Timestamp }
  emits RefundProcessed { refund: RefundId, charge: ChargeId, at: Timestamp }
}
```

The `booking` field on `ChargeSucceeded`/`ChargeFailed` is the booking
that initiated the charge — passed via Stripe metadata / Polar custom
fields / LemonSqueezy custom data and surfaced uniformly by codegen.

### Per-feature exports

Every feature's `prose` block declares exactly these exports. Other
features `uses:` them by these names.

| Feature   | Exports                                                                            |
|-----------|------------------------------------------------------------------------------------|
| auth      | `actor User, Session`; `flow Signup, Login, Logout, Verify`; `event UserSignedUp, UserVerified, UserLoggedIn, SessionRevoked` |
| listings  | `actor Listing, Calendar`; `flow CreateListing, UpdateListing, ListListings, HoldSlot, ReleaseSlot`; `event ListingCreated, ListingUpdated, ListingHidden` |
| coupons   | `actor Coupon`; `flow CreateCoupon, DeleteCoupon, ValidateCoupon, RedeemCoupon, RestoreCoupon`; `event CouponCreated, CouponRedeemed, CouponRestored` |
| booking   | `actor Booking`; `flow PlaceBooking, CancelBooking, CompleteBooking, SendBookingReminder`; `event BookingPlaced, BookingConfirmed, BookingCancelled, BookingReminderDue` |

### Cross-feature dependency graph

```
booking  uses  feature listings  for HoldSlot, ReleaseSlot
              feature coupons   for ValidateCoupon, RedeemCoupon, RestoreCoupon
              external Payments for Charge, Refund, Transfer
              external Payments for event ChargeSucceeded, ChargeFailed

listings uses feature auth      for event UserSignedUp     (host upgrade hook — optional)

coupons  uses (nothing — leaf feature)

auth     uses (nothing — leaf feature)
```

### Policies referenced across the project

| Policy                      | Defined in   | Attaches to                              |
|-----------------------------|--------------|------------------------------------------|
| `PasswordStrength`          | auth         | type `Password`                          |
| `BearerAuth`                | auth         | controller (feature-scope on auth)       |
| `RoleGated`                 | auth         | controller routes (per-route)            |
| `AdminGated`                | coupons      | controller routes (admin endpoints)      |
| `CouponEligibility`         | coupons      | flow `RedeemCoupon`                      |
| `CancellationPolicy`        | booking      | flow `CancelBooking`                     |
| `BookingAtomicity`          | booking      | flow `PlaceBooking`                      |

## Per-feature scope

### `auth` — User, Session, signup/login/logout (#5)

Adapted from `examples/auth/auth.candy`. Adds:
- `prose` block with intent/exports/policies.
- `User.role: Role` field (Guest by default; promoted by admin or self-service host upgrade).
- Role-gated controller routes via `RoleGated` policy attached per-route. Admin-only routes for user moderation; host-only checks for listing endpoints (in `listings`).
- All flows accept `key: Key` for idempotency.
- `Session` actor with token, expires, revoked. `Validate(now)` returns user id + role.

### `listings` — Listing, Calendar, hold-and-release (#7)

- `Listing(id)` — host id, status, title, location prose, `pricePerMinute: Money`, max guests.
- `Calendar(listing)` — held-set of `HeldEntry { slot: TimeRange, booking: Id }`. Overlap check uses half-open interval: `entry.slot.from < slot.to and entry.slot.to > slot.from`.
- `HoldSlot(listing, slot: TimeRange, booking: Id, key: Key)` — exported for booking saga; rejects `SlotUnavailable` on overlap. Compensation: `ReleaseSlot`.
- `ReleaseSlot(listing, slot: TimeRange, key: Key)` — idempotent unhold.
- `CreateListing` / `UpdateListing` host-gated; `ListListings(filter, host?, slot?)` public.
- `ListListings` with `AvailableInRange` filter excludes listings with any held slot overlapping the query slot.
- Emits `ListingCreated`, `ListingUpdated`, `ListingHidden`.

### `coupons` — Coupon, eligibility, redeem/restore (#9)

- `Coupon(code: CouponCode)` — kind (Percent | FixedAmount), value, max uses, expires, redemptions journal.
- `CouponEligibility` policy attached at flow-scope on `RedeemCoupon` — prose-heavy with `examples:` covering: not-found, expired, exhausted, already-redeemed-by-user, ok.
- `ValidateCoupon(code, user, now)` — pure check, no state change.
- `RedeemCoupon(code, user, booking, now, key)` — appends to redemptions, emits `CouponRedeemed`. Compensation: `RestoreCoupon`.
- `RestoreCoupon(coupon, booking, key)` — idempotent unredeem on saga rollback.
- Admin-only `CreateCoupon`/`DeleteCoupon`.
- Emits `CouponCreated`, `CouponRedeemed`, `CouponRestored`.

### `booking` — Booking saga, compensate, payments, reminder schedule (#8)

- `Booking(id)` — listing, guest, host, `dates: TimeRange`, status, total, charge?, coupon?, `reminder_sent: bool`.
- `PlaceBooking(listing, guest, dates: TimeRange, source: PaymentMethod, code: CouponCode?, now, key)` — the canonical saga:
  1. `step held = ask listings.HoldSlot(dates, booking_id, key)` rescue reject `SlotUnavailable`.
  2. `step discount = if code? then ask coupons.ValidateCoupon(code, guest, now) else 0`.
  3. `step minutes = (dates.to - dates.from) / 60; step total = listing.pricePerMinute * minutes - discount`.
  4. `step paid = ask Payments[Stripe].Charge(total, source, key)` rescue compensate held; reject `PaymentDeclined`.
  5. `step redeemed = if code? then ask coupons.RedeemCoupon(...) else unit` rescue compensate paid, held.
  6. `commit Booking { status: Pending, reminder_sent: false, ... }`; `emit BookingPlaced`.
  7. `subscribe ChargeSucceeded -> Confirm` (move to Confirmed).
- `CancelBooking` — guarded by `CancellationPolicy` (60s/30s test thresholds), then refund, restore coupon, release slot, set Cancelled.
- `SendBookingReminder` flow + schedule — fires every 1 minute for Confirmed bookings within 60s of `dates.from`; emits `BookingReminderDue`; sets `reminder_sent = true`.
- `BookingAtomicity` flow-scope policy: ensures all-or-nothing across the saga.
- Emits `BookingPlaced`, `BookingConfirmed`, `BookingCancelled`, `BookingReminderDue`.

### `externals` — Payments multi-provider (#20)

See "External actor" in the symbol contract above. The example pins
`Payments[Stripe]` as the default provider in the saga; preferences.candy
binds the per-target SDK for each of `stripe`, `polar`, `lemon`.

## Codegen targets

All four targets supported (Go/chi, Rust/axum, TypeScript/hono,
Python/fastapi). Per-target idiom highlights:

- **Go (chi)** — `sqlc` for queries, `chi` for routing, `stripe-go` for Payments[Stripe].
- **Rust (axum)** — `sqlx` for queries, axum tower layers for policies, `async-stripe` for Payments[Stripe].
- **TypeScript (hono)** — `drizzle` for ORM, hono middleware for policies, `stripe-node` for Payments[Stripe].
- **Python (fastapi)** — `sqlalchemy` for ORM, FastAPI dependencies for policies, `stripe-python` for Payments[Stripe].

## Conformance

Per-feature `.hurl` files under `evals/airbnb/`:

- `auth.hurl` — full coverage including role-gated checks (#10).
- `listings.hurl`, `booking.hurl`, `coupons.hurl` — TODO stubs to be filled (#11).

## Issue tracking

| Issue | File                  | Status      |
|-------|-----------------------|-------------|
| #2    | `types.candy`         | Implementing |
| #3    | `events.candy`        | Implementing |
| #4    | `invariants.candy`    | Implementing |
| #20   | `externals.candy`     | Implementing |
| #5    | `auth.candy`          | Implementing |
| #7    | `listings.candy`      | Implementing |
| #9    | `coupons.candy`       | Implementing |
| #8    | `booking.candy`       | Implementing |
| #10   | `evals/airbnb/auth.hurl` | Pending |
| #11   | other `evals/airbnb/*.hurl` | Pending |
