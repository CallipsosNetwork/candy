# Plan 004: Sync stale docs — README examples table, GRAMMAR.md case references, docs index, COVERAGE counts

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat e89c1d3..HEAD -- README.md docs/README.md evals/COVERAGE.md extensions/tree-sitter-candy/README.md extensions/neovim/README.md`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: docs
- **Planned at**: commit `e89c1d3`, 2026-06-12

## Why this matters

Four documentation drifts are actively wrong (worse than missing): the README
front door lists 3 of the 7 examples; four files reference `grammar.md`
lowercase while the file is `GRAMMAR.md` (broken on case-sensitive
filesystems — and the repo's own M1 retro records a grammar.md case-rename,
so the lowercase references are leftovers); the docs index omits 3 of its 6
docs; and COVERAGE.md's summary counts disagree with the actual hurl files.
COVERAGE.md is the conformance ledger — if its numbers can't be trusted,
neither can "green".

## Current state

1. **README examples table** — `README.md:70-82` lists only `hello.candy`,
   `todo`, `auth`. Actual `examples/` contents: `hello.candy` plus six
   directories: `auth`, `todo`, `wallet`, `billing`, `notifications`,
   `airbnb`. What each demonstrates (from NEXT.md and the specs):
   - wallet — admin/user split, scheduled transfers (TIME axis), integer-minor-unit money, journal as source of truth
   - billing — three schedules, Polar as canonical provider
   - notifications — multi-provider rescue chains, Postmark canonical
   - airbnb — multi-actor saga with compensation, multi-provider externals
2. **Lowercase `grammar.md` references** (file is `GRAMMAR.md` at repo root):
   - `README.md:42` — "See `grammar.md` for the full reference."
   - `README.md:88` — "...listed in full in `grammar.md`;"
   - `README.md:173` — "- `grammar.md` — full language reference..."
   - `docs/README.md:3` — "The grammar reference (`../grammar.md`)..."
   - `extensions/tree-sitter-candy/README.md:11` — "...defined in `grammar.md` at the repo root"
   - `extensions/neovim/README.md:4` — markdown link `[`grammar.md`](../../grammar.md)` — this one is a real relative link that 404s on case-sensitive checkouts.
   (`.retrospective/phase-M1-foundation.md:118` also mentions "grammar.md
   case-rename" — that is a historical record; do NOT edit retrospectives.)
3. **docs index** — `docs/README.md` lists `architecture.md`, `features.md`,
   `externals.md` but the directory also contains `candy-toml.md`,
   `cli-modes.md`, `testing-strategy.md`. One-line purposes:
   - candy-toml.md — the `candy.toml` per-example config schema
   - cli-modes.md — green (fresh) vs brown (existing-project) codegen modes
   - testing-strategy.md — hurl as cross-target reference; per-target native suites as the goal
4. **COVERAGE.md summary table** — `evals/COVERAGE.md:241-251`. Verified
   mismatches against `grep -c '^# ===' evals/<f>/<f>.hurl`:
   - todo: table says 31, actual 33
   - notifications: table says 12, actual 13
   Other rows were not individually verified — recount all of them.

Convention: prose style in these docs is terse and declarative; tables use the
existing column shapes. Match them.

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Recount hurl scenarios | `for d in auth todo wallet billing notifications; do echo "$d: $(grep -c '^# ===' evals/$d/$d.hurl)"; done; for f in evals/airbnb/*.hurl; do echo "$f: $(grep -c '^# ===' $f)"; done` | prints true counts |
| Case-reference sweep | `grep -rn 'grammar\.md' --include='*.md' . \| grep -v GRAMMAR \| grep -v retrospective \| grep -v plans/` | empty after fix |

## Scope

**In scope**:
- `README.md` (examples table + 3 case references)
- `docs/README.md` (3 new index entries + 1 case reference)
- `extensions/tree-sitter-candy/README.md`, `extensions/neovim/README.md` (case references only)
- `evals/COVERAGE.md` (summary-table counts only)

**Out of scope** (do NOT touch):
- `.retrospective/` — historical records stay verbatim.
- `evals/*/*.hurl` and per-feature checklist sections of COVERAGE.md — if a
  checklist contradicts a hurl file, that's a finding to report, not a doc to
  silently rewrite; this plan fixes only the summary-table counts.
- `GRAMMAR.md` content, `NEXT.md`, `CLAUDE.md`.
- Keyword lists in `extensions/` — only the README text references, nothing in
  grammar/highlighting files.

## Git workflow

- Branch: `docs/sync-readme-coverage`
- Atomic conventional commits, e.g. `docs(readme): list all seven examples`,
  `docs: fix grammar.md case references`, `docs(evals): recount COVERAGE summary`.
  No `git add -A`, no AI co-author footers.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Expand the README examples table

Extend the table at `README.md:74-79` to all seven examples using the
one-liners in "Current state". Keep the "Read them in that order" guidance by
ordering: hello → todo → auth → wallet → billing → notifications → airbnb
(trivial to realistic, matching how the repo describes them).

**Verify**: `grep -c 'examples/' README.md` increased by 4 (one row per new
example); table renders (pipe-count per row matches header).

### Step 2: Fix the case references

Replace `grammar.md` with `GRAMMAR.md` at the six locations in "Current
state" item 2 (and fix the neovim link path to `../../GRAMMAR.md`).

**Verify**: `grep -rn 'grammar\.md' --include='*.md' . | grep -v GRAMMAR | grep -v retrospective | grep -v plans/` → no output.

### Step 3: Complete the docs index

Add `candy-toml.md`, `cli-modes.md`, `testing-strategy.md` entries to
`docs/README.md`, matching the existing `- [name](name) — description` bullet
format.

**Verify**: `for f in docs/*.md; do b=$(basename $f); [ "$b" = README.md ] || grep -q "$b" docs/README.md || echo "MISSING $b"; done` → no output.

### Step 4: Recount the COVERAGE summary

Run the recount command, update every row of the summary table at
`evals/COVERAGE.md:241-251` whose "Hurl scenarios" number differs. Touch only
that column; leave `[x]`/`[d]`/`[~]` counts and "Verified target(s)" alone.

**Verify**: re-run the recount; every printed count equals the table's number
for that row.

## Test plan

Doc-only change; the verification greps in each step are the tests. Also run
`cd cli && cargo run -- lint ../examples/` → exit 0, to prove nothing
non-doc was touched.

## Done criteria

Machine-checkable. ALL must hold:

- [ ] README examples table has 7 rows
- [ ] Case-reference sweep grep → empty
- [ ] docs/README.md indexes all 6 sibling docs
- [ ] COVERAGE summary counts match `grep -c '^# ==='` for every row
- [ ] `git status` shows only the six in-scope files modified
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- A recount reveals a summary row off by more than ~5 — that suggests the
  per-feature checklist sections are also stale, which is beyond this plan's
  summary-only scope; report the row.
- The airbnb rows don't map cleanly onto hurl files (airbnb has four hurl
  files: auth, listings, booking, coupons) — recount per file; if COVERAGE's
  airbnb rows use a different breakdown, report rather than guess.
- Any fix seems to require editing a `.hurl` file or a retrospective.

## Maintenance notes

- COVERAGE.md counts drift every time scenarios are added (plan 003 adds two
  to auth — if it lands first, the auth row should already say 16; recount
  regardless). A future `candy test` (NEXT.md item 3) could emit these counts
  mechanically; until then they're hand-maintained.
- Reviewer scrutiny: that the new example one-liners don't overclaim (billing/
  notifications/airbnb have no generated targets yet — describe the specs,
  not "working backends").
