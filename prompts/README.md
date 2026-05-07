# prompts/

Codegen system prompts. The contract by which a candy spec becomes an
idiomatic backend in Go, Rust, TypeScript, or Python.

## Structure

```
prompts/
  README.md                  — this file.
  codegen-base.md            — universal codegen contract.
  codegen-go.md              — Go/chi overlay.
  codegen-rust.md            — Rust/axum overlay.
  codegen-typescript.md      — TypeScript/hono overlay.
  codegen-python.md          — Python/fastapi overlay.
```

The base prompt is universal: block-by-block translation contract,
cross-cutting rules, what to refuse. The four overlays are thin —
target framework wiring, library bindings, idiomatic shape per block.

Two of the overlays (Go, Rust) explicitly dispatch to existing Claude
Code skills (`golang-best-practices`, `rust-best-practices`) for
idiom guidance. The other two (TypeScript, Python) carry more inline
idiom because no equivalent skill ships in this repo's environment.

## Loading order at codegen time

1. `GRAMMAR.md` (root of repo).
2. `prompts/codegen-base.md`.
3. `prompts/codegen-<target>.md`.
4. The skill the overlay names, if any.
5. The project's `candy.toml`, `preferences.candy`, and every `.candy`
   file under it.

## How they're invoked

The candy CLI's `gen` subcommand (Phase C onward; see issues
#13/#14/#15/#16) loads these prompts plus the project spec and asks an
LLM to produce the target tree. v0.1 is prompt-driven; the prompts
themselves can also be loaded by hand into a Claude Code session for
generation.

## Versioning

`candy.toml`'s `[deps]` table currently informational; the entry
`"candy/codegen-go" = "0.1"` (and equivalent) names this prompt bundle
at version 0.1. Bumping the version is a contract change — every
example must regenerate cleanly under the new prompt.

## Conformance

Every overlay's "Verification before reporting done" section is the
acceptance gate. A target is generated correctly when:

- The chosen target's lint/typecheck/test commands all pass.
- The corresponding `evals/<feature>/<feature>.hurl` runs green
  against the generated server.

Don't edit hurl files to make them green; the spec mapping is the bug.

## Refs

- Phase A of `.claude/session-handoff.txt` (closed by this PR; closes #12).
- Decision context: GitHub issue #36 (closed) — codegen architecture
  decisions: option (c) base-plus-thin-overlays-with-skill-dispatch.
- `evals/README.md` — the conformance contract.
- `GRAMMAR.md` — the language reference.
