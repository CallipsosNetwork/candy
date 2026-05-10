# NEXT — post-alpha workstream

Alpha shipped 2026-05-08: 4 backends green (auth/Go, auth/Rust, todo/Go, wallet/Go), 99 hurl scenarios passing, codegen prompts + linter + commons/ + spec grammar all merged. See `.retrospective/phase-alpha-codegen.md` for the full debrief.

This file is the menu of what comes next, ordered by adoption-blocking leverage. Each item links to its tracking doc.

## Targets, ordered by priority

1. **Phase F — harder examples (billing, notifications, airbnb).** Each surfaces grammar / preferences / external-actor edges the simple examples didn't. Billing has 3 schedules + Polar canonical. Notifications has multi-provider rescue chains + Postmark. Airbnb has the multi-actor saga with compensation. **Highest signal for "is the spec ready for a real project."** Recommended starting point: billing (smallest of the three; reuses the schedule machinery proven on wallet).

2. **TS + Python auth targets.** Closes the four-target conformance story. Validates the codegen prompts on the two languages without a `*-best-practices` skill (where prompt quality was a documented unknown — see prompts/codegen-typescript.md and prompts/codegen-python.md headers).

3. **`candy test` subcommand (Phase G).** Consolidates the hurl runner into the Rust CLI. Sets up per-target CI workflows. Cheaper than Phase F and unlocks regression-catching infrastructure.

4. **Linter learns `spec` syntax + migrate one example to `use spec ...`.** Closes the loop from the spec-grammar PR (#44) and commons/ PR (#43): teach `candy lint` to parse `spec` / `use spec` / `refines`, add two new lint rules (`spec-required-intent`, `unresolved-use-spec`), then migrate one example (auth is smallest) to `use spec Email, Hash, Token, Password` as the proof of commons consumption.

5. **Tier-1 integration test emission.** Per `docs/testing-strategy.md`: generate target-native test suites (Go testing + httptest, Rust #[tokio::test] + axum-test, vitest + hono.fetch, pytest + httpx) alongside the backend code. Each hurl scenario transliterates mechanically. Hurl stays as the cross-target reference.

6. **Brown mode B3 adapter prototype.** Per `docs/cli-modes.md`: take a small existing Go project, generate just the controller layer as adapters around existing service code. Measure the diff. The "spec change → small code diff" metric is the test for whether brownfield is viable.

7. **Codegen-time spec/preferences checker.** Per `.retrospective/phase-alpha-codegen.md` §6 forward risks: a mechanical linter rule that catches when generated code drifts from `preferences.candy` (e.g. `when need jwt use golang-jwt` declared but no jsonwebtoken-equivalent imported). Cheaper than orchestrator review-and-rewrite.

## Parallelisable

Items 1 + 2 are non-blocking — different examples / different targets / different files. A Phase F sub-agent for billing-on-Go and a TS-auth sub-agent can run concurrently.

Items 3, 4, 7 each depend only on the merged main; can run sequentially or in parallel.

Items 5 and 6 are larger explorations; each deserves its own session and likely a discuss-phase before any code lands.

## What is locked

Don't relitigate without strong reason. See `.claude/session-handoff.txt` §7 "Decisions → Locked":

- Sessions are JWTs, not KSUID-in-DB (auth.candy prose pins this; revocation via small `revoked_jtis` table; two middlewares for bearer-strict vs bearer-permissive).
- Money is integer minor units; no floats anywhere money flows.
- `Id` and `Timestamp` are language-built-in named types; everything else (Money, Email, Password, Token, Key, Role) is project-declared (per GRAMMAR.md §type "Built-in named types").
- `subscribe` block form is ratified.
- `spec` / `use spec` / `refines` are ratified (PR #44).
- Hurl is the cross-target reference, not the long-term test surface (per `docs/testing-strategy.md`).

## When you start

1. Read `.claude/session-handoff.txt` for project-level state.
2. Read this file (NEXT.md) for the menu.
3. Read `.retrospective/phase-alpha-codegen.md` for what worked, what didn't, and forward risks.
4. Pick a target from §"Targets" above.
5. Discuss with the user before spawning a multi-hour codegen sub-agent — confirm the pick.
