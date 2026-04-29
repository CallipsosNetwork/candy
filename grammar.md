# candy — grammar reference

candy is a specification language for stateful backends. You describe a
system as **actors with state**, **flows that compose actors**, **controllers
that expose flows over HTTP**, **policies that capture rules**, and **events
that propagate**. From one spec, AI generates idiomatic backends in Go, Rust,
TypeScript, or Python.

The language is small (~45 single-word keywords), prose-heavy where prose
serves it, rigorous where ambiguity costs.

Files use the `.cndy` extension. The language is "candy".

---

## The five word-axes

Every keyword belongs to one of five families. Learn the families and the
words in each, and you can read any candy file.

```
ENTITY      things that exist
            actor  state  enum  type  derive  journal  audit  self  id
            flow  controller  event  policy

ACTION      things that happen
            ask  tell  emit  effect  commit  compensate  reject  step  accepts  subscribe

TIME        when, in what order, for how long
            now  then  after  before  until  expire  schedule  at  rescue

CONDITION   under what circumstances
            if  else  when  require  invariant  given  unless  where  any  in

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

| Block        | Purpose                                                  |
|--------------|----------------------------------------------------------|
| `actor`      | A stateful entity with identity, state, and messages.    |
| `flow`       | A multi-actor saga with explicit compensation.           |
| `controller` | HTTP surface — routes, auth, request/response shape.     |
| `policy`     | A rule cluster expressed in prose with examples.         |
| `event`      | A typed message broadcast to subscribers.                |
| `type`       | A record, or a branded primitive with pinned semantics.  |
| `enum`       | A sum (variant) type.                                    |
| `invariant`  | A truth that must hold (actor-local or system-wide).     |

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

`type` declares either a **branded primitive** (with `repr` and other
meta-fields) or a **record** (with named user fields). The body shape
disambiguates: meta-fields like `repr`, `unit`, `currency`, `max`, `format`
mark a primitive; user-named `field: Type` declarations mark a record.

```candy
type Money     { repr: int, unit: minor, currency: USD, round: nearest }
type Timestamp { repr: utc }
type Key       { repr: opaque, max: 128 }
type Email     { repr: string, max: 320, format: rfc5322 }

enum Status { Pending, Confirmed, Cancelled }

type LineItem {
  slot:  SlotId
  price: Money
}
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
Lists support `where <predicate>`, `+` (append), `[id]` (index by id field).

**Comments.** `// line comment`. No block comments — paragraphs go in
`intent:` blocks.

**String interpolation.** `"Hello, ${name}!"`.

---

## Project layout

A candy project is one or more `.cndy` files in a directory. Declarations
resolve across files; there is no import statement. Convention for non-trivial
projects:

```
project/
  actors/<Name>.cndy        // one actor per file
  flows/<Name>.cndy         // one flow per file
  controllers/<Name>.cndy   // one controller per file
  policies/<Name>.cndy      // one policy per file
  types.cndy                // shared types and enums
  events.cndy               // shared event declarations
  invariants.cndy           // system-level invariants
```

Small projects flatten to a single `.cndy` file. The examples in this
repository all do.
