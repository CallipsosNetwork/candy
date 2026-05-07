# candy codegen — base prompt

You are generating an idiomatic backend from a candy spec. This document is
the universal contract. A target overlay (`codegen-{go,rust,typescript,python}.md`)
layers language-specific idioms on top.

Loading order:

1. Read `GRAMMAR.md` (authoritative grammar reference).
2. Read this document.
3. Read the target overlay for the language you are generating.
4. Where the overlay says "dispatch to `<language>-best-practices`", load
   that skill before emitting code.
5. Read the project's `candy.toml`, `preferences.candy`, and every `.candy`
   file under the project root.

You generate code into `targets/<lang>/` at the project root. Generated
code is the source of truth for the runtime; the spec is the source of
truth for behaviour. Both are checked in.

---

## 1. What you are reading

Every input file is one of:

| File                       | Purpose                                                      |
|----------------------------|--------------------------------------------------------------|
| `candy.toml`               | Project name, runtime version, targets, deps.                |
| `preferences.candy`        | Per-target library and idiom preferences (`when need X use Y`). |
| `*.candy` (single-file)    | A feature when no folder of that name exists.                |
| `<feature>/prose.candy`    | The entry point of a folder feature.                         |
| `<feature>/*.candy`        | Sibling spec files (`actors.candy`, `flows.candy`, ...).     |
| `evals/<feature>/*.hurl`   | Behavioural conformance tests. The contract you must pass.   |

You **do not** receive `.md` design documents at codegen time. The spec
(`.candy`) and the conformance tests (`.hurl`) are the inputs. Every
behaviour the hurl exercises must be reachable in the code you emit.

### Feature layout detection

For each feature directory under `spec/` (or each top-level `.candy` file
when the project is flat):

| State                                  | Layout                |
|----------------------------------------|-----------------------|
| `<name>/prose.candy` exists            | Folder feature        |
| `<name>.candy` exists at the same level | Single-file feature   |
| Both exist                             | **Error** — refuse to generate; report ambiguity. |
| Neither                                | Not a feature.        |

The `prose` block is the feature's public interface. Read it before any
other block in that feature.

### Reading `preferences.candy`

`preferences.candy` is candy DSL, not TOML. Each `target <name> {}` block
holds either `notes:` prose or `when need <concept> use <library>`
sentences. The concept and library identifiers are user-chosen; resolve
them against your knowledge of the ecosystem and the conventions in the
target overlay. Examples:

```candy
target go         { when need database use sqlite, when need jwt use golang-jwt }
target rust       { when need database use rusqlite, when need jwt use jsonwebtoken }
target typescript { when need database use better-sqlite3, when need jwt use jsonwebtoken }
target python     { when need database use sqlite3, when need jwt use pyjwt }
```

If a concept is referenced in the spec (e.g. the spec calls
`Payments.Charge`) but no `when need payments use ...` line exists, fall
back to the overlay's documented default for that concept.

---

## 2. The 11 block types — universal mapping contract

Each block has a fixed translation contract. Target overlays specify the
idiomatic shape; this section specifies the **invariants** that hold
across all targets.

### `actor`

A stateful entity with private state and message handlers.

| Spec construct                | Emission contract                                   |
|-------------------------------|-----------------------------------------------------|
| `state { f: T = default }`    | A persistent record with a column/field per `f`.    |
| `derive x = expr`             | A computed accessor on the record. Never persisted. |
| `invariant <pred>`            | A runtime check enforced before each `commit`.      |
| `audit name { ... }`          | An append-only history table; one row per matched event. |
| `accepts Op(args) -> Result<Ok, Err>` | A handler function returning that exact result. |
| Implicit `Type.create(...)`   | Generate a constructor function.                    |
| Implicit `Type.findBy(field: v)` | Generate a lookup-by-field finder.                |
| Implicit `Type(id)`           | Generate an addressable instance handle by id.      |
| `self`                        | Bound to the current actor instance inside `accepts`. |

Every actor's state is private. Cross-actor reads or writes go through a
`flow`. Generated code must enforce this — even if the persistence layer
would technically allow direct reads.

### `external actor`

Same shape as `actor`, modifier flips ownership. No `state {}` block.

| Spec construct       | Emission contract                                          |
|----------------------|------------------------------------------------------------|
| `config: ...`        | A struct holding configuration. `secret "ENV"` reads from env. |
| `accepts Op(...)`    | An outbound SDK call. Wrap the SDK; map its errors to declared variants. |
| `emits Event { ... }` | A subscriber-dispatching handler. Webhook routes are codegen-derived (see §6). |
| `providers: [A, B, C]` | Multi-provider mode. Generate one binding per tag.       |
| `Actor[Tag].Op(...)` | Dispatches to the binding for `Tag`. Each tag has its own config block. |

Provider binding selection: `preferences.candy` line `when need <tag-lower> use <library>` picks the SDK for `<tag>`. If a provider tag has no preference line, generate a stub binding that returns a `NotConfigured` error variant at runtime and emit a TODO comment naming the provider.

### `flow`

A multi-actor saga.

| Spec construct                        | Emission contract                                   |
|---------------------------------------|-----------------------------------------------------|
| `flow Name(args) -> Result<Ok, Err>`  | A function with that exact result type.             |
| `step name = ask Actor.Op(...)`       | A sequential await. Bind the success value to `name`. |
| `step name = tell Actor.Op(...)`      | Fire-and-forget. Do not await. Returns unit.        |
| `rescue compensate prior; reject Err` | On failure, execute the compensation chain (named steps in reverse), then reject. |
| `rescue ask Actor[Other].Op(...)`     | Provider fallback. Try alternative; final `rescue reject` terminates. |
| `commit value`                        | Successful return.                                  |
| `emit Event { ... }`                  | Publish to subscribers. Honour delivery semantics (§event). |

Compensation: when a `step` named `paid` rejects after `step held = ...`
already committed, generate a call that asks `held`'s declared
compensating message. Actors are responsible for declaring their
compensators; if a flow's compensation chain references a step whose
actor declares no `Cancel`/`Release`/equivalent, this is a generation-time
error — refuse and report.

### `schedule`

A recurring or one-shot flow.

```candy
schedule ChargeCycle(now)
  every 30d
  for any subscription in Subscription where status == Active
```

Emission contract:

- A scheduler component for the target (see overlay) that fires at the
  declared cadence.
- For each fire, evaluate the `for any X in Y where <pred>` clause as a
  query against `Y`'s actor table.
- Invoke the named flow with the matched instance + `now`.
- Emit a `ScheduleFired` event for observability.
- No implicit retries. If the flow rejects, that is final for this
  firing; the next cadence still fires normally.

### `controller`

HTTP surface. Thin: routing, auth wiring, body shape, response mapping.

| Spec construct                      | Emission contract                                   |
|-------------------------------------|-----------------------------------------------------|
| `METHOD /path -> Target`            | A route handler in the target framework.            |
| `auth: none \| bearer \| basic`     | Middleware wiring per target overlay.               |
| `body: { f: T, ... }`               | Request body schema; reject malformed input with 400. |
| `map: ok(v) -> Status Shape`        | On success, return that status with that body shape. |
| `map: err(Variant) -> Status { ... }` | On declared error variant, return that status with that body shape. |
| `auth: bearer` implicits            | `self` = authenticated principal id; `bearer` = raw token. |
| `now` (implicit)                    | Inject the current timestamp at the route boundary, not inside the handler. |

Controllers carry no business logic. Every code path inside a generated
handler is one of: parse → validate → dispatch (flow or actor message) →
map response. If you find yourself writing an `if`/conditional inside a
controller for non-validation reasons, the logic belongs in a flow.

### `policy`

A named rule cluster with examples.

| Spec construct                          | Emission contract                                 |
|-----------------------------------------|---------------------------------------------------|
| `policy Name { intent, examples }`      | A function the spec invokes via `step ok = MyPolicy(input) rescue reject Err`. |
| `examples:` cases                       | Generate unit tests asserting each `given`/`then` mapping. |
| Attachment scopes (`type`, `actor`, `flow`, `controller`, `prose`) | Wrap the appropriate emission point. See §3 below. |

When a policy's intent is prose-only (no algorithmic body), implement it
literally from the prose. The examples are the conformance: generated
unit tests must pass for every listed `given`/`then`.

### `event`

A typed message broadcast to subscribers.

| Spec construct        | Emission contract                                          |
|-----------------------|------------------------------------------------------------|
| `payload: { ... }`    | A typed message struct/record.                             |
| `delivery: eager`     | At-least-once. May duplicate. Subscribers must be idempotent. |
| `delivery: strict`    | Exactly-once. Use the target's transactional outbox or equivalent. |
| `delivery: weak`      | At-most-once. Best effort; drop on failure.                |
| `order: by Field`     | Deliver in order of `Field` per consumer.                  |
| `order: total`        | Single global order.                                       |
| `order: causal`       | Vector-clock ordering.                                     |
| `subscribe X -> Op`   | Register the subscriber on the enclosing actor. Two forms — see below. |

`subscribe` has two surface forms (GRAMMAR.md "subscribe — terse and block forms"):

- **Terse**: `subscribe X -> Handler(args)`. Arguments are positional
  fields from the event payload. Compile to a direct dispatch.
- **Block**: `subscribe X -> on event(field, ...) { body }`. The
  `on event(...)` list destructures the payload by field name; the
  body is one or more flow-shape statements (`ask`/`tell`/`if … then
  ...`). Compile to: receive event → bind the named fields → run the
  body in the subscribing actor's context (`self` is the instance
  that owns the subscribe).

Block-form bindings inside `on event(...)` must match field names
declared in `event X { payload: { ... } }`. Unknown names are a
generation error. Names may be a subset of the payload — bind only
what the body uses. Use the block form when the subscriber needs a
guard (the common `if event.field == self.id then ask ...` pattern at
codegen-derived webhook dispatch); the terse form should not carry a
guard.

If the runtime substrate (the target's chosen queue/event-bus library)
does not support a declared delivery mode, refuse to generate and report
the mismatch. Don't downgrade silently.

### `type` and `enum`

| Spec form                                          | Emission contract                                 |
|----------------------------------------------------|---------------------------------------------------|
| `type Name int { unit: minor, currency: USD }`     | A branded integer; arithmetic preserves the brand. |
| `type Money int { ... }`                           | **Never represent as float.** Integer minor units throughout. |
| `type Name string { max: 320, format: rfc5322 }`   | A branded string; constructor validates `max` and `format`. |
| `type Name opaque { max: 128 }`                    | A branded byte-or-string blob; no parsing.        |
| `type Name decimal { ... }`                        | Use the target's exact-decimal library; never `f64`. |
| `type Name instant { tz: utc }`                    | A UTC timestamp.                                  |
| `type Name { f: T }`                               | A record. Plain struct.                           |
| `enum Status { Pending, Confirmed }`               | Sum type.                                         |
| `[T]`                                              | List of `T`.                                      |
| `Option<T>` or `T?`                                | Optional.                                         |
| `Result<Ok, Err>` (where `Err = A \| B`)            | Sum result. Each variant carries its declared payload. |
| `unit`                                             | The target's unit type.                           |

Identity types (`type Id ...`): generate an opaque wrapper so an `UserId`
cannot be passed where a `BookingId` is expected.

### `spec`

A `spec` block is a `type` with prose, examples, and reuse semantics
(GRAMMAR.md "spec"). Compile it identically to its underlying `type`:

| Spec construct                              | Emission contract                                 |
|---------------------------------------------|---------------------------------------------------|
| `spec X primitive { ... }`                  | Emit as if `type X primitive { ... body ... }`.   |
| `examples:` cases on the spec               | Generate unit tests asserting each `given`/`then` mapping (same as `policy`). |
| `currency: parameter` (or any `parameter` value) | This spec is unparameterized; refuse to use it as a type without an applied parameterization. |
| `use spec X`                                | Resolve `X` (project-local first; then `[deps]`); emit as if the resolved spec body had been written inline as a `type` block in this file. |
| `use spec X(field: value, ...)`             | Apply the parameter substitution to the resolved spec, then emit as for `use spec X`. |
| `spec X primitive refines { ... }`          | Resolve the original `X`, apply the listed overrides, emit as if the merged shape had been written inline as a `type X` declaration. |

Resolution rules for `use spec X`:

1. Project-local `spec X` declaration wins.
2. Otherwise, look up `X` in projects listed in `candy.toml` `[deps]`,
   in declaration order.
3. If unresolved, refuse and report `unresolved use spec X` with the
   file and line.

A spec cannot be used as a type until every `parameter` field is
filled — either by `use spec X(field: value)` at the consumption site,
or by `refines` substituting a value. Generation fails otherwise.

### `invariant`

| Form                              | Emission contract                                 |
|-----------------------------------|---------------------------------------------------|
| Actor-local predicate             | Check before every `commit` in that actor.        |
| System-wide prose invariant       | Generate a documented test asserting the invariant. Refuse to silently skip. |

### `prose`

The feature's interface block. Drive module structure from it.

| `prose:` field   | Emission contract                                          |
|------------------|------------------------------------------------------------|
| `intent:`        | Module-level docstring/comment header.                     |
| `exports:`       | The module's public surface. Anything not exported is private. |
| `uses:`          | Cross-feature imports + external SDK adapters.             |
| `policies:`      | Apply the listed policies at every controller and flow in the feature. |

`uses: feature X for OpName` resolves to feature `X`'s `exports:` list.
If `OpName` is not in `X`'s exports, generation fails with a clear error.

### `target`

`target <name> { ... }` blocks live in `preferences.candy`. They are
hints, not constraints. See §1 "Reading `preferences.candy`".

---

## 3. Policy attachment

The `policies:` field declares **where** a policy is enforced. Five valid
attachment points, increasing in scope:

| Scope        | Attachment                                                      | Emission                                                  |
|--------------|------------------------------------------------------------------|-----------------------------------------------------------|
| Type         | Inside a `type` block.                                          | Run policy at every value construction.                   |
| Actor        | Inside an `actor` block.                                        | Wrap every `accepts` handler.                             |
| Flow         | Inside a `flow` block.                                          | Run policy as a precondition before the first step.       |
| Controller   | Inside a `controller` block (or per-route).                     | Middleware before route dispatch.                         |
| Feature      | Inside a `prose` block.                                         | Apply to every controller and flow inside the feature.    |

Generated code must make every attachment point explicit and grep-able.
Do not hide policy enforcement inside a base class or dynamic registry —
each enforcement site shows the policy by name in source.

---

## 4. Cross-cutting rules (universal — overrides any per-target idiom)

| Rule | Why |
|------|-----|
| Money is integer minor units. **No floats anywhere money flows.** | Avoid `0.1 + 0.2 = 0.30000000000000004` and similar. |
| `now: Timestamp` is an input parameter. Never read a global clock. | Determinism, testability, replay. |
| Idempotency keys are explicit. Every replayable message accepts `key: Key`. Replay returns the prior result; effects do not run twice. | Safety under retries. |
| One actor owns its state. No other actor reads or writes that state directly. Cross-actor mutation goes through a `flow`. | Locality of reasoning, isolation. |
| Time is UTC; durations write `7d`, `10m`, `1h`, `30s`, `500ms`. Arithmetic: `now after 7d`, `expires before now`. | Single tz, single shape. |
| Every `Result<Ok, A \| B \| C>` variant is reachable. Every declared error must be possible to produce in tests. Dead variants are a generation error. | Errors are part of the contract, not stubs. |
| `audit name { ... }` is append-only. Generated writes never `UPDATE` audit rows. | Audit means audit. |
| `derive` is computed; never persisted. | One source of truth. |
| `subscribe X -> Y` makes Y idempotent if X has `delivery: eager`. | At-least-once requires idempotent subscribers. |

If a target's chosen library fights any of these rules, you change the
library, not the rule. (Example: a JSON encoder that emits floats for
integer minor-unit money is not acceptable. Pick another, or wrap it.)

---

## 5. Reserved primitives

These functions are available without import:

| Function           | Returns                                              |
|--------------------|------------------------------------------------------|
| `generate()`       | A new id of the contextually expected type.          |
| `hash(value)`      | A hash of `value` using the target's chosen hash library. |
| `verify(v, h)`     | Verifies `v` against hash `h`.                       |
| `sum(list)`        | Sum of a list of numbers (int/decimal, branded or unbranded). |
| `length(list)`     | Cardinality.                                         |
| `last(list)`       | Last element; rejects empty list.                    |
| List operators     | `where <predicate>`, `+` (append), `[id]` (index by id field). |

`sum`, `length`, `last` apply to lists of any numeric type, including
branded ones (`[Money]` works). Implement them per target without
breaking the type brand.

`generate()`'s id type is inferred from the assignment target. When the
spec writes `step booking_id = generate()` and uses `booking_id` as a
`BookingId`, generated code constructs a `BookingId`.

**Race-condition trap**: when a flow refers to a generated id in both
its happy-path and a `rescue` path, the id must be generated **once** at
the top of the flow and reused — never re-called. (M1 caught a real bug
where `PlaceBooking` re-called `generate()` in its rescue path and
released the wrong booking. Don't repeat that.)

---

## 6. Webhooks

When an `external actor` declares `emits SomeEvent { ... }`, codegen
produces:

1. An inbound HTTP route at a conventional path (overlay-specified).
2. Signature verification against the provider's webhook secret (read
   from `config:`).
3. Payload mapping from the provider's wire shape to the declared
   `emits` event shape.
4. Dispatch to every `subscribe SomeEvent -> ...` handler in the
   project.

The spec does **not** declare controller routes for webhooks. If you see
a `controller` block with a route that maps to a webhook handler, that
is a spec error — refuse and report.

For multi-provider externals, each provider's wire shape is mapped to
the same declared `emits` event. Generated code dispatches via the
provider tag (Stripe payload → `Payments[Stripe]` binding's mapper).

---

## 7. Project layout in the generated tree

The generated tree under `targets/<lang>/` mirrors the spec's feature
structure plus per-target boilerplate.

```
targets/<lang>/
  README.md                   — how to run; what was generated; from which spec commit.
  <build-manifest>            — go.mod / Cargo.toml / package.json / pyproject.toml.
  <project-source-root>/
    <feature>/                — one subdirectory per feature in the spec.
      ...                     — actor, flow, controller, policy implementations.
    shared/                   — types, events, invariants common across features.
    runtime/                  — the substrate (DB pool, scheduler, event bus).
    main entry point          — wires everything together.
  test/                       — integration tests; can run hurl against the binary.
```

Every generated source file starts with a header line naming:

- The spec source path it was generated from (e.g. `// generated from spec/auth/auth.candy`).
- The candy runtime version from `candy.toml`.
- A "do not edit" notice; humans regenerate, they do not edit.

---

## 8. Conformance gate — your output is judged by hurl

`evals/<feature>/<feature>.hurl` is the behavioural contract. Every
declared behaviour must be reachable. Concretely:

- Every `controller`'s every `ok(...) -> Status` is exercised by at
  least one happy-path scenario. Generated code must hit that exact
  status with that exact body shape.
- Every `err(Variant) -> Status` is exercised. Generated code must map
  that exact variant to that exact status.
- Auth-required routes return 401 on missing/invalid bearer.
- Role-gated routes return 403 on wrong role.
- Replayable flows declaring `key: Key` return identical responses on
  identical-key replay.
- State preconditions are exercised both ways (e.g. verify-already-
  verified → 409). Generated code must produce the declared error
  variant in the declared case.
- Compensation paths are tested when the failure point is mockable.
  Generated code must make the compensating call when its declared
  step rejects.
- Schedule predicates are tested by setting up matching state and
  sleeping past the cadence. Generated schedulers must fire the flow.

If your code passes the relevant `.hurl`, the spec was implemented
correctly. If it fails, the diff between observed and asserted is your
bug list.

---

## 9. Determinism and regeneration

Generated code is fully owned by the codegen. Humans do not edit
`targets/<lang>/`. When the spec changes:

1. Regenerate the target.
2. Run the hurl conformance suite.
3. Commit the regenerated tree.

Generation is not required to be byte-identical across runs — but
arbitrary churn is a smell. Use stable orderings (alphabetical by name
for declarations, source order for scopes). Do not embed timestamps in
generated source.

---

## 10. What to refuse

Refuse to generate, with a clear error, when:

| Condition                                                                                       |
|-------------------------------------------------------------------------------------------------|
| The spec uses a syntax not described in `GRAMMAR.md`.                                           |
| Both `<name>/prose.candy` and `<name>.candy` exist (ambiguous feature layout).                  |
| A `uses: feature X for Op` references an `Op` not in `X`'s `exports:`.                          |
| A `flow`'s compensation chain references an actor with no compensating message declared.        |
| A `subscribe X -> Y` requires idempotency (X is `delivery: eager`) but Y has no `key: Key`.     |
| A multi-provider external references a tag with no `config <Tag>:` block.                       |
| A `controller` route maps to a webhook handler (webhook routes are codegen-derived; not in spec). |
| A `type` declares `float` as its underlying primitive (money rule).                             |
| Any required field on a block is missing per `GRAMMAR.md`.                                      |

When refusing, name the file and line of the offending construct, name
the rule, and stop. Do not emit partial output.

---

## 11. Style notes — universal

- **Names**: preserve identifiers from the spec. `BookingId` in the spec
  is `BookingId` in the code. No transliteration to snake_case unless
  the target's idiom demands it (Go: types `BookingId`, fields
  `booking_id`; Python: type `BookingId`, fields `booking_id`; Rust:
  type `BookingId`, fields `booking_id`; TS: type `BookingId`, fields
  `bookingId`).
- **Comments**: short. The spec block's `intent:` is the docstring.
  Don't restate what the code does; explain why if and only if the spec
  doesn't.
- **Imports**: only what's used. Never grep-bait.
- **Tests**: every `policy` block's `examples:` translates to a unit
  test; every `controller`'s every `ok`/`err` mapping is reachable from
  the hurl scenarios.

---

## 12. Sequence — how to actually generate

For a project with N features and T targets, the order is:

1. Read `candy.toml`. Identify targets.
2. Read `preferences.candy`. Build per-target preference maps.
3. Walk the spec. Build a global symbol table:
   - All declared types, enums, events, policies, actors, externals,
     flows, controllers, schedules.
   - Per-feature `prose` exports/uses graph.
4. Validate the spec against §10. If anything fails, refuse.
5. For each target:
   1. Load the target overlay.
   2. Dispatch to the language-best-practices skill if the overlay
      names one.
   3. Emit the runtime substrate (DB pool, scheduler, event bus, HTTP
      framework wiring).
   4. Emit shared types, enums, events.
   5. Per feature, emit actors → flows → controllers → policies, in
      dependency order.
   6. Emit `main` (or equivalent) wiring everything together.
   7. Emit test scaffolding (policy-example tests, hurl runner).
6. Run the target's lint/format tooling. Fix any issue before reporting
   the generation done.

---

## 13. Out of scope for v0.1

- Database migrations beyond the initial schema (no incremental migrations yet).
- Multi-region / multi-tenant routing.
- Authorization beyond `auth: bearer | basic | none` and policy-based RBAC.
- Streaming responses.
- GraphQL or any non-REST surface.
- Cryptographic key rotation.

When the spec needs any of these, refuse and report. Don't half-generate.

---

## Reference

- `GRAMMAR.md` — language reference.
- `evals/README.md` — conformance contract.
- `docs/architecture.md` — substrate vs spec layering.
- `docs/externals.md` — external actor pattern, webhooks-as-events.
- `docs/features.md` — feature layout detection.
- `docs/candy-toml.md` — manifest schema.
