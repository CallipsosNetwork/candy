# airbnb — full marketplace example

A short-stay rental marketplace: hosts list properties, guests book stays
on date ranges, coupons reduce price, payments settle the booking, and
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
  listings.candy      ← Listing, Calendar, CRUD + HoldDates/ReleaseDates (#7)
  coupons.candy       ← Coupon, Validate/Redeem/Restore, admin CRUD (#9)
  booking.candy       ← Booking, PlaceBooking saga, CancelBooking (#8)
```

Conformance evals live at `evals/airbnb/{auth,listings,booking,coupons}.hurl` (#10, #11).

## Symbol contract

This section is the source of truth for cross-feature names. Every spec
file in this project must use these exact identifiers and shapes.

### Shared types (defined in `types.candy`)

```
type Id          opaque  { max: 64 }
type Timestamp   instant { tz: utc }
type Date        instant { tz: utc, precision: day }
type DateRange   { from: Date, to: Date }
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

event BookingPlaced     { payload: { booking: Id, listing: Id, guest: Id, dates: DateRange, total: Money, at: Timestamp }, delivery: eager, order: by booking }
event BookingConfirmed  { payload: { booking: Id, charge: ChargeId, at: Timestamp }, delivery: eager, order: by booking }
event BookingCancelled  { payload: { booking: Id, reason: string, at: Timestamp }, delivery: eager, order: by booking }
```

### System-wide invariants (defined in `invariants.candy`)

```
invariant SlotIntegrity:
  "no two Confirmed bookings share a (listing, date) tuple"

invariant CouponSingleUse:
  "a coupon is redeemed at most once per (coupon, user) pair"

invariant BookingHostMatchesListing:
  "for any Booking b: b.host == Listing(b.listing).host"

invariant SessionUserConsistency:
  "for any active Session s: User(s.user) exists and is verified"
```

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
| listings  | `actor Listing, Calendar`; `flow CreateListing, UpdateListing, ListListings, HoldDates, ReleaseDates`; `event ListingCreated, ListingUpdated, ListingHidden` |
| coupons   | `actor Coupon`; `flow CreateCoupon, DeleteCoupon, ValidateCoupon, RedeemCoupon, RestoreCoupon`; `event CouponCreated, CouponRedeemed, CouponRestored` |
| booking   | `actor Booking`; `flow PlaceBooking, CancelBooking, CompleteBooking`; `event BookingPlaced, BookingConfirmed, BookingCancelled` |

### Cross-feature dependency graph

```
booking  uses  feature listings  for HoldDates, ReleaseDates
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

- `Listing(id)` — host id, status, title, location prose, base price (Money), max guests, calendar (derived from Calendar actor).
- `Calendar(listing)` — sparse map of Date → BookingId. State is the held-set.
- `HoldDates(listing, dates: DateRange, booking: Id, key: Key)` — exported for booking saga; rejects `DatesUnavailable` on overlap. Compensation: `ReleaseDates`.
- `ReleaseDates(listing, dates: DateRange, key: Key)` — idempotent unhold.
- `CreateListing` / `UpdateListing` host-gated; `ListListings(filter)` public.
- Emits `ListingCreated`, `ListingUpdated`, `ListingHidden`.

### `coupons` — Coupon, eligibility, redeem/restore (#9)

- `Coupon(code: CouponCode)` — kind (Percent | FixedAmount), value, max uses, expires, redemptions journal.
- `CouponEligibility` policy attached at flow-scope on `RedeemCoupon` — prose-heavy with `examples:` covering: not-found, expired, exhausted, already-redeemed-by-user, ok.
- `ValidateCoupon(code, user, now)` — pure check, no state change.
- `RedeemCoupon(code, user, booking, now, key)` — appends to redemptions, emits `CouponRedeemed`. Compensation: `RestoreCoupon`.
- `RestoreCoupon(coupon, booking, key)` — idempotent unredeem on saga rollback.
- Admin-only `CreateCoupon`/`DeleteCoupon`.
- Emits `CouponCreated`, `CouponRedeemed`, `CouponRestored`.

### `booking` — Booking saga, compensate, payments (#8)

- `Booking(id)` — listing, guest, host, dates, status, total, charge?, coupon?
- `PlaceBooking(listing, guest, dates, source: PaymentMethod, code: CouponCode?, now, key)` — the canonical saga:
  1. `step held = ask Listing(listing).HoldDates(dates, self.id, key)` rescue reject `DatesUnavailable`.
  2. `step discount = if code? then ask coupons.ValidateCoupon(code, guest, now) else 0`.
  3. `step total = listing.price * dates.nights - discount`.
  4. `step paid = ask Payments[Stripe].Charge(total, source, key)` rescue compensate held; reject `PaymentDeclined`. (Caller can swap `[Stripe]` → `[Polar]` or `[Lemon]`; the canonical example pins Stripe; a fallback variant `PlaceBookingWithFallback` is included to demonstrate the rescue chain.)
  5. `step redeemed = if code? then ask coupons.RedeemCoupon(code, guest, self.id, now, key) else unit` rescue compensate paid (refund), held; reject `CouponConflict`.
  6. `commit Booking { status: Pending, charge: paid, total, ... }`; `emit BookingPlaced`.
  7. `subscribe ChargeSucceeded(self.charge) -> ConfirmBooking` (move to Confirmed).
- `CancelBooking(booking, reason, now, key)` — guarded by `CancellationPolicy` (refund eligibility window), then refund, restore coupon, release dates, set Cancelled.
- `BookingAtomicity` flow-scope policy: ensures all-or-nothing across the saga.
- Emits `BookingPlaced`, `BookingConfirmed`, `BookingCancelled`.

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
