# candy testing strategy — beyond hurl

`hurl` is the conformance gate today, not the long-term contract.
Candy should be expressive enough to generate not only the feature
implementation but also the test suite for it — in the **target
language's idiomatic test framework**. Hurl is a temporary
scaffolding; the final shape is target-native UAT, with a focus
on integration.

---

## Where we are

Today every example ships an `evals/<feature>/<feature>.hurl`. The
same hurl file runs unchanged against any generated backend (Go,
Rust, TS, Python). Cross-target conformance is the contract:

> If the same hurl scenarios pass, the backends are behaviourally
> equivalent.

This was the right v0.1 choice. Hurl is target-agnostic, so we
proved candy's pipeline once and it applies to four targets without
re-authoring the test corpus.

But for an adoption-ready candy, the test surface a team writes
and reads daily must be in the target language they're already
using.

| Today (hurl)                              | Tomorrow (target-native)                            |
|-------------------------------------------|------------------------------------------------------|
| Cross-target conformance contract.        | Same — hurl stays as the cross-target reference.    |
| One file per feature.                     | Same — one file per feature, plus per-target suites. |
| Authored by hand from the spec.           | **Generated** alongside the feature code.            |
| Run via `hurl --variables-file ...`.      | Run via the target's native runner: `go test`, `cargo test`, `vitest`, `pytest`. |
| HTTP-level black box.                     | HTTP-level + spec-level (per-actor, per-policy).     |

---

## Two test tiers per target

The codegen should emit **two** test suites per target, both derived
from the spec.

### Tier 1 — Integration (focus, primary)

End-to-end HTTP-level tests that exercise the server the same way
hurl does today, but written in the target's idiomatic test
framework. Each tier-1 test corresponds to a hurl scenario.

| Target     | Idiomatic surface                                   |
|------------|------------------------------------------------------|
| Go         | `testing` package + `httptest.Server` + a simple HTTP client (or `resty`). Subtests per scenario. |
| Rust       | `#[tokio::test]` + `axum::test_helpers::TestServer` (or `axum-test`). |
| TypeScript | `vitest` + `supertest`-style. Hono has its own `app.fetch(req)` for in-process testing — preferred over real HTTP for speed. |
| Python     | `pytest` + `httpx.AsyncClient` against the FastAPI app, or FastAPI's `TestClient`. |

**Source of truth.** The `evals/<feature>/<feature>.hurl` scenarios
remain canonical for cross-target conformance. The integration
suite is a target-language transliteration:

- Each `# === scenario ===` block becomes one `t.Run(...)` (Go),
  `#[tokio::test]` (Rust), `it(...)` (TS), `def test_...()`
  (Python).
- `[Captures]` become local variables.
- `[Asserts]` become target-native assertions.

The transliteration is mechanical. The codegen has the spec; it
knows which routes exist, what the controller declares for each
status code, what the eval scenarios look like — it can emit the
target-native suite from the same inputs.

This is the **focus** per the user's directive: integration tests
come first because they prove the backend behaves correctly under
real HTTP traffic.

### Tier 2 — Unit (supporting)

Per-policy, per-flow, per-actor tests that don't go through HTTP.
These already exist for the four PR'd targets — every spec
`policy` block's `examples:` translates to a unit test (the Go
target's `policies_test.go`, Rust's `#[cfg(test)]` mod inside
`policies.rs`, etc.).

Spec → unit-test mapping (current rules in the codegen prompts):

| Spec construct                      | Unit test                                         |
|-------------------------------------|---------------------------------------------------|
| `policy X { examples: ... }`        | One test per `given`/`then` pair.                 |
| `actor X { invariant <pred> }`      | One test asserting the predicate holds after each `accepts` mutation. (Not yet emitted.) |
| `flow X` rescue paths               | One test per declared error variant. (Not yet emitted.) |
| `event X { delivery: strict }`      | One test asserting at-most-once observability. (Not yet emitted.) |

Tier 2 catches policy / invariant regressions without spinning up
the server. Tier 1 catches integration regressions where the server
+ DB + middleware + serialisation must all line up.

### What hurl becomes

Not deleted. Demoted to its proper role: the **cross-target
reference**.

- A single hurl run still verifies a generated backend matches the
  conformance contract.
- The CI matrix runs hurl against each target backend (today done
  ad-hoc; should be per-PR CI).
- Teams may or may not run hurl in their day-to-day loop. The
  language-native suites are the default loop; hurl is the shared
  fence-line.

---

## Spec-side work to support this

The spec needs to give the codegen enough information to emit
high-quality integration tests, not just CRUD shells. Some
candidate additions:

| Construct                                 | Why                                                             |
|-------------------------------------------|------------------------------------------------------------------|
| `controller X { examples: ... }`          | Today examples live in `policy` blocks. Adding HTTP-shape examples (request/response pairs) on `controller` would let codegen emit integration scenarios directly. |
| `flow X { properties: ... }`              | A property-based testing surface (e.g. "two replays with the same key produce the same response"). Codegen → property tests via the target's idiomatic library (`testing/quick`, `proptest`, `fast-check`, `hypothesis`). |
| `seed: ...` block on a feature            | Declarative test-mode seed data. Codegen emits the per-target test setup. |
| `mock external X { Op returns ... }`      | Per-test stubbing of external actors (Stripe, Postmark) without the SDK roundtrip. Today's `[d]` deferred scenarios are exactly the cases this would unlock. |

These are **not** in scope for the immediate post-alpha work. They
are the spec evolutions that make tier-1 generation richer than
"transliterate hurl one-to-one."

---

## Sequencing

1. **Now.** Hurl stays canonical; tier-2 unit tests already emit
   per spec policy examples (proven on Go and Rust auth targets).
2. **Next.** Add tier-1 integration test emission to the codegen
   prompts. Each target overlay names its idiomatic test framework
   (per the table in §"Tier 1"). One example as a proof-point —
   probably auth, since its hurl is the smallest.
3. **Later.** Spec evolutions for richer test generation
   (`controller examples`, `flow properties`, `mock external`).
4. **Eventually.** Hurl becomes optional — kept only as the
   cross-target reference contract; teams running on a single
   target use only the language-native suite.

---

## Why this matters for adoption

A team adopting candy on an existing project (the brown mode in
[cli-modes.md](./cli-modes.md)) won't write hurl files. They'll
write the tests their team already understands — `go test`,
`vitest`, etc. If candy can't generate those, it's a meaningful
ergonomic gap. If it can, candy becomes a tool that not only
specifies what the backend does but also verifies that it does it,
in the team's existing idiom.

The diff between "candy emits hurl I have to learn" and "candy
emits `_test.go` I already read every day" is the difference
between a niche tool and a default.

---

## Open questions

- **Snapshot vs assertion.** Some spec examples (especially
  policy results) lend themselves to snapshot tests; others
  (e.g. JSON body shapes) need explicit assertions. Per-target
  decision or spec-level pin?
- **Test data fixtures.** `evals/<feature>/fixtures.env` is a flat
  KV file. Tier-1 generation needs target-language fixtures
  (Go fixtures package, Rust `lazy_static`, TS const objects,
  Python pytest fixtures). Same content, different shape.
- **Integration test setup.** Each target needs an idiomatic way
  to start the server, run scenarios, tear down. Avoid "one
  global state per test" anti-patterns; each test bootstraps its
  own.
- **Cross-target consistency.** When the spec changes, all
  target-native suites and the hurl reference should update in
  lockstep. The codegen pipeline ensures this; the workflow
  ergonomics around it (CI, watch mode) are open.
- **Mocking strategy for externals.** When `external actor
  Payments` exists, tier-1 tests need a fake. The spec has the
  shape; the codegen can emit a stub. Per-target conventions
  differ (Go interfaces with hand-rolled stubs vs vitest's
  `vi.mock` vs pytest's `monkeypatch`).

---

## Refs

- `evals/README.md` — current hurl-based conformance contract.
- `prompts/codegen-base.md` §8 "Conformance gate" — describes the
  hurl-passes-as-success rule. Will need updating once tier-1
  integration emission lands.
- `docs/cli-modes.md` — brownfield mode adoption story (this doc
  is the reason brownfield can be palatable).
- `.retrospective/phase-alpha-codegen.md` §8 "What didn't work" —
  notes "no CI for codegen targets yet"; tier-1 native suites are
  the natural CI surface.
