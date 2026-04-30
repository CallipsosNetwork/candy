# candy — grammar reference

candy is a specification language for stateful backends. You describe a
system as **actors with state**, **flows that compose actors**, **controllers
that expose flows over HTTP**, **policies that capture rules**, and **events
that propagate**. From one spec, AI generates idiomatic backends in Go, Rust,
TypeScript, or Python.

The language is small (~50 single-word keywords), prose-heavy where prose
serves it, rigorous where ambiguity costs.

Files use the `.candy` extension. The language is "candy".

---

## The five word-axes

Every keyword belongs to one of five families. Learn the families and the
words in each, and you can read any candy file.

```
ENTITY      things that exist
            actor  external  state  config  providers  enum  type
            derive  journal  audit  self  id
            flow  controller  event  policy  target  prose

ACTION      things that happen
            ask  tell  emit  emits  effect  commit  compensate  reject
            step  accepts  subscribe  use  uses  exports

TIME        when, in what order, for how long
            now  then  after  before  until  expire  schedule  at  rescue

CONDITION   under what circumstances
            if  else  when  require  invariant  given  unless  where  any  in  need

INTENT      why this exists, what good looks like
            intent  examples  because
```

`flow`, `controller`, `event`, `policy` are block-defining ENTITY keywords —
they declare a thing in the system. `policy` is also an INTENT primitive (it
holds the why) but lives once in the table under ENTITY.

---

## Hard rules

1. **No underscores in keywords.** Compounds must find a single word or
   compose two real ones. Underscores in keywords are drift.
2. **Prose has a place.** `intent:` and `examples:` are first-class fields on
   every block. Free-form English never floats outside a recognized field.
3. **One source of truth.** If a value can be derived, use `derive`. Never
   store what you can compute.
4. **No floats for money.** Money is integer minor units; currency is pinned
   in the type declaration.
5. **Time is UTC; `now` is an input.** Actors and flows receive `now` as a
   parameter. Never call a global clock.
6. **Idempotency keys are explicit.** Replayable messages declare a
   `key: Key` parameter. Replay returns the prior result; effects do not run
   twice.
7. **One actor owns its state.** No other actor reads or writes another
   actor's state directly. Cross-actor mutation goes through a `flow`.

---

## Block types at a glance

| Block             | Purpose                                                  |
|-------------------|----------------------------------------------------------|
| `actor`           | A stateful entity with identity, state, and messages.    |
| `external actor`  | An SDK adapter — same shape as actor, no `state`.        |
| `flow`            | A multi-actor saga with explicit compensation.           |
| `controller`      | HTTP surface — routes, auth, request/response shape.     |
| `policy`          | A rule cluster expressed in prose with examples.         |
| `event`           | A typed message broadcast to subscribers.                |
| `type`            | A record, or a branded primitive with pinned semantics.  |
| `enum`            | A sum (variant) type.                                    |
| `invariant`       | A truth that must hold (actor-local or system-wide).     |
| `prose`           | Feature interface — intent, exports, uses, policies.     |
| `target`          | Per-target library and idiom preferences (preferences.candy). |

---

## actor

A stateful entity with identity. Holds private state, accepts messages,
declares invariants. State is private; no other actor reads it directly.

```candy
actor Name(id: IdType) {
  state {
    field: Type = default
    ...
  }

  derive computed = expression
  invariant <prose-or-predicate>
  audit name { ... }                  // optional append-only history

  accepts MessageName(arg: Type, ...) -> Result<Ok, Err> {
    intent: "What this message does and why."
    require <precondition>  rescue reject <ErrVariant>
    step    name = expression           // optional local binding
    effect: <state mutation>
    emit    EventName { ...payload }
    commit  <success value>             // 'commit' alone returns unit
  }
}
```

Every actor type has these implicit messages, no declaration needed:

- `Type.create(initial_state) -> Type` — create a new instance.
- `Type.findBy(field: value) -> Result<Type, NotFound>` — look up by field.
- `Type(id)` — address an existing instance by id.

Inside an actor, `self` refers to the current instance.

---

## external actor

Same syntax as `actor`, with one modifier that flips ownership: we don't own
the SDK's state. Used for third-party services with explicit messages —
Stripe, SendGrid, Twilio, OpenAI.

External actors have no `state {}` block. They have:

- **`config:`** — credentials, endpoints, options. References env vars via
  `secret "ENV_NAME"`.
- **`accepts`** — outbound calls into the SDK. Each declares input, output,
  and explicit error variants.
- **`emits`** — inbound events (typically webhooks). Subscribers handle
  them just like internal events.

```candy
external actor Payments {
  intent: "Payment provider — charges, refunds, intents."

  config:
    api_key:        secret "PAYMENTS_KEY"
    webhook_secret: secret "PAYMENTS_WEBHOOK_SECRET"

  accepts Charge(amount: Money, source: PaymentMethod, key: Key)
    -> Result<ChargeId, PaymentDeclined | NetworkError | RateLimited>

  accepts Refund(charge: ChargeId, amount: Money?)
    -> Result<RefundId, RefundError | NetworkError>

  emits ChargeSucceeded { charge: ChargeId, at: Timestamp }
  emits ChargeFailed    { charge: ChargeId, reason: string, at: Timestamp }
}
```

### Calling — sync return

A flow uses `ask` to invoke and await the response. The return type is the
`Ok` of the declared `Result`.

```candy
flow PlaceBooking(amount: Money, source: PaymentMethod, key: Key) -> ... {
  step paid = ask Payments.Charge(amount, source, key)
              rescue reject PaymentDeclined          // paid: ChargeId
  ...
}
```

`tell` is the fire-and-forget variant — returns unit, doesn't await:

```candy
flow LogAnalytics(event: AnalyticsEvent) -> unit {
  tell Analytics.Track(event)
  commit
}
```

### Webhook return — async via subscribe

When the SDK confirms asynchronously (e.g. Stripe's `charge.succeeded`
webhook), the external `emits` an event and subscribers react:

```candy
actor Booking(id: Id) {
  ...
  subscribe ChargeSucceeded -> ConfirmBooking(charge)
  subscribe ChargeFailed    -> RevertHold(reason)
}
```

The subscriber doesn't know which provider fired the event; the contract is
the event shape on the external actor.

Webhook routes are **codegen-derived** — when an external actor declares
`emits`, the codegen produces the inbound HTTP handler that validates the
provider's signature and dispatches to subscribers. The spec does not
declare controller routes for webhooks.

### Single provider, swappable

When you'll only ever use one provider but want to swap by editing one
line, keep the external actor abstract. The contract is universal; the
concrete SDK is chosen per target via `preferences.candy`:

```candy
// preferences.candy
target go         { when need payments use stripe-go }
target rust       { when need payments use polar-rust }
target typescript { when need payments use lemonsqueezy-node }
```

Switching providers is a one-line preference edit; no spec change. Codegen
maps each provider's native webhook shapes onto the declared `emits`.

### Multiple providers

When you need more than one provider live at the same time — fallback
chains, per-user routing, per-region selection — declare them explicitly
via `providers:` and select at the call site with `Actor[Tag]`:

```candy
external actor Payments {
  providers: [Stripe, Polar, Lemon]

  config Stripe: api_key: secret "STRIPE_KEY"
  config Polar:  api_key: secret "POLAR_KEY"
  config Lemon:  api_key: secret "LEMONSQUEEZY_KEY"

  accepts Charge(amount: Money, source: PaymentMethod, key: Key)
    -> Result<ChargeId, PaymentDeclined | NetworkError | RateLimited>

  emits ChargeSucceeded { charge: ChargeId, at: Timestamp }
  emits ChargeFailed    { charge: ChargeId, reason: string, at: Timestamp }
}
```

Selection at the call site uses `Actor[Tag]`, parallel to `Actor(id)` for
internal actors:

```candy
flow ChargeWithFallback(amount: Money, source: PaymentMethod, key: Key)
  -> Result<ChargeId, AllProvidersFailed>
{
  step paid = ask Payments[Stripe].Charge(amount, source, key)
              rescue ask Payments[Polar].Charge(amount, source, key)
              rescue ask Payments[Lemon].Charge(amount, source, key)
              rescue reject AllProvidersFailed
  commit paid
}
```

Each provider tag becomes a separate library binding in `preferences.candy`
(lowercased provider name = preferences key):

```candy
target go {
  when need stripe use stripe-go
  when need polar  use polar-go
  when need lemon  use lemonsqueezy-go
}
```

Codegen reads `providers:` from the spec, then looks each tag up in
preferences.

---

## flow

A cross-actor saga. Steps execute in order. On rejection, prior named steps
compensate in reverse.

```candy
flow Name(arg: Type, ..., now: Timestamp, key: Key) -> Result<Ok, Err> {
  intent: """Multi-line prose describing purpose."""

  step name = ask Actor(id).Message(args)
              rescue compensate prior; reject ErrorCase

  step name = if condition then expression           // no else => unit on false

  commit value
  emit  EventName { ...payload }
}
```

- `ask` sends a message and awaits the result; may reject.
- `tell` sends a message and does not await (fire-and-forget).
- `step name = ...` binds the result of an action for later reference.
- `rescue` introduces a failure handler for the immediately-prior action.
- `compensate <step>` undoes a previously-committed named step (by re-asking
  its compensating message; the actor is responsible for declaring it).
- `commit <value>` ends the flow with a success value. Bare `commit` returns
  unit.

### Scheduled flows

A `schedule` declaration runs a flow periodically or at a specific time. It
lives at the top level of a feature, alongside `flow`. Two forms:

**Recurring** — run every `<duration>`, optionally for each instance
matching a predicate:

```candy
schedule ChargeCycle(now)
  every 30d
  for any subscription in Subscription where status == Active
```

The flow receives the matched instance (via the loop variable) and `now` as
inputs. Predicates use the same `where` syntax as derive/lookup.

**One-shot** — run once at a specific timestamp:

```candy
schedule SendReminder(user.id, now)
  at user.created after 24h
  for any user in User where verified == false
```

Schedules emit `ScheduleFired` events for observability. Failures rescheduling
follow the flow's own `rescue` semantics — there is no implicit retry at the
schedule layer.

---

## controller

HTTP surface. Thin: routes, auth, body shape, response mapping. No logic.

```candy
controller Name {
  METHOD /path -> Target {
    auth: none | bearer | basic
    body: { field: Type, ... }
    map:
      ok(value)    -> StatusCode ResponseShape
      err(Variant) -> StatusCode { ... }
  }
}
```

Conventions inside a controller:

- Path params, body fields, and headers are in scope by name.
- `now` is implicitly available.
- `auth: bearer` implicitly binds `self` to the authenticated principal id
  and `bearer` to the raw token string.
- `Target` is one of: a flow invocation `MyFlow(args)`, an actor message
  `MyActor(id).Message(args)`, or a state read `MyActor(id).field`.

---

## policy

A named rule cluster. Prose-first. Examples are first-class and double as
conformance. Invoked like a function: `step ok = MyPolicy(input) rescue
reject ...`.

```candy
policy Name {
  intent: """The rule, in prose."""
  examples:
    - given: <input>
      then:  ok(<value>)
    - given: <input>
      then:  err(<Variant>)
}
```

---

## prose

A feature's interface block. Declares what the slice does, what it exports,
what it depends on, and which policies it carries. Lives at the top of a
single-file feature or in `prose.candy` of a folder feature. Reading the
`prose` block tells a developer everything they need before reading the
implementation.

```candy
prose {
  intent: """
    Account signup, login, logout. Issues sessions; verifies credentials.
  """

  exports:
    actor User, Session
    flow  Signup, Login, Logout
    type  Email, Password
    event UserSignedUp, UserVerified

  uses:
    feature  Wallet   for Topup, Debit
    feature  Auth     for event UserSignedUp
    external Payments for Charge, Refund

  policies: [BearerAuth, RateLimit]
}
```

Fields:

- **`intent:`** — what + why. Conventionally always present.
- **`exports:`** — public API. Anything not exported is private; codegen
  refuses cross-feature references to private items.
- **`uses:`** — cross-feature and external dependencies. Three forms:
  - `feature X for OpName` — calls a flow or actor message exported by feature X.
  - `feature X for event EventName` — subscribes to an event emitted by feature X.
  - `external X for OpName` — calls or subscribes to an external actor.

  Makes the dependency graph grep-able.
- **`policies:`** — feature-scoped policies that apply to every controller
  and flow inside the feature.

All four sections are optional; `intent:` is conventionally always present.

---

## Policy attachment

A `policy` block defines a rule. The `policies:` field declares *where* the
rule is enforced. Five valid attachment points, increasing in scope:

| Scope        | Where `policies:` appears                                                |
|--------------|--------------------------------------------------------------------------|
| Type         | Inside a `type` block. Every value of the type satisfies the policy at construction. |
| Actor        | Inside an `actor` block. Wraps every `accepts` on the actor.             |
| Flow         | Inside a `flow` block. Governs the whole saga.                           |
| Controller   | Inside a `controller` block (and per-route). Runs before route dispatch. |
| Feature      | Inside a `prose` block. Applies to every controller and flow in the feature. |

```candy
type Password string {
  policies: [PasswordStrength]                   // type-scope
}

actor User(id: Id) {
  policies: [AuditLog]                           // actor-scope
  ...
}

flow PlaceBooking(...) -> ... {
  policies: [TransactionalAtomicity, RateLimit]  // flow-scope
  ...
}

controller Bookings {
  policies: [BearerAuth]                         // controller-scope
  POST /bookings -> PlaceBooking(...) {
    policies: [AntiSpam]                         // route-scope
    ...
  }
}
```

Explicit attachment makes the policy graph grep-able: every enforcement
point is visible in source, with no implicit coupling.

---

## event

A typed message broadcast to subscribers.

```candy
event Name {
  payload:  { field: Type, ... }
  delivery: eager | strict | weak
  order:    by Field | total | causal
}
```

- `delivery: eager` — at-least-once (may duplicate).
- `delivery: strict` — exactly-once.
- `delivery: weak`   — at-most-once.

Actors subscribe via `subscribe EventName -> action` inside their block.

---

## type / enum

`type` declares either a **branded primitive** (a name plus an underlying
primitive shape and meta-fields) or a **record** (named user fields). The
position of the underlying primitive disambiguates: an identifier between
the type name and the body marks a primitive; no identifier marks a record.

Underlying primitives are a fixed set: `int`, `string`, `opaque`, `bool`,
`bytes`, `instant`, `decimal`. The body holds meta-fields that pin
semantics (`unit`, `currency`, `round`, `max`, `format`, `tz`, ...).

```candy
type Money     int     { unit: minor, currency: USD, round: nearest }
type Timestamp instant { tz: utc }
type Key       opaque  { max: 128 }
type Email     string  { max: 320, format: rfc5322 }
type Flag      bool

enum Status { Pending, Confirmed, Cancelled }

type LineItem { slot: SlotId, price: Money }    // record — no underlying primitive
```

Type composition:

- `[T]` — list of `T`.
- `T?` or `Option<T>` — optional.
- `Result<Ok, Err>` — success or failure (where `Err` is one variant or a
  union `A | B | C`).
- `unit` — built-in unit type (no value).

---

## invariant

A truth that must always hold.

```candy
// actor-local: declared inside an actor block
invariant balance >= 0

// system-wide: declared at file top-level
invariant SlotIntegrity:
  "no two Confirmed bookings share a slot id"
```

Predicates are checked at runtime; prose invariants are enforced by
conformance tests.

---

## target

Per-target library and idiom preferences. Lives in `preferences.candy` at the
project root. AI codegen consumes these as hints, not requirements.

```candy
target typescript {
  notes: "ESM only; tagged unions over class hierarchies"
  when need queue use bullmq
  when need id    use cuid2
  when need db    use drizzle
  when need hash  use argon2
}

target python {
  when need queue use celery
  when need id    use ulid
  when need db    use sqlalchemy
}
```

A `target` block contains:

- `notes:` — free-form prose for stylistic preferences that don't fit a
  `when need … use …` sentence.
- `when need <concept> use <library>` — preference sentences. Both
  `<concept>` and `<library>` are user-chosen identifiers; the language
  does not enumerate them. AI uses them as hints when the concept arises
  during generation.

The target name (`typescript`, `python`, `go`, `rust`) must match a target
declared in `candy.toml`.

---

## Cross-cutting conventions

**Idempotency.** Any replayable message accepts `key: Key`. The same key
returns the prior result without re-applying effects.

**Time.** `now: Timestamp` is passed in. Durations are written `7d`, `10m`,
`1h`, `30s`, `500ms`. Arithmetic: `now after 7d`, `expires before now`.

**Identity.** `self` is the current actor instance. `Actor(id)` addresses a
specific instance.

**Errors.** Each flow/message declares its error variants explicitly.
`Result<Ok, A | B | C>` is a sum of variants. Pattern: `err(VariantName
field)` destructures variant payload.

**Reserved primitives.** Built-in functions: `generate()` (new id),
`hash(value)`, `verify(value, hash)`, `sum(list)`, `length(list)`, `last(list)`.
`sum`, `length`, `last` work on lists of any numeric primitive, including
branded `int`/`decimal` types like `Money`. Lists support `where <predicate>`,
`+` (append), `[id]` (index by id field).

**Comments.** `// line comment`. No block comments — paragraphs go in
`intent:` blocks.

**String interpolation.** `"Hello, ${name}!"`.

---

## Project layout

A candy project has a `candy.toml` manifest at the root and `.candy` files
beneath. Declarations resolve across files; there is no import statement.
Per-target library preferences live in `preferences.candy` at the project
root. Convention for non-trivial projects:

```
project/
  candy.toml                // manifest: project, targets, deps
  preferences.candy          // per-target library and idiom preferences
  spec/
    types.candy              // shared types and enums
    events.candy             // shared event declarations
    invariants.candy         // system-level invariants
    <feature>/
      actors.candy           // actors for this feature
      flows.candy            // flows for this feature
      controllers.candy      // HTTP surface for this feature
      policies.candy         // rule clusters for this feature
  conformance/<feature>.hurl
  targets/<lang>/           // generated code (output)
```

Small projects flatten to a single `.candy` file. The `examples/` directory
in this repository contains tutorial-scale specs; `examples/airbnb/` is a
full project.

## Feature layout detection

Each feature picks its layout independently. The toolchain detects which by
filesystem alone — no config required:

| Filesystem state                  | Layout                |
|-----------------------------------|-----------------------|
| `spec/<name>/prose.candy` exists  | Folder feature        |
| `spec/<name>.candy` exists        | Single-file feature   |
| Both exist                        | Error (ambiguous)     |
| Neither exists                    | Not a feature         |

The feature name is the directory name (folder layout) or the filename
without `.candy` (single-file). The `prose` block is the entry point in
either case — top of `<name>.candy` for single-file, contents of
`prose.candy` for folder.

Convert single-file → folder mechanically: extract each block into the
matching sibling file (`actors.candy`, `flows.candy`, etc.), leave the
`prose` block in `prose.candy`.
