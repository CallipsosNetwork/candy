# Plan 006: Consolidate the duplicated `base_type_name` helper in the lint rules

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat e89c1d3..HEAD -- cli/candy/src/lint/rules/`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P3
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: tech-debt
- **Planned at**: commit `e89c1d3`, 2026-06-12

## Why this matters

Two lint rules each define their own `fn base_type_name(t: &str) -> &str`, and
the copies have already drifted: they return **different results** for
`Option<T>` (one returns `T`, the other `Option`) and for `[T]?` (different
strip order). Any future fix to type-wrapper stripping must be applied twice
or the rules silently disagree about what a "base type" is. One shared helper,
with the more complete behavior, removes the divergence.

## Current state

- `cli/candy/src/lint/rules/broken_symbol_ref.rs:67-85` — the **complete**
  version (keep this behavior):

```rust
/// Strip list/optional wrappers to get the base type name.
fn base_type_name(t: &str) -> &str {
    let t = t.trim();
    // [T] -> T
    let t = if t.starts_with('[') {
        t.trim_start_matches('[').trim_end_matches(']')
    } else { t };
    // T? -> T
    let t = t.trim_end_matches('?');
    // Option<T> -> T
    let t = if let Some(inner) = t.strip_prefix("Option<") {
        inner.trim_end_matches('>')
    } else { t };
    // Result<Ok, Err> — just take the name itself
    t.split('<').next().unwrap_or(t).trim()
}
```

- `cli/candy/src/lint/rules/actor_state_defaults_typed.rs:86-94` — the
  abbreviated version: trims `?` *before* brackets and has no `Option<`
  handling (so `Option<T>` → `Option`, and `[T]?` strips in a different
  order).
- The shared module is `cli/candy/src/lint/rules/mod.rs` — it declares the
  rule submodules and `run_all` (lines 20–37). No shared helpers exist there
  yet; this introduces the first one.
- Toolchain gates (all green at planning time, enforced by CI):
  `cargo fmt --all -- --check`, `cargo clippy -- -D warnings`,
  `cargo test --all` (18 tests), `cargo run -- lint ../examples/` (exit 0).
- Convention note: rules import from crate paths like
  `use crate::lint::parser::...` — match that style
  (`use super::base_type_name;` or `use crate::lint::rules::base_type_name;`).

## Commands you will need

| Purpose | Command (from `cli/`) | Expected on success |
|---|---|---|
| Format | `cargo fmt --all -- --check` | exit 0 |
| Lint | `cargo clippy -- -D warnings` | exit 0 |
| Tests | `cargo test --all` | 18+ passed, 0 failed |
| Examples still clean | `cargo run -- lint ../examples/` | exit 0 |

## Scope

**In scope**:
- `cli/candy/src/lint/rules/mod.rs` (add the shared helper + unit tests)
- `cli/candy/src/lint/rules/broken_symbol_ref.rs` (remove local fn, import shared)
- `cli/candy/src/lint/rules/actor_state_defaults_typed.rs` (remove local fn, import shared)

**Out of scope** (do NOT touch):
- The other eight rule files, `parser.rs`, `output.rs`, `main.rs`.
- Rule behavior beyond what the helper unification implies — if unifying makes
  `actor_state_defaults_typed` newly resolve `Option<T>` defaults differently
  and a fixture starts failing, that is a STOP, not a fixture edit.

## Git workflow

- Branch: `refactor/lint-shared-base-type-name`
- One commit: `refactor(cli/lint): share base_type_name across rules`. No AI footers.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Add the shared helper with unit tests

Move the `broken_symbol_ref.rs` version (the complete one) into
`cli/candy/src/lint/rules/mod.rs` as `pub(crate) fn base_type_name`. Add a
`#[cfg(test)] mod tests` in `mod.rs` covering: `"User"` → `User`,
`"[Money]"` → `Money`, `"Token?"` → `Token`, `"[Item]?"` → `Item`,
`"Option<Key>"` → `Key`, `"Result<A, B>"` → `Result`, `"  spaced  "` → `spaced`.

**Verify**: `cargo test --all` → previous 18 integration tests plus the new
unit tests all pass.

### Step 2: Switch both rules to the shared helper

Delete both local `fn base_type_name` definitions; import the shared one in
each file. Remove any imports the deletions orphan.

**Verify**: `grep -rn "fn base_type_name" cli/candy/src/` → exactly 1 match
(in `rules/mod.rs`); `cargo clippy -- -D warnings` → exit 0.

### Step 3: Full gate

**Verify**: from `cli/`: `cargo fmt --all -- --check && cargo clippy -- -D warnings && cargo test --all && cargo run -- lint ../examples/` → all exit 0.

## Test plan

New unit tests in `mod.rs` (Step 1 list — they pin the unified semantics,
including the two divergence cases `Option<Key>` and `[Item]?`). The existing
18 integration tests (subprocess-based, `cli/candy/tests/integration_test.rs`)
guard rule behavior end-to-end.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] Exactly one `fn base_type_name` in the crate
- [ ] `cargo fmt --all -- --check`, `cargo clippy -- -D warnings`, `cargo test --all` all exit 0
- [ ] `cargo run -- lint ../examples/` exits 0
- [ ] `git status` shows only the three in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- Any integration test fails after Step 2 — the abbreviated helper's different
  `Option<`/`[T]?` semantics was load-bearing for `actor_state_defaults_typed`;
  report which fixture and why instead of editing fixtures or re-forking the
  helper.
- `candy lint ../examples/` reports new violations — same root cause; report.

## Maintenance notes

- Future rules needing type-stripping should import this helper; a reviewer
  seeing a third local copy should point here.
- If the parser ever grows a real type AST (chumsky was recommended in early
  design notes but the current parser is hand-rolled), this string-level
  helper is the first thing the AST should replace.
