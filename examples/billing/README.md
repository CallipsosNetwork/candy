# billing — auth-integrated subscriptions, Polar-first charging, test-mode 60s plans

A subscription billing example with inlined auth. Users sign up, log in, and
subscribe to plans. A scheduled job per active subscription attempts the charge
against Polar (the canonical provider). When a charge declines, the spec retries
after `plan.retry_delay` seconds; once the `plan.escalation_window` expires or
three consecutive failures accumulate, the subscription is suspended.

This example is the TIME-axis demonstration: three `schedule` blocks drive the
entire billing lifecycle. All three fire `every 1m` — predicates gate actual
work, so production plans at 30d intervals are unaffected. The 1m cadence makes
60s test plans observable in a single schedule tick.

## What this exercises

- **Inlined auth** — `User`, `Session`, `Signup`, `Login`, `Logout` using the
  same JWT + argon2 + SQLite pattern as `examples/auth`.
- **Role-based access** — `enum Role { Admin, Customer }`. Admins manage plans;
  Customers (or Admins) subscribe.
- **`schedule` keyword** — three periodic jobs: `ChargeCycle`, `RetryDue`,
  `EscalateOverdue`, all at `every 1m` (GRAMMAR.md TIME axis).
- **Per-plan configurable thresholds** — `Plan.retry_delay` and
  `Plan.escalation_window` let test plans use 60s/5m and production plans use
  7d/30d without any code change.
- **`external actor Payments` with `providers: [Polar, Stripe, Lemon]`** —
  abstract multi-provider declaration. Polar is first (canonical).
- **`Actor[Tag]` selection in flows** — `Payments[Polar]` at the call site in
  `ChargeCycle`; `ChargeCycleWithFallback` chains Polar → Stripe → Lemon.
- **Subscriber actor** — `Subscription` subscribes to `ChargeSucceeded` and
  `ChargeFailed` webhook events as an async backstop.
- **State enum with transitions** — `SubscriptionStatus { Active, PastDue,
  Suspended, Cancelled }`.
- **Idempotency keys** on every replayable flow — re-firing a schedule with the
  same key returns the prior result without double-charging.

## Why Polar

Polar is developer-friendly: the API is clean, the dashboard is transparent, and
the webhook shape is straightforward. This example ships with `POLAR_KEY` and
`POLAR_WEBHOOK_SECRET` as the canonical secrets. Stripe and Lemon remain in
`providers:` so switching (or running `ChargeCycleWithFallback`) is a one-token
edit at the call site.

## Test mode

Create a plan with short thresholds:

```
POST /admin/plans
{
  "name": "test-60s",
  "amount": 100,
  "interval": 60,
  "retry_delay": 60,
  "escalation_window": 300,
  "key": "..."
}
```

With `interval = 60` and the `every 1m` schedule cadence, a full lifecycle
(subscribe → charge → decline → retry → escalate → suspend) completes in under
two minutes during evaluation runs. Production plans at `interval = 2592000`
(30d) are unaffected.

## Domain model

### Auth types and actors

```
type Email        string  { max: 320, format: rfc5322 }
type Password     string  { intent: "plaintext only; only PasswordHash persists" }
type PasswordHash opaque
type Token        opaque  { max: 256 }

enum Role { Admin, Customer }

actor User(id: Id)     — email, hash, role (default Customer), created, verified
actor Session(token)   — user, issued, expires, revoked
```

Signup creates a `Customer`-role user. Admin users are provisioned out-of-band.
Tokens are JWTs: signed with a server secret, payload carries `user_id` + `role`,
`exp = issued + 7d`. Revocation uses a small revoked-tokens table; no
session-store lookup on the hot path.

### Billing types and actors

```
type Money         int     { unit: minor, currency: USD, round: nearest }
type PaymentMethod opaque  { max: 256 }
type ChargeId      opaque  { max: 64 }
type Duration      int     { unit: seconds }

enum SubscriptionStatus { Active, PastDue, Suspended, Cancelled }
```

**Plan(id: Id)** — admin-managed. Key fields:

| field               | production default | test value |
|---------------------|--------------------|------------|
| `interval`          | 2592000 (30d)      | 60         |
| `retry_delay`       | 604800 (7d)        | 60         |
| `escalation_window` | 2592000 (30d)      | 300 (5m)   |

**Subscription(id: Id)** — billing state machine for one customer on one plan.
`customer` is a `User` id. Tracks `status`, `source` (payment method),
`next_charge_date`, `last_failed_at`, `first_failed_at`, `attempts`.

Subscription state machine:

```
Active   -- charge fails   --> PastDue
PastDue  -- retry succeeds --> Active
PastDue  -- escalated      --> Suspended
Active   -- cancel         --> Cancelled
PastDue  -- cancel         --> Cancelled
Suspended -- reactivate   --> Active   (requires fresh payment method)
```

Cancelled is terminal.

### Flows

```
// Auth
flow Signup(email, password, now, key)   -> Result<{ user, token }, WeakPassword | EmailTaken>
flow Login(email, password, now)         -> Result<{ user, token }, InvalidCredentials>
flow Logout(token, now)                  -> unit

// Plan management (admin)
flow CreatePlan(name, amount, interval, retry_delay, escalation_window, now, key)
  -> Result<{ plan }, InvalidPlan>
flow DeletePlan(plan, now, key)          -> Result<unit, PlanNotFound | PlanInUse>

// Subscription lifecycle
flow Subscribe(customer, plan, source, now, key)
  -> Result<{ subscription }, InvalidPlan | InvalidPaymentMethod>
flow CancelSubscription(subscription, now, key)
  -> Result<unit, SubscriptionNotFound | AlreadyCancelled>
flow Reactivate(subscription, source, now, key)
  -> Result<unit, SubscriptionNotFound | NotSuspended | InvalidPaymentMethod>

// Charge engine
flow ChargeCycle(subscription, now, key)
  -> Result<ChargeId, ChargeFailed | SubscriptionNotFound | PlanNotFound>
flow ChargeCycleWithFallback(subscription, now, key)
  -> Result<ChargeId, ChargeFailed | SubscriptionNotFound | PlanNotFound | AllProvidersFailed>
flow RetryDue(subscription, now, key)
  -> Result<ChargeId, ChargeFailed | SubscriptionNotFound | PlanNotFound>
flow EscalateOverdue(subscription, now)
  -> Result<unit, SubscriptionNotFound | AlreadySuspended | AlreadyCancelled>
```

### Schedules

All three fire `every 1m`. The predicate is the gate.

```
schedule ChargeCycle(subscription, now, generate())
  every 1m
  for any subscription in Subscription
  where status == Active and next_charge_date <= now

schedule RetryDue(subscription, now, generate())
  every 1m
  for any subscription in Subscription
  where status == PastDue
    and last_failed_at + subscription.plan.retry_delay <= now
    and attempts < 3

schedule EscalateOverdue(subscription, now)
  every 1m
  for any subscription in Subscription
  where status == PastDue
    and (attempts >= 3 or first_failed_at + subscription.plan.escalation_window <= now)
```

### Controllers

| Method | Path                        | Target                    | Auth          | Notes                      |
|--------|-----------------------------|---------------------------|---------------|----------------------------|
| POST   | /signup                     | Signup                    | none          |                            |
| POST   | /login                      | Login                     | none          |                            |
| POST   | /logout                     | Logout                    | bearer        |                            |
| POST   | /subscriptions              | Subscribe                 | bearer        | self = authenticated user  |
| GET    | /subscriptions/:id          | Subscription(id)          | bearer        |                            |
| POST   | /subscriptions/:id/cancel   | CancelSubscription        | bearer        |                            |
| POST   | /subscriptions/:id/reactivate | Reactivate              | bearer        |                            |
| POST   | /admin/plans                | CreatePlan                | bearer+admin  | AdminGated policy          |
| DELETE | /admin/plans/:id            | DeletePlan                | bearer+admin  | AdminGated policy          |

### Events

```
event UserSignedUp          { delivery: eager }
event UserLoggedIn          { delivery: eager, order: by user }
event SessionRevoked        { delivery: eager }
event SubscriptionCreated   { delivery: eager, order: by subscription }
event ChargeSucceeded       { delivery: eager, order: by subscription }
event ChargeFailed          { delivery: eager, order: by subscription }
event SubscriptionSuspended { delivery: strict, order: by subscription }
event SubscriptionCancelled { delivery: eager, order: by subscription }
event SubscriptionReactivated { delivery: eager, order: by subscription }
```

`SubscriptionSuspended` is `strict` (exactly-once) because downstream consumers
(email, access revocation) must not double-fire.

### Policies

- **PasswordStrength** — argon2id hashing, length >= 12, blocklist check.
- **BillingAtomicity** — charge result and subscription state transition land
  together; idempotency key prevents double-charge on replay.
- **BillingFrequency** — `ChargeCycle` skips subscriptions not yet due, already
  charged this period, or not in Active status.
- **AdminGated** — plan management routes require `role == Admin`.

### External dependencies

**`external actor Payments` with `providers: [Polar, Stripe, Lemon]`.**

Polar is the canonical provider. `ChargeCycle` uses `Payments[Polar]` directly.
`ChargeCycleWithFallback` chains Polar → Stripe → Lemon before giving up.

```
accepts Charge(amount: Money, source: PaymentMethod, key: Key)
  -> Result<ChargeId, PaymentDeclined | NetworkError | RateLimited>

accepts Refund(charge: ChargeId, amount: Money?, key: Key)
  -> Result<RefundId, RefundError | NetworkError>

emits ChargeSucceeded { charge: ChargeId, subscription: Id, at: Timestamp }
emits ChargeFailed    { charge: ChargeId, subscription: Id, reason: string, at: Timestamp }
emits RefundProcessed { refund: RefundId, charge: ChargeId, at: Timestamp }
```

The webhook route is the inbound side of the external actor — Polar POSTs
`ChargeSucceeded` / `ChargeFailed` envelopes; the runtime validates the signature
against `POLAR_WEBHOOK_SECRET` and dispatches to subscribed actors.

## Codegen targets

All four targets supported. Per-target idioms (see `preferences.candy`):

- **Go (chi)** — `polar-go` for payments; `golang-jwt` + `argon2` for auth;
  `sqlc` for DB; cron via `robfig/cron` for `schedule`.
- **Rust (axum)** — `polar-rust`; `jsonwebtoken` + `argon2`; `sqlx`; tokio cron.
- **TypeScript (hono)** — `polarsh-sdk`; `jsonwebtoken` + `argon2`; `drizzle`;
  BullMQ recurring jobs.
- **Python (fastapi)** — `polar-python`; `pyjwt` + `argon2-cffi`; `sqlalchemy`;
  Celery beat.

## Status

- [ ] Implementation pending
- [ ] Eval pending
