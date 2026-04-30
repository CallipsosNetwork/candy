# billing — recurring charges, retry-on-decline, and provider failover

A subscription billing example. Customers subscribe to a plan; a
scheduled job per active subscription attempts the monthly charge
against an external payment provider. When a charge declines, the
spec retries after 7 days; if it still fails after 21 days, the
subscription is suspended (with a `SubscriptionSuspended` event) and
the provider's charge is compensated. Multiple providers (Stripe,
Polar, Lemon) are declared abstractly and selected per call.

This example is the TIME-axis demonstration: it is the smallest
self-contained spec that exercises `schedule`, `after`, and `until`
together. Recurring billing is the canonical "deadline-driven async
state machine" — the spec stays declarative while the substrate
handles the wakeup.

## What this exercises

- **`schedule` keyword** — periodic charge job per active
  subscription (GRAMMAR.md TIME axis).
- **`after 7d` for retry delay** — failed charges schedule a retry
  with an explicit delay (GRAMMAR.md "Time").
- **`until` for escalation timeout** — retries continue until 21
  days past the cycle, then escalate to suspension.
- **`external actor Payments` with `providers: [Stripe, Polar,
  Lemon]`** — abstract multi-provider declaration (GRAMMAR.md
  "Multiple providers").
- **`Actor[Tag]` selection in flows** — `Payments[Stripe]` at the
  call site.
- **Subscriber actor** — `Subscription` subscribes to `ChargeFailed`
  webhook events to drive the state machine.
- **Compensation on terminal failure** — suspended subscription
  refunds any partial in-flight authorization.
- **State enum with transitions** — `SubscriptionStatus { Active,
  PastDue, Suspended, Cancelled }`.
- **Audit trail** — every billing cycle appends to an audit on the
  subscription.

## Domain model

### Types

```
type Id        opaque  { max: 64 }
type Timestamp instant { tz: utc }
type Key       opaque  { max: 128 }
type Money     int     { unit: minor, currency: USD, round: nearest }
type PlanId    opaque  { max: 64 }
type ChargeId  opaque  { max: 64 }

enum SubscriptionStatus { Active, PastDue, Suspended, Cancelled }

type BillingCycle {
  cycle:    int          // 0-indexed sequence number
  amount:   Money
  charge:   ChargeId?
  outcome:  CycleOutcome
  at:       Timestamp
}

enum CycleOutcome { Succeeded, Declined, Pending }
```

### Actors

**Customer(id: Id)** — identified by `id`. State carries
`email: Email`, `default_method: PaymentMethod`, and `created`. The
external payment provider holds the source-of-truth payment method;
the candy actor tracks only the reference.

**Subscription(id: Id)** — identified by `id`. State:

```
state {
  customer:    Id
  plan:        PlanId
  amount:      Money
  status:      SubscriptionStatus = Active
  period_end:  Timestamp
  cycles:      [BillingCycle] = []
  created:     Timestamp
}
```

Invariants:

- `period_end > created` (a subscription always covers a future
  window at creation).
- A subscription with `status == Cancelled` accepts no further
  state transitions except `period_end` rollover to terminal.

The Subscription subscribes to webhook events from `Payments`:

```
subscribe ChargeSucceeded -> RecordSuccess(charge)
subscribe ChargeFailed    -> RecordFailure(charge, reason)
```

### Flows

```
flow Subscribe(customer: Id, plan: PlanId, amount: Money, key: Key, now: Timestamp)
  -> Result<Id, InvalidPlan | PaymentMethodMissing>

flow Cancel(subscription: Id, now: Timestamp)
  -> Result<unit, AlreadyCancelled>

flow ChargeCycle(subscription: Id, now: Timestamp, key: Key)
  -> Result<BillingCycle, AlreadyCharged | SubscriptionInactive>

flow RetryCharge(subscription: Id, originalCycle: int, now: Timestamp, key: Key)
  -> Result<BillingCycle, EscalationTimeout>
```

- **`Subscribe`** validates the plan, checks the customer has a
  payment method on file, sets `period_end = now after 30d`, and
  emits `SubscriptionStarted`.
- **`Cancel`** sets `status = Cancelled`, leaves `period_end`
  intact (service runs through end of current period), emits
  `SubscriptionCancelled`.
- **`ChargeCycle`** is the scheduled-every-period flow. Calls
  `Payments[Stripe].Charge(amount, customer.method, key)`. On
  success, append a `Succeeded` cycle, advance `period_end` by 30d,
  emit `SubscriptionCharged`. On decline, append a `Declined` cycle,
  set `status = PastDue`, emit `SubscriptionPastDue`, and schedule
  `RetryCharge` `after 7d`.
- **`RetryCharge`** runs `until` 21d past the original cycle's
  timestamp. On success, restore `status = Active`. On final
  failure, set `status = Suspended`, emit `SubscriptionSuspended`,
  and `compensate` any partial Stripe authorization with
  `Payments.Refund`.

### Schedule

```
schedule ChargeCycle(subscription, now, generate())
  every 30d
  for any subscription in Subscription where status == Active
```

The schedule entry lives in the spec next to the flow. Codegen wires
it to the per-target queue (BullMQ for TS, Celery for Python, sqlc
+ cron for Go, axum + tokio for Rust).

### Controllers

| Method | Path                       | Target                                   | Auth   | Statuses                              |
|--------|----------------------------|------------------------------------------|--------|---------------------------------------|
| POST   | /subscriptions             | Subscribe(self, plan, amount, key, now)  | bearer | 201 / 422                             |
| GET    | /subscriptions/:id         | Subscription(id)                         | bearer | 200 / 404                             |
| DELETE | /subscriptions/:id         | Cancel(id, now)                          | bearer | 204 / 409 AlreadyCancelled            |
| POST   | /webhooks/payments         | (dispatch to Payments emits)             | none   | 200 (signature-checked)               |

The webhook route is the inbound side of the external actor — a
provider POSTs `ChargeSucceeded` / `ChargeFailed` envelopes; the
runtime validates the signature against the per-provider
`webhook_secret` and dispatches to the subscribed actors.

### Events

```
event SubscriptionStarted   { payload: { subscription: Id, customer: Id, plan: PlanId, at: Timestamp }, delivery: eager }
event SubscriptionCharged   { payload: { subscription: Id, cycle: int, amount: Money, at: Timestamp }, delivery: eager }
event SubscriptionPastDue   { payload: { subscription: Id, cycle: int, reason: string, at: Timestamp }, delivery: eager }
event SubscriptionSuspended { payload: { subscription: Id, at: Timestamp }, delivery: strict }
event SubscriptionCancelled { payload: { subscription: Id, at: Timestamp }, delivery: eager }
```

`SubscriptionSuspended` is `strict` (exactly-once) because downstream
consumers (email notification, access revocation) must not double-fire.

### Policies

- **BillingAtomicity** — flow-scope on `ChargeCycle` and
  `RetryCharge`. Asserts that the external charge result and the
  Subscription status transition land together: either both apply or
  the cycle is left in `Pending` for safe retry.

### External dependencies

**`external actor Payments` with `providers: [Stripe, Polar, Lemon]`.**

Per-provider `config:` blocks declare `api_key` and `webhook_secret`
secrets. The accepts/emits surface:

```
accepts Charge(amount: Money, method: PaymentMethod, key: Key)
  -> Result<ChargeId, PaymentDeclined | NetworkError | RateLimited>

accepts Refund(charge: ChargeId, amount: Money?)
  -> Result<RefundId, RefundError | NetworkError>

emits ChargeSucceeded { charge: ChargeId, at: Timestamp }
emits ChargeFailed    { charge: ChargeId, reason: string, at: Timestamp }
emits RefundProcessed { refund: RefundId, at: Timestamp }
```

This example uses `Payments[Stripe]` as the single live provider in
flows; the `providers:` list is declared so that switching to Polar
or Lemon (or running them in parallel for fallback) is a one-token
edit at the call site, not a spec change.

## Codegen targets

All four targets supported. Per-target idioms:

- **Go (chi)** — `stripe-go`; cron via `robfig/cron` or simple ticker
  for `schedule`. Webhook route validates `Stripe-Signature`.
- **Rust (axum)** — `async-stripe`; tokio cron for `schedule`. Webhook
  validation via `stripe::Webhook::construct_event`.
- **TypeScript (hono)** — `stripe-node`; BullMQ recurring jobs for
  `schedule`. Webhook via `stripe.webhooks.constructEvent`.
- **Python (fastapi)** — `stripe`; Celery beat for `schedule`. Webhook
  via `stripe.Webhook.construct_event`.

## Conformance

Eval lives at `evals/billing/billing.hurl` (tracked by #28). Cover:

- Happy path: subscribe → first cycle charges → 30d later, second
  cycle charges → cancel → service continues until period_end.
- Decline + recovery: cycle declines → status PastDue → retry after
  7d succeeds → status returns to Active.
- Escalation: cycle declines → all retries fail → 21d later, status
  becomes Suspended and any in-flight charge is refunded.
- Idempotency: re-firing a `ChargeCycle` with the same `key` returns
  the prior cycle without double-charging.
- Webhook race: out-of-order `ChargeSucceeded` and `ChargeFailed`
  resolve to a deterministic terminal state.

## Issue tracking

- Implementation: #26
- Eval: #28 (scaffold)

## Status

- [ ] Implementation pending
- [ ] Eval pending
