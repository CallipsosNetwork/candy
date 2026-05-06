# Project Retrospective Summary

Cumulative tracker across phases. Auto-updated by `/retro`. New phases are appended; existing rows are updated in place when their retro is regenerated.

---

## Phases

| Phase | Status | Confidence (C/Q/R) | Key outcome | Retrospective |
|---|---|---|---|---|
| M1 Foundation | Complete | 4 / 4 / 4 | Spec language + 7 examples + editor tooling. Foundation ready for codegen. | [phase-M1-foundation.md](phase-M1-foundation.md) |

C = Completeness, Q = Quality, R = Risk Exposure (rubric: 5 best, 1 worst).

---

## Cross-phase metrics (cumulative)

| Metric | M1 | Total |
|---|---|---|
| Commits | ~52 | ~52 |
| Merged PRs | 4 | 4 |
| Files touched | 173 | 173 |
| Lines added | ~59,000 | ~59,000 |
| Lines deleted | ~1,400 | ~1,400 |
| GitHub issues closed | ~14 | ~14 |
| Sub-agent waves | 3 | 3 |

---

## Recurring themes

- **Sub-agent self-reporting catches real bugs.** Three correctness issues in M1 were caught only because agents flagged judgment calls in their delivery reports. Future phase prompts should explicitly request this style of report.
- **Late-arriving design decisions are expensive.** Multiple M1 reframes (canonical auth, multi-provider externals, RBAC scope, minute-granularity) cost retroactive PR work. Counter-pattern: write a project-level scope/contract README BEFORE launching agents, not during.
- **Atomic commit discipline matters.** One bundled-commit slip in M1 required `git reset HEAD~1` to recover. `git add -A` is the smell.

---

## Cumulative risk register

| Risk | Phase introduced | Status | Likely close phase |
|---|---|---|---|
| `schedule` syntax untested by formal grammar review or codegen executor | M1 | Open — high priority | M2 (linter + codegen) |
| `subscribe` block-form syntax used but not formally in `GRAMMAR.md` | M1 | Open | M2 (small grammar PR) |
| Cross-example auth duplication will drift | M1 | Open — low priority | M3+ (cross-project imports) |
| Cross-target backend parity unverified | M1 | Open — by design | M2/M3 (codegen + eval-running) |
| Spec quality drift without a linter | M1 | Open | M2 (linter ships) |

---

## Project-level decisions log

| Decision | Phase | Status |
|---|---|---|
| Multi-provider externals via `Sketch B` (`providers:` field + `Actor[Tag]` selector) | M1 | Locked |
| Preferences stays as candy DSL (not TOML) | M1 | Locked |
| RBAC folded into `todo`; standalone `rbac` example deleted | M1 | Locked |
| airbnb is minute-granular (`TimeRange`, not `DateRange`) | M1 | Locked |
| Polar = canonical billing provider; Postmark = canonical email provider | M1 | Locked |
| Eval format: hybrid `.md` narrative + `.hurl` executable | M1 | Locked |
| Codegen prompts: thin base + per-target overlays leaning on language-best-practices skills | Decided post-M1 (in design discussion) | Provisional — first PR will validate |

---

*Stakeholder reports per phase: see `phase-<N>-<slug>-stakeholder.md`.*
