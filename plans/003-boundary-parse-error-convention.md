# Plan 003: Ratify the boundary type-parse error convention, fix the Rust auth target, and close the eval gap

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat e89c1d3..HEAD -- GRAMMAR.md prompts/codegen-base.md evals/auth examples/auth/targets/rust/src/auth/controllers.rs`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: plans/001-conformance-eval-runner.md (verification gate; can proceed without it using the manual fallback)
- **Category**: bug / spec ambiguity
- **Planned at**: commit `e89c1d3`, 2026-06-12

## Why this matters

The two auth backends disagree on what happens when a request body field fails
its type parse: Go signup returns `400 bad_request` for a malformed email;
Rust signup returns `422 weak_password` — an email problem reported as a
password problem. The hurl conformance suite has no invalid-email scenario, so
the divergence is invisible to the project's own quality gate. This is exactly
the class of spec edge the alpha exists to surface: the spec's controller
`map:` blocks only cover the flow's declared error variants, and the grammar
never states what a parse failure at the HTTP boundary maps to. This plan
ratifies the convention (matching the Go behavior, which the base prompt
already implies), fixes the Rust target, and adds the missing eval scenarios
so all future targets are held to it.

## Current state

The divergence:

- Go signup — `examples/auth/targets/go/internal/auth/controllers.go:147-151`:

```go
		email, err := shared.NewEmail(body.Email)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "bad_request", "detail": err.Error()})
			return
		}
```

- Rust signup — `examples/auth/targets/rust/src/auth/controllers.rs:69-72`:

```rust
    let email = match Email::parse(&body.email) {
        Ok(e) => e,
        Err(_) => return error_json(StatusCode::UNPROCESSABLE_ENTITY, "weak_password", None),
    };
```

- Login is consistent in BOTH targets and intentionally different from signup:
  a malformed email on login maps to `401 invalid_credentials`
  (`controllers.go:204-208`, `controllers.rs:108-111`), honoring the spec's
  opaque-error intent — `examples/auth/auth.candy:148`: "Errors are opaque —
  never reveal which of email or password was wrong." Do not change login.

What the contract already implies — `prompts/codegen-base.md:166` (§2
`controller` table): `body: { f: T, ... }` → "Request body schema; reject
malformed input with 400." So the Rust signup line is a one-off generation
error; what's missing is (a) an explicit grammar-level statement, (b) an eval
scenario enforcing it, (c) wording covering the opaque-credentials exception
that both targets' login already implements.

Where the convention belongs:

- `GRAMMAR.md` — `## controller` section starts at line 377; it documents
  `map:` but says nothing about boundary parse failures. There is also a
  `## Cross-cutting conventions` section at line 771.
- `evals/auth/auth.hurl` — 14 scenarios today; negative tests exist for weak
  password (asserting `jsonpath "$.error" == "weak_password"` at lines
  39/54/69) but none for a syntactically invalid email. Scenario conventions:
  section markers `# === <name> ===`, asserts on status + `$.error` (see
  `evals/README.md:86-105`).
- `evals/auth/auth.md` — the narrative twin; must stay aligned with the
  `.hurl` (repo rule, `evals/README.md:24-26`).
- `evals/COVERAGE.md` — has an auth section and a summary row (`auth | 14`)
  that must be ticked up when scenarios are added.

The convention to ratify (this is the decision, pre-made — matching Go and the
base prompt):

1. A request body (or path/query) field that fails its declared type's parse
   or validation is rejected at the boundary with `400 { error: "bad_request" }`,
   before the flow is invoked. Boundary parse failures are not domain errors
   and never map to a flow's declared error variants.
2. Exception: when a flow's intent declares opaque errors over its credential
   fields (as auth's `Login` does), parse failures of those fields map to the
   same opaque variant the flow would return (`401 invalid_credentials`),
   so that response shape never reveals which input was wrong.

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Build Rust target | `cd examples/auth/targets/rust && cargo build` | exit 0 |
| Conformance, auth only (plan 001) | `scripts/run-evals.sh auth go && scripts/run-evals.sh auth rust` | both PASS |
| Manual fallback | start backend, then `hurl --variables-file evals/auth/fixtures.env --variable BASE_URL=http://localhost:8080 evals/auth/auth.hurl --test` | all pass |
| Linter still green | `cd cli && cargo run -- lint ../examples/` | exit 0 |

## Scope

**In scope**:
- `GRAMMAR.md` (controller section and/or cross-cutting conventions — add the convention)
- `prompts/codegen-base.md` (§2 controller table — extend the existing 400 row with the opaque-credentials exception; 1–2 lines)
- `evals/auth/auth.hurl`, `evals/auth/auth.md`, `evals/COVERAGE.md` (new scenarios + count)
- `examples/auth/targets/rust/src/auth/controllers.rs` (the one-line fix) and `examples/auth/targets/rust/HANDOFF.md` (judgment-call log entry)

**Out of scope** (do NOT touch):
- Login handlers in either target — already conformant and intentionally opaque.
- `examples/auth/targets/go/` — already conformant. (If the new hurl scenario
  fails against Go, that's a STOP, not a Go edit.)
- `examples/auth/auth.candy` — the convention is grammar-level, not per-spec;
  the spec's `map:` blocks stay as they are.
- Other examples' evals (todo, wallet, ...) — adding invalid-input scenarios
  everywhere is follow-up work; this plan establishes the convention plus the
  auth reference case.

## Git workflow

- Branch: `fix/boundary-parse-convention`
- Atomic conventional commits, suggested split: `docs(grammar): ...` +
  `feat(evals/auth): ...` + `fix(auth/rust): ...`. No `git add -A`, no AI
  co-author footers.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Ratify the convention in GRAMMAR.md

In the `## controller` section (starts line 377), after the `map:` material,
add a short subsection (e.g. `### Boundary parse failures`) stating points 1
and 2 from "Current state" above, with a 3-line example contrasting signup
(`400 bad_request`) and login (`401 invalid_credentials`, opaque). Match the
document's existing voice (terse, declarative).

**Verify**: `grep -n "bad_request" GRAMMAR.md` → ≥1 match.

### Step 2: Tighten the base prompt

In `prompts/codegen-base.md` §2 controller table (line ~166), the `body:` row
already says "reject malformed input with 400". Extend that row (or add one
sentence under the table) to make the two-part rule explicit: parse failures
never map to flow error variants; opaque-credential flows are the one
exception. Reference the GRAMMAR.md subsection added in Step 1.

**Verify**: `grep -n "never map" prompts/codegen-base.md` → ≥1 match (or
equivalent phrasing you used; the point is the rule is greppable).

### Step 3: Fix the Rust signup handler

`examples/auth/targets/rust/src/auth/controllers.rs:71` — change the email
parse failure arm from
`error_json(StatusCode::UNPROCESSABLE_ENTITY, "weak_password", None)` to
`error_json(StatusCode::BAD_REQUEST, "bad_request", None)`.
Append a dated entry to `examples/auth/targets/rust/HANDOFF.md`.

**Verify**: `cd examples/auth/targets/rust && cargo build` → exit 0.

### Step 4: Add the eval scenarios

In `evals/auth/auth.hurl`, add two scenarios following the existing section
style:

1. `# === signup rejects malformed email ===` — POST `{{BASE_URL}}/signup`
   with email `"not-an-email"`, a valid strong password from the existing
   fixtures, and a fresh idempotency key → assert status 400 and
   `jsonpath "$.error" == "bad_request"`.
2. `# === login with malformed email is opaque ===` — POST `{{BASE_URL}}/login`
   with email `"not-an-email"` and any password → assert status 401 and
   `jsonpath "$.error" == "invalid_credentials"`.

Mirror both in `evals/auth/auth.md` (narrative + expected responses), update
the file-header scenario count in `auth.hurl` (convention:
`evals/README.md:88-89`), and bump `evals/COVERAGE.md`'s auth row from 14 to 16
(plus its checklist section if one itemizes scenarios).

**Verify**: `grep -c '^# ===' evals/auth/auth.hurl` → 16.

### Step 5: Conformance gate — both targets

**Verify**: `scripts/run-evals.sh auth go` → PASS and
`scripts/run-evals.sh auth rust` → PASS (or the manual fallback against each).
The two new scenarios must pass against BOTH targets — that is the whole point.

## Test plan

The two new hurl scenarios are the tests; they must be green on Go (proving
the convention matches existing behavior) and on Rust (proving the fix). The
rest of the 14 existing scenarios must stay green untouched.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `grep -n "weak_password" examples/auth/targets/rust/src/auth/controllers.rs` shows no match on the email-parse arm (line ~71); the WeakPassword flow-variant mapping (lines ~85-94) remains
- [ ] `grep -c '^# ===' evals/auth/auth.hurl` → 16, and `auth.md` documents both new scenarios
- [ ] `evals/COVERAGE.md` auth row says 16
- [ ] `scripts/run-evals.sh auth go` and `scripts/run-evals.sh auth rust` both PASS
- [ ] `cd cli && cargo run -- lint ../examples/` → exit 0 (GRAMMAR change introduced no spec edits)
- [ ] `git status` shows changes only to in-scope files
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- The new signup-invalid-email scenario FAILS against the Go target. That
  means Go's actual parse behavior differs from the excerpt (drift) — the
  convention decision may need revisiting; surface it.
- `shared.NewEmail` / `Email::parse` accepts `"not-an-email"` (i.e. email
  validation is laxer than assumed). Report what each target actually
  validates; do not invent a stricter validator in either target.
- Fixing Rust seems to require touching anything beyond the one match arm in
  `controllers.rs` plus `HANDOFF.md`.
- You find yourself wanting to edit `auth.candy` — the convention belongs in
  GRAMMAR.md; a spec edit means the design needs the maintainer.

## Maintenance notes

- Follow-up (deferred): sweep todo/wallet/billing/notifications/airbnb evals
  for the same gap — every controller with typed body fields should get one
  malformed-input scenario. Mechanical once this reference case exists.
- TS/Python auth targets (NEXT.md item 2), when generated, inherit this
  convention from the updated base prompt — their eval runs will enforce it
  via the two new scenarios for free.
- Reviewer scrutiny: that the GRAMMAR.md wording covers path/query params,
  not just body fields, without over-promising what targets do today.
