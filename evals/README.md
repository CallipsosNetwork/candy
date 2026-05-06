# Behavioral conformance evals

Hurl-based black-box tests that any backend generated from a candy spec
must pass. The same `.hurl` file runs unchanged against the Go, Rust,
TypeScript, and Python target backends — that's the conformance contract.

This directory is the *spec for what the generated code must do*. It is
authored before any backend exists; the hurl scripts can be read and
reviewed as a behavioral contract today, and run as tests once codegen
ships.

## Structure

```
evals/
  README.md                       — this file (the contract)
  COVERAGE.md                     — per-feature coverage checklist
  <example>/
    fixtures.env                  — KEY=value seed data, hurl-loadable
    <feature>.md                  — narrative scenario (the plan)
    <feature>.hurl                — executable script (what runs)
```

One `<feature>.md` + `<feature>.hurl` pair per feature. The `.md` is what
humans (and Claude) read to understand intent; the `.hurl` is what runs.
They must stay aligned — if a scenario step changes in one, change both.

## How to run

```sh
# Against any one target backend, with its base URL:
hurl --variables-file evals/auth/fixtures.env \
     --variable BASE_URL=http://localhost:8080 \
     evals/auth/auth.hurl

# Against all four targets in sequence (once codegen lands):
for target in go rust typescript python; do
  start_backend $target
  hurl --variables-file evals/auth/fixtures.env \
       --variable BASE_URL=$(backend_url $target) \
       evals/auth/auth.hurl
  stop_backend $target
done
```

Hurl 4.x or later. Backends are expected to be reachable at `BASE_URL`
and to start with empty state — every test bootstraps the actors it
needs (no shared seed data).

## Coverage rules — what counts as "done" for a feature

For every controller endpoint:

1. **Every `ok(...) -> Status` is exercised.** At least one happy-path
   request that returns the documented success status with the documented
   body shape.
2. **Every `err(Variant) -> Status` is exercised.** Negative tests that
   trigger each declared error variant.
3. **Auth-required routes get a missing-bearer + invalid-bearer case.**
   Both must return 401.
4. **Role-gated routes get a wrong-role case.** Must return 403.
5. **Replayable flows (those declaring `key: Key`) get an idempotency
   replay test.** Same key sent twice → same response, same observable
   state (verified via a state-readback request when possible).
6. **State-precondition errors are exercised.** Every transition that has
   a "from" precondition gets both sides tested (e.g. Verify-an-already-
   verified user → 409).

For every saga (multi-step flow with `compensate`):

7. **The happy path is exercised end-to-end.**
8. **At least one compensation path is exercised** (forcing one step to
   fail and verifying upstream state rolls back). When the failing step
   is on an external actor and we don't have a stub harness, the
   compensation test is documented in the `.md` and skipped in the
   `.hurl` with a TODO.

For every scheduled flow:

9. **The schedule predicate is exercised** by setting up state that the
   predicate matches, sleeping past the next firing window, and asserting
   the scheduled flow ran (via a state-readback or emitted event).

`COVERAGE.md` mirrors this list per feature with checkbox rows.

## Hurl conventions

- **File header**: one comment block at the top giving the feature name,
  scenario count, and a one-line summary.
- **Section markers**: `# === <scenario name> ===` for each scenario.
  Scenarios run sequentially; later scenarios may rely on earlier
  captures.
- **Variables**: lowercase snake_case (`{{user_token}}`,
  `{{listing_id}}`). Defined either in `fixtures.env` (seed values) or
  via `[Captures]` from prior responses (dynamic ids/tokens).
- **Captures**: name them after the value, not the source endpoint. Use
  `user_id` not `signup_user_id`.
- **Asserts**: prefer `[Asserts]` over `HTTP <status>` for anything
  beyond the status code itself. Assert on `jsonpath` for structured
  bodies; on header values for CORS / cache / auth headers.
- **Idempotency keys**: when a flow accepts `key: Key`, the request body
  must include `idempotency_key`. Reuse the same `{{var}}` for replay
  tests so the captures match.
- **Negative tests**: every `err(...)` test asserts both the status
  *and* the error body shape (`jsonpath "$.error" == "weak_password"`).

## Test isolation

Each `.hurl` file is self-contained:

1. Bootstrap users via `/signup` flows (every example except hello has
   inlined auth — see the spec evolutions PR for the canonical shape).
2. Capture tokens, ids, and any other identifiers via `[Captures]`.
3. Run scenarios sequentially using captured + fixture values.
4. No teardown — backends are expected to start with empty state.

Cross-`.hurl`-file scenarios (e.g. an airbnb booking that depends on a
listing the listings hurl created) are out of scope. Each file
re-bootstraps its own state.

## Test data philosophy

- **Fixed strings.** `alice@candy.local`, `correct horse battery staple
  9`, `Test Listing — Minute Studio`. Reproducibility beats realism.
- **Models, not values.** Tests model behavior — "user receives an id"
  not "user receives uuid `abc-123`". Capture the id; assert it exists,
  matches a pattern if needed (`@regex` on JSONpath); never pin a
  specific value.
- **Per-example fixtures.env.** Seed values that don't naturally come
  from a flow capture. Hurl-native key=value format.
- **No production secrets in fixtures.** Real API keys for Polar /
  Postmark / etc. live in target-level `.env` files (codegen wires);
  fixtures.env contains only benign test values.

## What's deferred

These categories of tests are documented in scenario `.md` files but
omitted from `.hurl` until the supporting harness exists:

- **Webhook handler tests** (e.g. injecting a `ChargeSucceeded` event to
  verify a Booking transitions to Confirmed). Needs a test-mode in the
  external SDK adapter.
- **Failure-injection compensation tests** (forcing `Payments.Charge`
  to fail to verify `HoldDates` rolls back). Needs an external mock.
- **Cross-target invariant tests** (the same scenario producing
  byte-identical state across all four targets). Comes after #17.

When you write a `.md`, document these gaps clearly. Coverage does not
mean "tested today" — it means "documented today, runnable once the
harness exists."

## Adding a new feature

When a new flow / controller / actor lands in a spec:

1. Add a row in `COVERAGE.md` for each new endpoint.
2. Update the relevant `<feature>.md` with the new scenario narrative.
3. Add the corresponding scenario block to `<feature>.hurl`.
4. If the feature is its own example, create a new directory.
