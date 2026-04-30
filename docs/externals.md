# External actors

An `external actor` is the pattern for integrating SDKs and third-party
services. Same syntax as a regular actor; one modifier flips ownership.

## The unification

Five things you'd otherwise have to invent separately fall out of
treating SDKs as actors:

### Webhooks = events

When Stripe sends a `charge.succeeded` webhook, it's just `external actor
Payments` emitting `ChargeSucceeded`. Internal actors `subscribe` to it
the same way they subscribe to internal events. **One mental model
covers internal events, external events, and webhooks.**

### Compensation works naturally

A saga step `step paid = ask Payments.Charge(...)` failing triggers
`compensate hold; reject`. If a later step fails *after* a successful
charge, the compensator is `ask Payments.Refund(...)`. Same mechanic as
undoing internal state. The flow doesn't care that one step crossed a
network.

### Idempotency is uniform

A `key: Key` parameter on an external `accepts` maps cleanly to Stripe's
`Idempotency-Key` header (or the SDK equivalent). The candy contract
stays the same; codegen wires the right mechanism per target.

### Failure taxonomy is forced

Network errors, rate limits, auth failures become explicit
`Result<Ok, NetworkError | RateLimited | AuthFailed>` variants on every
`accepts` signature. No silent swallow; every call site has to handle
them.

### The spec is the contract

When Stripe's API changes, you update the `external actor` block; codegen
regenerates the wrapper. External dependencies become versionable,
diffable, reviewable like any other piece of the system.

## What's different from an internal actor

External actors **have no `state {}` block**. We don't own their state.
They have:

- **`config:`** — credentials, endpoints, options. Often references
  secrets via `secret "ENV_NAME"`.
- **`accepts`** — outbound calls into the SDK. Each declares input,
  output, and explicit error variants.
- **`emits`** — inbound events (typically webhooks). Subscribers handle
  them.

That's it. The `external` modifier strips the state-ownership story;
everything else stays.

## Example

```candy
external actor Payments {
  intent: "Stripe SDK adapter — charges, refunds, customer setup"

  config:
    api_key:        secret "STRIPE_KEY"
    webhook_secret: secret "STRIPE_WEBHOOK_SECRET"

  accepts Charge(amount: Money, source: PaymentMethod, key: Key)
    -> Result<ChargeId, PaymentDeclined | NetworkError | RateLimited>

  accepts Refund(charge: ChargeId, amount: Money?)
    -> Result<RefundId, RefundError | NetworkError>

  emits ChargeSucceeded { charge: ChargeId, at: Timestamp }
  emits ChargeFailed    { charge: ChargeId, reason: string, at: Timestamp }
  emits RefundProcessed { refund: RefundId, at: Timestamp }
}
```

A flow uses it identically to an internal actor:

```candy
flow PlaceBooking(...) -> ... {
  step hold = ask Slot.Hold(...)             rescue reject SlotUnavailable
  step paid = ask Payments.Charge(amount, source, key)
                                              rescue compensate hold;
                                                     reject PaymentDeclined
  commit Booking.Confirm(...)
  emit BookingPlaced
}
```

A subscriber reacts to webhooks the same way it reacts to internal
events:

```candy
actor Booking(id: Id) {
  ...
  subscribe ChargeFailed -> RevertHold(reason)
}
```

## When to use external actor vs. substrate

**External actor** when you'd be inventing a contract anyway: Stripe,
SendGrid, Twilio, OpenAI, Algolia. Specific SDK calls with specific
shapes. The contract has domain semantics.

**Substrate** when the dependency is universal infrastructure with
idiomatic per-language libraries: Postgres, Redis, BullMQ, S3. Lives in
`preferences.candy`. Never in `spec/`.

The dividing line: substrate has no domain semantics; external actors
do.

## Where they live

In a project, external actors typically live in:

- **`spec/externals.candy`** — shared declarations for SDKs used by
  multiple features (`Payments`, `Email`, `Analytics`).
- **`spec/<feature>/externals.candy`** — feature-specific externals (rare).

Other features `use external Payments for Charge, Refund` in their
`prose.candy`.

## Codegen responsibilities

When AI generates code from an external actor declaration, it produces:

- An SDK client wrapper using the target's idiomatic library (per
  `preferences.candy` — e.g., `stripe-python` for Python, `stripe-go`
  for Go).
- Auth wiring from `config:` (env vars, secret managers).
- Idempotency-key threading on every call.
- Explicit error mapping into the declared `Result` variants — network
  retries / rate-limit backoff / typed errors.
- Webhook handler endpoints (if the actor declares `emits`) that
  validate signatures and dispatch to subscribers.

The candy spec describes the contract; the codegen prompts know how to
honor it per target language.
