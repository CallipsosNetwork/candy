# Plan 001: One-command conformance runner for generated backends, wired into CI

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat e89c1d3..HEAD -- .github/workflows/ci.yml scripts/ evals/README.md`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `e89c1d3`, 2026-06-12

## Why this matters

The repo has 99 hurl conformance scenarios and four generated backends that are
declared green (auth/Go, auth/Rust, todo/Go, wallet/Go), but nothing runs them
automatically. CI (`.github/workflows/ci.yml`) only checks the Rust linter CLI —
the generated backends are never even compiled on a PR. A regression in a spec,
a prompt, or a generated tree is invisible until someone manually starts a
server and runs hurl. This plan creates `scripts/run-evals.sh` (one command to
run any feature's hurl suite against any built target) and adds a CI job that
runs the four green suites on every PR. Plans 002, 003, and 005 use this script
as their verification gate.

## Current state

- `.github/workflows/ci.yml` — the only CI workflow that runs on PRs. Its
  single job `lint-and-test` (working-directory `cli`) runs: `cargo fmt --all
  -- --check`, `cargo clippy -- -D warnings`, `cargo test --all`, and
  `cargo run -- lint ../examples/` (lines 30–40). No Go toolchain, no hurl, no
  backend builds.
- `evals/README.md:28-48` — documents the manual invocation:

  ```sh
  hurl --variables-file evals/auth/fixtures.env \
       --variable BASE_URL=http://localhost:8080 \
       evals/auth/auth.hurl
  ```

  and states: "Backends are expected to be reachable at `BASE_URL` and to
  start with empty state — every test bootstraps the actors it needs."
- Backend start contracts (verified by reading each `cmd/server/main.go` /
  `src/main.rs`; all read the same three env vars):

  | Backend | Start command (from its target dir) | Env vars (all have dev defaults) |
  |---|---|---|
  | `examples/auth/targets/go` | `go run ./cmd/server` | `PORT` (8080), `DB_PATH` (/tmp/auth-dev.db), `JWT_SECRET` |
  | `examples/auth/targets/rust` | `cargo run` | `PORT` (8080), `DB_PATH` (/tmp/auth-dev.db), `JWT_SECRET` |
  | `examples/todo/targets/go` | `go run ./cmd/server` | `PORT` (8080), `DB_PATH` (/tmp/todo.db), `JWT_SECRET` |
  | `examples/wallet/targets/go` | `go run ./cmd/server` | `PORT` (8080), `DB_PATH` (/tmp/wallet.db), `JWT_SECRET` |

- `examples/auth/targets/go/scripts/run.sh` already exists but hardcodes
  `/usr/local/go/bin/go` (non-portable) and covers only one backend. Do not
  reuse it; the new script supersedes it for eval purposes (leave it in place —
  it is part of a generated tree).
- Eval suites that must pass today: `evals/auth/auth.hurl` (vs auth go + rust),
  `evals/todo/todo.hurl` (vs todo go), `evals/wallet/wallet.hurl` (vs wallet
  go). Each has a sibling `fixtures.env` in the same directory. The other
  suites (billing, notifications, airbnb/*) have no generated backend yet —
  out of scope.
- Repo conventions: bash scripts use `set -euo pipefail` (see
  `examples/auth/targets/go/scripts/run.sh`). Commit style is conventional
  commits, e.g. `feat(evals): ...` — see `git log --oneline`.

## Commands you will need

| Purpose | Command | Expected on success |
|---|---|---|
| Build Go backend | `cd examples/<f>/targets/go && go build ./...` | exit 0 |
| Build Rust backend | `cd examples/auth/targets/rust && cargo build` | exit 0 |
| Run a hurl suite | `hurl --variables-file evals/<f>/fixtures.env --variable BASE_URL=http://localhost:<port> evals/<f>/<f>.hurl` | exit 0, all asserts pass |
| Hurl present | `hurl --version` | prints 4.x or later |
| CLI checks still green | `cd cli && cargo test --all` | 18 tests pass |

## Scope

**In scope** (the only files you should create or modify):
- `scripts/run-evals.sh` (create; `scripts/` directory at repo root does not exist yet — create it)
- `.github/workflows/ci.yml` (add a job; do not modify the existing `lint-and-test` job)
- `evals/README.md` (replace the pseudocode "for target in ..." block in "How to run" with the real script invocation)

**Out of scope** (do NOT touch):
- Anything under `examples/*/targets/` — generated trees; this plan only reads/builds them.
- `evals/*/*.hurl` and `fixtures.env` — if a suite fails, that is a finding to report, not a fixture to edit (repo rule: "Don't modify the spec or fixture to make a test pass").
- The Rust CLI (`cli/`) — `candy test` as a subcommand is a separate roadmap item (NEXT.md item 3); this script is the interim harness, not the CLI feature.
- `.github/workflows/release.yml`.

## Git workflow

- Branch: `feat/eval-runner-script` (repo convention: `feat/` prefix, branch off `main`)
- Conventional commits, atomic: one commit for the script, one for the CI job, one for the README update. No `git add -A`. No AI co-author footers.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Write `scripts/run-evals.sh`

Create an executable bash script (`chmod +x`) with this contract:

```
Usage: scripts/run-evals.sh <feature> <target> [port]
       scripts/run-evals.sh --all
  <feature>: auth | todo | wallet
  <target>:  go | rust
  --all:     runs the four green pairs: auth/go, auth/rust, todo/go, wallet/go
```

Behavior for a single `<feature> <target>` run:

1. Resolve repo root from the script's own location (`cd "$(dirname "${BASH_SOURCE[0]}")/.."`).
2. Fail fast with a clear message if `hurl` is not on PATH, or if
   `examples/<feature>/targets/<target>` does not exist.
3. Pick the port: `$3` if given, else default 8765 (NOT 8080 — avoid colliding
   with a dev server the user may have running).
4. Create a fresh temp DB path per run (`mktemp -u /tmp/candy-eval-<feature>-XXXXXX.db`) —
   the contract requires empty state.
5. Start the backend in the background with `PORT`, `DB_PATH`, `JWT_SECRET=eval-secret`
   exported. Go: `go run ./cmd/server` from the target dir. Rust: `cargo run`
   from the target dir (build output may take minutes on first run — that's fine).
   Record the PID; install a `trap` that kills the process group and removes the
   temp DB on EXIT.
6. Wait for readiness: poll `curl -s -o /dev/null http://localhost:$PORT/` in a
   loop (any HTTP response counts, including 404 — the routes are mostly POST);
   timeout after 120s with a clear failure message.
7. Run: `hurl --variables-file evals/<feature>/fixtures.env --variable BASE_URL=http://localhost:$PORT evals/<feature>/<feature>.hurl --test`
8. Propagate hurl's exit code as the script's exit code. Print a one-line
   `PASS <feature>/<target>` or `FAIL <feature>/<target>` summary.

`--all` runs the four pairs sequentially and exits non-zero if any failed,
printing a final summary table.

Use `set -euo pipefail`. Use `go` and `cargo` from PATH (do not hardcode
toolchain paths).

**Verify**: `bash -n scripts/run-evals.sh` → exit 0 (syntax OK), and
`scripts/run-evals.sh auth go` → ends with `PASS auth/go`, exit 0.

### Step 2: Verify all four green pairs locally

**Verify**: `scripts/run-evals.sh --all` → all four lines `PASS`, exit 0.
If any suite fails, STOP (see STOP conditions) — do not edit fixtures, specs,
or generated code.

### Step 3: Add the `conformance` job to CI

Append a second job to `.github/workflows/ci.yml` (keep `lint-and-test`
untouched):

```yaml
  conformance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: 'stable'
          cache-dependency-path: examples/*/targets/go/go.sum

      - name: Install Rust toolchain
        uses: dtolnay/rust-toolchain@stable

      - name: Cache cargo (auth rust target)
        uses: actions/cache@v4
        with:
          path: |
            ~/.cargo/registry
            ~/.cargo/git
            examples/auth/targets/rust/target
          key: ${{ runner.os }}-cargo-authrust-${{ hashFiles('examples/auth/targets/rust/Cargo.lock') }}

      - name: Install hurl
        run: |
          curl -sLO https://github.com/Orange-OpenSource/hurl/releases/download/4.3.0/hurl_4.3.0_amd64.deb
          sudo dpkg -i hurl_4.3.0_amd64.deb

      - name: Run conformance evals
        run: scripts/run-evals.sh --all
```

(If a newer hurl 4.x/5.x version is current, use it — anything ≥ 4.x per
`evals/README.md:46`.)

**Verify**: `bash -n scripts/run-evals.sh` still exits 0, and the workflow file
parses: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml'))"` → exit 0.

### Step 4: Update `evals/README.md` "How to run"

Replace the hypothetical loop (lines 36–43, the `for target in go rust
typescript python; do start_backend ...` block — it references shell functions
that do not exist) with the real commands:

```sh
# One feature against one target:
scripts/run-evals.sh auth go

# Everything that has a generated backend:
scripts/run-evals.sh --all
```

Keep the single-backend manual `hurl` invocation above it (lines 30–34) — it
documents the underlying contract.

**Verify**: `grep -n "run-evals.sh" evals/README.md` → at least 2 matches;
`grep -n "start_backend" evals/README.md` → no matches.

## Test plan

The script is itself test infrastructure; its test is Step 2 (`--all` green).
Also exercise one failure path: run `scripts/run-evals.sh auth go 1` (port 1 is
unbindable) and confirm the script fails with the readiness-timeout message and
a non-zero exit rather than hanging — then nothing is left running
(`pgrep -f "cmd/server"` → no output).

## Done criteria

Machine-checkable. ALL must hold:

- [ ] `scripts/run-evals.sh --all` exits 0 with four `PASS` lines
- [ ] After any script run, no orphan servers: `pgrep -f "cmd/server"` → empty
- [ ] `.github/workflows/ci.yml` contains both jobs `lint-and-test` (unchanged) and `conformance`
- [ ] `cd cli && cargo test --all` → 18 passed (untouched, still green)
- [ ] `git status` shows changes only to the three in-scope files
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- Any of the four suites FAILS against its current backend. The repo's
  COVERAGE.md says these are green; a failure means either environment trouble
  or a real regression — surface it, don't patch fixtures/specs/targets.
- A backend fails to compile (`go build` / `cargo build` non-zero). Generated
  trees must not be hand-edited; report which one and the error.
- Readiness polling needs more than the 120s timeout even after one retry
  (likely first-run `cargo build` — extend once to 300s for rust; if still
  failing, stop).
- The wallet suite hangs on its schedule scenarios for more than ~5 minutes
  (the suite includes sleep-past-firing-window steps; some waiting is normal —
  see `evals/README.md:78-83`).

## Maintenance notes

- NEXT.md item 3 (`candy test` Rust subcommand) is the long-term home for this
  logic. When that lands, the CI job should swap `scripts/run-evals.sh --all`
  for `candy test` and the script can be deleted. Keep the script thin so
  nothing grows attached to it.
- When a new `examples/<f>/targets/<lang>` goes green, add the pair to the
  `--all` list and to COVERAGE.md's "Verified target(s)" column.
- Reviewer scrutiny: the trap/cleanup path (no orphaned servers in CI), and
  that the CI job does not silently skip a pair.
