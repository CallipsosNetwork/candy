# Features

A feature is a coherent slice of the system — actors, flows, controllers,
policies, events that belong together. The airbnb example has four:
auth, listings, booking, coupons. (Wallet lives as its own standalone
example.)

## What a feature contains

- **Intent** — what this slice does and why it exists.
- **Exports** — public actors, flows, types, events other features can
  call or reference.
- **Uses** — dependencies on other features and external services.
- **Policies** — rules attached at the feature scope.
- Implementation: actors, flows, policies, events, controllers, types.

The first four live in a `prose` block. The implementation lives in
sibling files (folder format) or further down the same file (single-file
format).

## Two layouts

Both supported. Pick per feature.

### Folder format

Every feature is a directory. Each concern lives in its own file.

```
myproject/spec/booking/
  prose.candy         ← intent, exports, uses, policies (read first)
  types.candy         ← feature-local types
  actors.candy        ← Booking actor, etc.
  policies.candy      ← CancellationPolicy, etc.
  flows.candy         ← PlaceBooking, CancelBooking
  events.candy        ← BookingPlaced, BookingCancelled
  controllers.candy   ← Bookings HTTP routes (read last)
```

Use when the feature is large enough that splitting helps navigation —
typically when a single file would exceed ~250 lines.

### Single-file format

Like Prisma's `schema.prisma`: one file holds the whole slice.

```
examples/airbnb/auth.candy
```

```candy
prose {
  intent: """Account signup, login, logout. Issues sessions; verifies credentials."""
  exports:
    actor User, Session
    flow  Signup, Login, Logout
  policies: [BearerAuth]
}

type Email    string  { max: 320, format: rfc5322 }
type Password string  { policies: [PasswordStrength] }

actor User(id: Id) { ... }
actor Session(token: Token) { ... }

policy PasswordStrength { ... }

flow Signup(...) -> ... { ... }
flow Login(...)  -> ... { ... }
flow Logout(...) -> ... { ... }

event UserSignedUp { ... }
event UserLoggedIn { ... }

controller Auth { ... }
```

Use when the feature fits on a screen (~150–250 lines). One scroll, full
context, lower friction.

## Detection rule

No config required. The toolchain detects layout from filesystem:

| Filesystem state                  | Layout                |
|-----------------------------------|-----------------------|
| `spec/<name>/prose.candy` exists   | Folder feature        |
| `spec/<name>.candy` exists         | Single-file feature   |
| Both exist                        | Error (ambiguous)     |
| Neither exists                    | Not a feature         |

Feature name = directory name (folder) or filename without `.candy`
(single-file).

## prose block

Same shape in either layout — top of `prose.candy` (folder) or top of
`<name>.candy` (single-file).

```candy
prose {
  intent: """
    One or more paragraphs explaining what this feature does and why it
    exists. The audience is a developer landing in this slice for the
    first time; the prose should orient them in one minute.
  """

  exports:
    actor   <ActorName>  [, <ActorName>...]
    flow    <FlowName>   [, <FlowName>...]
    type    <TypeName>   [, <TypeName>...]
    event   <EventName>  [, <EventName>...]

  uses:
    feature  <FeatureName>  for <Op1>, <Op2>, ...
    external <ExternalName> for <Op1>, <Op2>, ...

  policies: [<PolicyName>, <PolicyName>, ...]
}
```

All four sections optional; `intent:` is conventionally always present.

### exports

The public API of this feature. Anything not exported is private to the
feature; codegen refuses cross-feature references to private items.

### uses

Cross-feature and external dependencies, **with the specific operations**
each consumer relies on. Useful properties:

- **Cross-feature dependency graph is grep-able.**
  `grep -r "feature Wallet" spec/` finds every consumer.
- **Codegen knows what to wire.** It can construct dependency-injected
  containers without guessing.
- **Reviewers can spot creep.** When a feature starts using
  `feature X for OpA, OpB, OpC, OpD, OpE, ...`, that's a signal the
  boundary needs work.

### policies

Feature-scoped policies — apply to every controller and flow in the
feature. Per-actor or per-flow policies still attach individually inside
their own blocks.

## Choosing a layout

Rule of thumb (guidance, not enforcement):

- **Single-file** if the whole feature fits on a screen (~150–250 lines).
- **Folder** if you'd otherwise scroll past hundreds of lines of unrelated
  blocks.

Layout can change. A single-file feature converts into a folder feature
mechanically: extract each block into the matching sibling file, leave
the `prose` block in `prose.candy`.

## Reading order within a feature

```
prose.candy / prose block   ← read first
types
actors
policies
flows
events
controllers                 ← read last
```

Top-down. Intent first. Implementation order matches dependency order, so
a parser resolves forward references without preprocessing.

## Examples in this repo

- `examples/auth/auth.candy` — single-file feature (tutorial scale; no `prose`
  block since the example predates this design — the airbnb auth feature
  shows the production form).
- `examples/airbnb/auth.candy` — single-file production feature with
  `prose` block.
- `examples/airbnb/booking.candy` — multi-actor saga with cross-feature
  dependencies and external SDK use.
