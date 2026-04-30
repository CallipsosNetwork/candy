# notifications — event-driven fan-out to email and SMS

A NotificationWorker actor subscribes to internal events
(`UserSignedUp`, `OrderShipped`) and dispatches user-facing
notifications via external providers. Email goes through one of
SendGrid, Mailgun, or Resend; SMS goes through Twilio or Vonage.
Provider selection happens at the call site with `Actor[Tag]`.
Failures retry with exponential backoff and emit
`NotificationFailed` if the retry budget is exhausted.

This is the smallest spec that exercises `subscribe`, multi-provider
external actors, and a fan-out pipeline together. The point is to
show the asynchronous edge of the system — the side where work is
*pulled* by subscribers rather than *pushed* by a controller.

## What this exercises

- **`subscribe` to internal events** — `NotificationWorker`
  subscribes to `UserSignedUp` and `OrderShipped`, both declared as
  `uses:` imports in the prose block (GRAMMAR.md "event",
  "subscribe").
- **`external actor` with `providers:` for Email and SMS** — two
  separate external actors, each with its own provider set
  (GRAMMAR.md "Multiple providers").
- **`Actor[Tag]` selection in flows** — `Email[SendGrid]`,
  `SMS[Twilio]`.
- **Fan-out pattern** — one source event triggers multiple delivery
  attempts across channels.
- **Retry-on-failure via re-emit** — a delivery failure emits a
  delayed retry envelope rather than blocking; subscribers handle
  retries asynchronously.
- **Delivery-status events** — `NotificationSent` and
  `NotificationFailed` for downstream observability.
- **Idempotency** — every `Send` carries a `key: Key` derived from
  the source event id, so re-delivery does not duplicate.

## Domain model

### Types

```
type Id           opaque  { max: 64 }
type Timestamp    instant { tz: utc }
type Key          opaque  { max: 128 }
type EmailAddress string  { max: 320, format: rfc5322 }
type PhoneNumber  string  { max: 20,  format: e164 }
type MessageId    opaque  { max: 128 }    // provider-side id

enum NotificationKind { Welcome, Shipped }
enum Channel          { EmailChannel, SMSChannel }
enum AttemptOutcome   { Delivered, TransientFailure, PermanentFailure }

type DeliveryAttempt {
  attempt:     int
  channel:     Channel
  provider:    string         // "sendgrid" | "twilio" | etc.
  message:     MessageId?
  outcome:     AttemptOutcome
  reason:      string?
  at:          Timestamp
}
```

### Actors

**NotificationWorker(id: Id)** — singleton-ish actor (one instance
per region or shard). Stateful only enough to dedup recently-handled
event keys and track attempt counts. Subscribes:

```
subscribe UserSignedUp -> Send(payload.user, Welcome, EmailChannel, payload.user, now)
subscribe OrderShipped -> Send(payload.user, Shipped, EmailChannel, payload.order, now)
subscribe OrderShipped -> Send(payload.user, Shipped, SMSChannel,   payload.order, now)
```

The `key` argument to `Send` is derived from the source event id so
that any re-delivery of the source event collapses to a single
notification.

State:

```
state {
  recent: [DeliveryAttempt] = []   // bounded ring; drop oldest beyond N
}
```

Invariants: `length(recent) <= 1000` (a soft guard — true persistence
lives in the database; this is a read-through cache for dedup).

### Flows

```
flow Send(recipient: Id, kind: NotificationKind, channel: Channel,
          key: Key, now: Timestamp)
  -> Result<DeliveryAttempt, RecipientUnreachable | NoContactInfo>

flow Retry(prior: DeliveryAttempt, now: Timestamp, key: Key)
  -> Result<DeliveryAttempt, RetryBudgetExhausted>
```

- **`Send`** looks up the recipient's contact info, picks a provider
  via `Actor[Tag]` (default `Email[SendGrid]` or `SMS[Twilio]`),
  calls `SendEmail` or `SendSMS`, and emits `NotificationSent` on
  success. Transient failures emit a delayed retry envelope (handled
  by `Retry` after `30s`, then `2m`, then `10m`).
- **`Retry`** re-attempts with exponential backoff up to a
  configurable `MaxAttempts` (default 3). Terminal failure emits
  `NotificationFailed`.

### Controllers

| Method | Path                          | Target                                       | Auth   | Statuses                          |
|--------|-------------------------------|----------------------------------------------|--------|-----------------------------------|
| POST   | /admin/notifications/test     | Send(recipient, kind, channel, generate(), now) | bearer | 202 / 422                         |
| POST   | /admin/notifications/replay   | Retry(attempt, now, generate())              | bearer | 202 / 409 RetryBudgetExhausted    |
| POST   | /webhooks/email               | (dispatch to Email emits)                    | none   | 200 (signature-checked)           |
| POST   | /webhooks/sms                 | (dispatch to SMS emits)                      | none   | 200 (signature-checked)           |

The two webhook routes are the inbound side of the external actors —
SendGrid/Mailgun/Resend POST delivery-status events; Twilio/Vonage
POST SMS status. Subscribers (here, `NotificationWorker`) react.

The bulk of the work happens via `subscribe`, not via these routes.
Controllers exist for admin testing and replay only.

### Events

Internal events imported via `uses:` (declared in the features that
emit them, not here):

```
event UserSignedUp { payload: { user: Id, email: EmailAddress, at: Timestamp }, delivery: eager }
event OrderShipped { payload: { user: Id, order: Id, at: Timestamp },           delivery: eager }
```

Events emitted by this feature:

```
event NotificationSent   { payload: { recipient: Id, kind: NotificationKind, channel: Channel, provider: string, message: MessageId, at: Timestamp }, delivery: eager }
event NotificationFailed { payload: { recipient: Id, kind: NotificationKind, channel: Channel, attempts: int, reason: string, at: Timestamp }, delivery: strict }
```

`NotificationFailed` is `strict` (exactly-once) because it likely
triggers an alert or compensation upstream.

### Policies

- **DeliveryAtLeastOnce** — feature-scope. Asserts that every source
  event handled by `NotificationWorker` results in either a
  `NotificationSent` or a `NotificationFailed`. No silent drops.

### External dependencies

**`external actor Email` with `providers: [SendGrid, Mailgun, Resend]`.**

```
accepts SendEmail(to: EmailAddress, subject: string, body: string, key: Key)
  -> Result<MessageId, EmailRejected | NetworkError | RateLimited>

emits EmailDelivered { message: MessageId, at: Timestamp }
emits EmailBounced   { message: MessageId, reason: string, at: Timestamp }
```

**`external actor SMS` with `providers: [Twilio, Vonage]`.**

```
accepts SendSMS(to: PhoneNumber, body: string, key: Key)
  -> Result<MessageId, SMSRejected | NetworkError | RateLimited>

emits SMSDelivered { message: MessageId, at: Timestamp }
emits SMSFailed    { message: MessageId, reason: string, at: Timestamp }
```

Per-provider `config:` blocks declare API keys and webhook secrets.

## Codegen targets

All four targets supported. Per-target idioms:

- **Go (chi)** — providers via their official Go SDKs; channels +
  goroutines for the retry pipeline. Webhook routes per provider.
- **Rust (axum)** — provider clients behind a trait; tokio tasks for
  the retry pipeline.
- **TypeScript (hono)** — BullMQ for the retry queue; per-provider
  Node SDKs.
- **Python (fastapi)** — Celery for the retry queue; per-provider
  Python SDKs.

## Conformance

Eval lives at `evals/notifications/notifications.hurl` (tracked by
#28). Cover:

- Happy path: `UserSignedUp` → `NotificationSent` (email).
- Fan-out: `OrderShipped` → both an email and an SMS notification.
- Retry: simulated transient `NetworkError` retries up to budget,
  succeeds on attempt 2, emits `NotificationSent`.
- Permanent failure: `EmailRejected` exhausts retry budget, emits
  `NotificationFailed`.
- Idempotency: re-delivering the same source event with the same
  derived key does not duplicate the notification.
- Webhook integration: a provider POSTs `EmailDelivered` →
  `NotificationWorker` reconciles attempt state.

## Issue tracking

- Implementation: #27
- Eval: #28 (scaffold)

## Status

- [ ] Implementation pending
- [ ] Eval pending
