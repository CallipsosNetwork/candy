# candy CLI — green and brown modes

Codegen has **two modes of operation** that share a spec language but
have very different semantics around code ownership, edit ergonomics,
and what "regenerate" means. Today the alpha proves only the first;
the second is an open design problem and is the most important thing
to get right for adoption.

---

## Mode 1 — green (spec → fresh code)

Status: **proven** by the alpha (auth/Go, auth/Rust, todo/Go,
wallet/Go).

The codegen owns the entire `targets/<lang>/` tree. The candy spec
is the source of truth; the generated code is reproducible from the
spec. Humans don't edit `targets/<lang>/`; if a behaviour is wrong,
the spec or the codegen prompts are the fix, not the file.

| Property                    | Behaviour                                           |
|-----------------------------|-----------------------------------------------------|
| Source of truth             | The spec. Generated code is derivative.             |
| First codegen               | `targets/<lang>/` doesn't exist. Codegen creates it.|
| Re-codegen on spec change   | Tree is overwritten in place. Stable orderings minimise diff churn. |
| Re-codegen with no spec change | Idempotent (modulo timestamp-free, hash-stable orderings). Diff should be empty. |
| Drift                       | Not allowed. Generated files start with a "do not edit — regenerate from spec" header. |
| Conflict resolution         | None. The spec wins, period.                        |
| Test loop                   | Spec change → regenerate → run hurl → done.         |

This is the "greenfield" workflow. It maps cleanly onto the
spec-as-contract metaphor: the candy file is the program, the
generated code is the artifact.

The whole alpha pipeline is green-mode.

---

## Mode 2 — brown (spec → existing code)

Status: **unproven**. No code in candy assumes this mode.

The user already has a Go (or Rust, TS, Python) project — built by
hand or by some other framework. They want candy to apply to
**part** of it, or to migrate the whole thing onto candy semantics
without throwing away the current code. This is the realistic
adoption path: most teams have an existing codebase and won't
rewrite it from scratch to use candy.

**The success metric is the diff between the spec and the
edit/refine workflow.** Concretely: when the spec changes, or the
code changes, or both — how disruptive is the next round of
codegen? Small spec change → small code diff. Code change with no
spec change → either a clean preservation of human edits, or a
surfaced conflict the user can resolve. Spec change AND code change
→ a structured three-way merge.

If brownfield mode is too disruptive, no one will adopt candy on
existing code, and candy is forever a green-only tool. If it's
clean, candy becomes a refactor partner — *the* metric to chase.

### Design space — sketches, not commitments

| Sketch | Description | Ownership | Diff cost on spec change | Conflict surface |
|--------|-------------|-----------|---------------------------|-------------------|
| **B1 — Replacement** | Codegen overwrites `targets/<lang>/` regardless of human edits | Codegen owns everything (same as green) | Zero — spec is the truth | None — human edits lost |
| **B2 — Three-way merge** | Codegen produces "expected"; merge against current using last codegen as base | Shared. Human-owned regions marked. | Small if merge succeeds; surfaces conflicts when both sides changed the same region | Conflict markers in diff; user resolves |
| **B3 — Adapter mode** | Codegen produces adapter modules around existing code; existing code stays put | Codegen owns adapters; humans own original modules | Small — only adapters change | None on the original code; conflicts only inside adapters |
| **B4 — Refactor mode** | Codegen produces patches against current code, guided by mappings (User actor → existing User class) | Shared via mappings | Variable — depends on mapping fidelity | Patch-rejection if mapping target moved; user re-points the mapping |

(B2) is the most general. (B3) is the most pragmatic for adoption.
(B4) is the most ambitious — it assumes the codegen can express
its output as a patch against arbitrary code, which is hard.

### Hard sub-problems

- **Mapping detection.** How does candy know that spec entity
  `actor User` corresponds to the existing `models/user.go`
  struct? Options:
  - Explicit user-provided mapping file
    (`candy.brown.toml`?)
  - Heuristic detection (name match, structure match, route match)
  - LLM-driven matching pass
  - All three composed
- **Stable identity for spec entities.** Every actor, flow,
  controller needs a stable identifier so that on re-codegen, candy
  finds the right target module even if the user renamed it. The
  spec name (`actor User`) is the obvious key, but it's user-
  controlled and can drift.
- **Drift handling.** Between codegen runs, the user edits the
  generated/adapter code. Re-codegen needs to either (a) preserve
  the edits, (b) detect them and surface, or (c) refuse to overwrite
  without explicit confirmation. Last-codegen baseline (B2's three-
  way merge) is the cleanest mechanism.
- **Conflict UI.** When codegen disagrees with current code, the
  user needs to see the conflict in a way that's resolvable
  without re-running codegen. Inline diff markers? A separate
  conflict file? An interactive `candy resolve` subcommand?
- **Round-tripping.** If candy's brown mode produces patches and
  the user accepts them, can the result be regenerated from the
  spec alone? Probably not for B3/B4 — adapter mode and refactor
  mode produce code dependent on the existing surrounding code.

### Related work to learn from

- **Prisma's `db pull` / `db push` / `migrate dev`.** Schema-first
  workflow against existing databases. The migration model is the
  closest analogue. Lesson: the diff between schema and current
  state IS the workflow's primary surface.
- **GraphQL codegen with watch mode.** Generates client/server
  bindings on schema change. Lesson: regeneration must be cheap and
  incremental; full overwrites kill the workflow.
- **`rust-analyzer` and other LSP refactor tools.** Patch-style
  application against existing code. Lesson: structural editing of
  arbitrary code requires AST-level understanding; string-level
  edits drift.
- **Cursor Composer / GH Copilot multi-file edits.** LLM-produced
  patches against existing code. Lesson: the LLM doesn't need to
  own the file; it needs to express its intent as a coherent diff
  that the user accepts or rejects.

### Related candy work

- **Issue #38** — `[strategy] candy bootstrapper for existing
  projects`. The **inverse** direction: introspect existing code,
  produce a starter `.candy` spec. Useful as a brownfield onramp.
- **This doc** is the spec → existing code direction.

---

## What must be true before brown mode lands

1. **Green mode is rock-solid.** Alpha proves the spec-to-code path
   for ≥1 target per example. Brown mode is harder; building it on
   wobbly green is malpractice.
2. **The eval contract carries through.** A brown-applied codebase
   must still pass the relevant `evals/<feature>/<feature>.hurl`.
   That's how we know the application worked, not just that the
   files compile.
3. **Last-codegen baseline.** The toolchain must record what it
   produced last time. Without a baseline, three-way merge is
   impossible. Likely a `.candy/snapshot/<commit>` directory.
4. **Mapping language.** Mappings need to be expressible in candy
   itself or in a small adjunct file. Probably a new top-level
   block: `mapping <SpecEntity> -> <existing module path>`.

## Proposed next actions

- **Now (this PR):** This document. No code changes.
- **Post-alpha tracker:** File a new issue
  `[strategy] candy CLI brown mode — spec applied to existing code`
  citing this document. Sibling to issue #38.
- **First experiment:** Once `candy gen` (the green CLI subcommand)
  ships, prototype B3 (adapter mode) by hand on a small Go project
  — generate just the controller layer as adapters around existing
  service code. See what the diff looks like.
- **Second experiment:** Try B2 (three-way merge) on the same
  project after a manual edit. Measure diff size on a no-op spec
  change.

The metric — **how much does the code diff move when the spec
moves slightly?** — is the test for whether candy is a tool people
actually want on a real codebase.

---

## Refs

- `prompts/codegen-base.md` — generation contract for green mode.
- `evals/README.md` — conformance contract that must hold under
  both modes.
- Issue #38 — code-to-spec direction (inverse).
- `.retrospective/phase-alpha-codegen.md` — alpha proves green
  mode.
