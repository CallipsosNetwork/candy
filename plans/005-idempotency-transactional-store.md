# Plan 005: Make the signup idempotency record transactional with user creation (prompt rule + auth/go)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat e89c1d3..HEAD -- prompts/codegen-base.md examples/auth/targets/go/internal/auth/`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S–M
- **Risk**: MED (touches the auth/go persistence path)
- **Depends on**: plans/001-conformance-eval-runner.md (verification gate). Run after 002/003 to avoid merge friction in the same files.
- **Category**: bug / prompt gap
- **Planned at**: commit `e89c1d3`, 2026-06-12

## Why this matters

The candy contract says replaying a flow with the same idempotency key
"returns the prior result; effects do not run twice"
(`prompts/codegen-base.md` §4). The generated auth/go Signup writes the
idempotency record *after* committing the user, and deliberately swallows a
store failure. In that failure window, a client retry gets `409 email_taken`
instead of the replay result the spec promises — the idempotency contract
silently degrades. (The in-code comment also misdiagnoses the consequence as
"a duplicate user", which the email-uniqueness check prevents.) Both writes go
to the same SQLite database, so a transaction fixes it outright. The prompt
needs the rule so future targets don't regenerate the same gap.

## Current state

- The contract — `prompts/codegen-base.md:329` (§4 Cross-cutting rules table):
  "Idempotency keys are explicit. Every replayable message accepts `key: Key`.
  Replay returns the prior result; effects do not run twice." Nothing about
  atomicity of the idempotency record with the flow's state mutations.
- The gap — `examples/auth/targets/go/internal/auth/flows.go:102-108`:

```go
	// Persist idempotency record for future replays.
	if err := deps.Idempotency.StoreSignup(ctx, key, user.ID); err != nil {
		// Non-fatal: idempotency store failure does not roll back the user.
		// A subsequent replay will create a duplicate user; v0.1 has no
		// distributed transaction. Acceptable for the conformance gate.
		_ = err
	}
```

- The structures involved (all share one `*sql.DB` opened in
  `cmd/server/main.go`):
  - `examples/auth/targets/go/internal/auth/actors.go:33` — `type UserRepo struct{ db *sql.DB }`; `Create` at line 39.
  - `examples/auth/targets/go/internal/auth/actors.go:247` — `type IdempotencyRepo struct{ db *sql.DB }`; `FindSignup` at 253, `StoreSignup` at 267.
  - `examples/auth/targets/go/internal/auth/flows.go:25-31` — `Deps` holds `Users *UserRepo`, `Idempotency *IdempotencyRepo`, etc.
  - The Signup flow body is `flows.go:51-110`: idempotency lookup (fatal on
    error, lines 53-54) → password policy → email-taken check → `Users.Create`
    → JWT issue → `Idempotency.StoreSignup` (non-fatal) → event emit.
- This tree is a generated artifact ("do not edit — regenerate from spec"
  header). The repo's precedent for targeted corrections is a manual
  regeneration: change the source of truth (prompt) and the generated tree in
  the same PR, logging the judgment call in the tree's `HANDOFF.md` (see
  commits `dd23c22`, `aadd902`).
- todo/go and wallet/go also have idempotency stores (`loadIdem` in
  `examples/todo/targets/go/internal/auth/flows.go:189`); whether they share
  the same non-fatal pattern was NOT audited. Check them read-only; if they
  have the same gap, report it in your completion notes — do NOT expand scope.

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Build/vet | `cd examples/auth/targets/go && go build ./... && go vet ./...` | exit 0 |
| Conformance | `scripts/run-evals.sh auth go` | PASS (includes the idempotency replay scenario) |
| Manual fallback | start backend, `hurl --variables-file evals/auth/fixtures.env --variable BASE_URL=http://localhost:8080 evals/auth/auth.hurl --test` | all pass |

## Scope

**In scope**:
- `prompts/codegen-base.md` (§4 — extend the idempotency row)
- `examples/auth/targets/go/internal/auth/flows.go` (Signup)
- `examples/auth/targets/go/internal/auth/actors.go` (tx-aware variants of the two writes, if needed)
- `examples/auth/targets/go/HANDOFF.md` (judgment-call entry)

**Out of scope** (do NOT touch):
- todo/go and wallet/go trees — read-only check, report findings only.
- The Rust auth target.
- `examples/auth/auth.candy` and the eval suite — the existing replay scenario
  already covers the happy idempotency path; fault-injection testing of the
  store-failure window needs a harness that doesn't exist (`evals/README.md`
  "What's deferred").

## Git workflow

- Branch: `fix/auth-go-idempotency-tx`
- Two atomic commits: `docs(prompts): idempotency record commits atomically with flow effects` and `fix(auth/go): write idempotency record in the user-create transaction`. No `git add -A`, no AI footers.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Extend the prompt rule

In `prompts/codegen-base.md` §4, extend the idempotency row (line ~329):
when the idempotency record and the flow's state mutations share a store, they
commit in **one transaction**; when they cannot (different stores), a failed
idempotency write makes the flow fail — it is never silently swallowed, because
a lost record breaks the replay contract for every future retry.

**Verify**: `grep -n "one transaction\|same transaction" prompts/codegen-base.md` → ≥1 match.

### Step 2: Make the auth/go writes transactional

Smallest design that fits the existing repo-struct pattern:

1. In `actors.go`, refactor the two write methods to take a `dbtx` interface
   both `*sql.DB` and `*sql.Tx` satisfy:

```go
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
```

   Keep the public signatures `Users.Create(ctx, u)` / `Idempotency.StoreSignup(ctx, key, id)`
   working as today (delegating to the repo's `db`), and add tx variants
   (e.g. `CreateTx(ctx, tx, u)` / `StoreSignupTx(ctx, tx, key, id)`) — or have
   the methods accept the `dbtx` directly, whichever produces the smaller
   diff. Match the file's existing error-wrapping style
   (`fmt.Errorf("...: %w", err)`).
2. In `flows.go` Signup, replace the `Users.Create` call and the
   `Idempotency.StoreSignup` block with a single transaction: `BeginTx` →
   create user → store idempotency record → `Commit`; any error rolls back and
   the flow returns it. `Deps` has no raw DB handle today — add `DB *sql.DB`
   to `Deps` (`flows.go:25-31`) and wire it where `Deps` is constructed
   (find it: `grep -rn "Deps{" examples/auth/targets/go/`).
3. Delete the stale four-line comment; replace with one line stating the
   transactional guarantee.
4. JWT issuance and the event emit stay OUTSIDE the transaction (no DB writes;
   keep ordering: tx commit → JWT → emit. Note the JWT for the *response* was
   already issued fresh per request — re-check the flow body so the issue call
   still happens exactly once on the happy path).
5. Append the HANDOFF.md entry.

**Verify**: `cd examples/auth/targets/go && go build ./... && go vet ./...` → exit 0.

### Step 3: Conformance gate

**Verify**: `scripts/run-evals.sh auth go` → PASS. The auth suite includes a
same-key replay scenario (`evals/README.md:62-64` coverage rule 5) — it must
still return the same user with a fresh token.

## Test plan

The hurl idempotency replay scenario is the behavioral test for the contract's
happy path. The failure window itself (store fails mid-flow) is not testable
without fault injection — out of scope per `evals/README.md` "What's
deferred"; the transaction makes the window structurally impossible rather
than tested-around.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `grep -n "Non-fatal" examples/auth/targets/go/internal/auth/flows.go` → no match
- [ ] `grep -n "BeginTx" examples/auth/targets/go/internal/auth/flows.go` → ≥1 match
- [ ] `go build ./... && go vet ./...` exit 0 in the auth/go tree
- [ ] `scripts/run-evals.sh auth go` → PASS
- [ ] Prompt rule grep from Step 1 passes
- [ ] `git status` shows changes only to the four in-scope files
- [ ] Completion notes state whether todo/go and wallet/go share the gap (read-only check)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- `Users.Create` turns out to do more than a single INSERT (e.g. reads back
  the row through a separate query that a `*sql.Tx` changes the semantics of)
  — report before restructuring.
- Wiring `DB *sql.DB` into `Deps` requires touching more than 2 construction
  sites.
- The hurl replay scenario fails after the change — the transaction reordered
  something observable; report the failing scenario verbatim.
- SQLite returns locking errors (`database is locked`) under the suite —
  the driver/config may need `_busy_timeout`; report rather than tuning
  connection settings in a generated tree.

## Maintenance notes

- When todo/wallet (and future targets) are regenerated, the new prompt rule
  applies; the read-only check in this plan tells the maintainer whether those
  trees need the same manual regeneration sooner.
- Reviewer scrutiny: transaction boundaries — exactly user-create +
  idempotency-store inside; JWT + event emit outside; rollback path returns
  the underlying error.
- Deferred: a fault-injection harness that can actually exercise the failure
  window (`evals/README.md` "What's deferred" already tracks the category).
