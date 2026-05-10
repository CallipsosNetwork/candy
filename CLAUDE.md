# CLAUDE.md — candy

Project-level context Claude Code loads on every session in this repo.

## Read first, in order

1. **`NEXT.md`** — the menu of next-step targets. Always check this first when starting a session in this repo; it tells you what to work on and what's locked.
2. **`.claude/session-handoff.txt`** — the running handoff between sessions. Active phase status, locked decisions, open items.
3. **`README.md`** — project overview.
4. **`GRAMMAR.md`** — the candy language reference.
5. **`evals/README.md`** — the conformance contract (hurl is the cross-target reference for now; per-target native suites planned — see `docs/testing-strategy.md`).

## Key directories

| Path                                  | Purpose                                                                                                                                   |
|---------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| `prompts/`                            | Codegen prompts (base + 4 target overlays).                                                                                               |
| `examples/<feature>/`                 | Canonical example specs.                                                                                                                  |
| `examples/<feature>/targets/<lang>/`  | Generated backends. Do not hand-edit; regenerate from spec.                                                                               |
| `commons/types/`                      | Canonical type specs (Email, Hash, Money, Token, Password, Phone). Provisional `spec` syntax pending linter integration.                  |
| `evals/<feature>/`                    | hurl scenarios + fixtures + .md narratives.                                                                                               |
| `cli/`                                | Rust `candy` CLI (currently: `candy lint`).                                                                                               |
| `docs/`                               | Architecture, externals, features, candy.toml schema, cli-modes (green/brown), testing-strategy.                                         |
| `.retrospective/`                     | Per-phase retrospectives.                                                                                                                 |

## Standing rules — surface deviations

- **Honour `preferences.candy` exactly.** Sub-agents have repeatedly tried to substitute a "simpler" implementation (KSUID-in-DB instead of JWT) when the spec/preferences pin a specific library. Don't.
- **Don't modify the spec or fixture to make a test pass.** When the eval fails, find the wrong side. Usually the fixture is wrong; sometimes the spec is ambiguous and needs ratification. Surface the conflict, don't silence it.
- **Atomic commits per logical change.** No `git add -A`.
- **No AI co-author footer.** The project disables it.
- **Generated source files start with the codegen header.** "Generated from spec/<path> — do not edit; regenerate from spec." Humans regenerate; they don't edit `targets/<lang>/`.

## When in doubt

- Spec ambiguity: read GRAMMAR.md and the relevant example's `.candy` files; flag in HANDOFF if still unclear.
- Locked decision conflict: `.claude/session-handoff.txt` §7 lists what's locked. Don't relitigate without strong reason.
- New phase: write a HANDOFF.md inside the generated tree as the running judgment-call log.
- Stuck after two attempts: stop and surface to the user. Don't loop.
