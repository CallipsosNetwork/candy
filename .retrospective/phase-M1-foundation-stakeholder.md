# M1 Stakeholder Report — Foundation Phase

**Period:** 2026-04-29 → 2026-05-06
**Status:** Complete

---

## What we shipped

A candy spec language and 7 example projects that show what real backends look like when described in candy.

- **Language reference** with ~50 keywords across 5 word-axes.
- **7 examples**: hello (smallest), auth (the JWT reference), todo (with role-based access), wallet (money + scheduled transfers), billing (subscription with retries), notifications (multi-provider email/SMS), airbnb (the full marketplace, 8 spec files).
- **Editor support**: Neovim and VS Code highlighters; tree-sitter grammar skeleton.
- **Documentation**: architecture, externals, features, manifest schema.

---

## Numbers

- 4 GitHub PRs merged.
- ~59,000 lines of specs, docs, and tooling added.
- ~14 GitHub issues closed.
- ~14 sub-agent runs across 3 parallel waves.

---

## Confidence

| Dimension | Score (1-5) |
|---|---|
| Did we deliver what was scoped? | **4 / 5** |
| Quality of what shipped | **4 / 5** |
| Risk that something will bite us in M2 | **4 / 5** |

The "4 not 5" reflects: an honest acknowledgment that the eval framework (the next deliverable, PR #34) is in flight, and that the `schedule` keyword and a linter haven't been formally pressure-tested yet.

---

## What worked

- **Parallel AI sub-agents** with a contract-first README pattern. We ran 4–5 agents simultaneously to write airbnb's 8 spec files and four other examples, with internally consistent results.
- **Atomic commits per issue** keep the git history readable.
- **Sub-agents self-reporting their judgment calls** caught three real bugs at review that would have shipped otherwise.

---

## What we'd do differently — deferred

The thorough "what didn't work" assessment is held until PR #34 (eval framework) merges. Real eval pressure on the spec quality is the right time to make that call.

One thing already known: we shipped 5 examples without inlined auth, then retrofitted all of them in one PR. If we'd recognized "every role-gated example needs auth" at M1's start instead of mid-stream, that PR would have been smaller.

---

## Risks heading into M2

| Risk | What we're doing about it |
|---|---|
| `schedule` keyword has only been tested by example use, not by codegen running it | Build a candy linter (catch syntax errors); build a per-target codegen executor for schedules |
| Spec drift across new examples without enforcement | Linter ships early in M2 |
| Cross-target backend behavior parity unverified | Eval framework (PR #34) defines the contract; codegen wave validates against it |

---

## Next phase preview (M2)

- Eval framework merges.
- Codegen prompt authoring.
- Candy linter (new artifact).
- First Go/chi backend generated from `examples/auth/` and run against `evals/auth/auth.hurl`.

**Readiness: High** for the prompt-authoring path. **Medium** for actual backend generation, since prompt quality has yet to be empirically tested.
