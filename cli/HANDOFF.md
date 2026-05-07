# CLI Phase B — Judgment Call Report

This file records every non-obvious decision made while building the v0.1 linter.
It exists so the next developer can audit the gap between "what GRAMMAR.md says"
and "what the linter actually checks."

---

## 1. Rules where semantics were invented

### `idempotency-key`

GRAMMAR.md says "replayable messages declare a `key: Key` parameter." It does not define
what makes a message "replayable." The implementation interprets this as: any `flow` block
that contains an `ask <ExternalActor>.<Op>(...)` call, where the callee is declared as
`external actor` somewhere in the project, and the flow lacks a `key: Key` parameter.
Scope limited to `flow` blocks; `accepts` on internal actors are not checked (too noisy,
and the grammar does not give a clear boundary). See also: `idempotency_key.rs` L9-12.

### `underscores-in-keywords`

The grammar rule "no underscores in keywords" is not precisely defined. Implemented as:
block-level names and `accepts` message names must not contain `_`. Field names inside
record types and state field names are not flagged (snake_case fields appear throughout
the examples). Left as `TODO(underscores)` in `underscores_in_keywords.rs` L16-17 if
GRAMMAR.md later clarifies.

### `schedule-syntax-valid` — duration regex

GRAMMAR.md does not specify valid duration literal syntax. Invented rule:
`\d+(d|h|m|s|ms)` (digits followed by d, h, m, s, or ms). The examples use `1m`, `15s`,
`24h`, all of which match. This regex is checked in `parse_schedule_declaration()` in
`parser.rs`.

### `schedule-syntax-valid` — `for any` clause

GRAMMAR.md shows `for any X in Y where ...` in schedule examples. The rule flags a
schedule that is missing a `for` keyword entirely. It does not validate the `X in Y
where ...` body — that is left as a future concern.

### Feature name derivation for `broken-cross-feature-ref`

GRAMMAR.md does not specify how to derive a feature name from a file path. Invented rule
(in `feature_name_from_path()`, `parser.rs` L245-257): if the filename is `prose.candy`,
use the parent directory name; otherwise, use the file stem. So `examples/auth/auth.candy`
→ feature `auth`, and `examples/auth/prose.candy` would also → feature `auth`.

### Built-in type resolution set

The linter must not false-positive on types like `Id`, `Timestamp`, `Money`, `Key` that
appear in the examples but are never declared as `type` blocks. These are hardcoded as
always-resolved in `Project::declared_names()` (`parser.rs` L204-221). The list was
assembled empirically from the examples. Any type name not in this set and not declared
in the project will be flagged by `broken-symbol-ref` / `event-payload-types-resolve`.

---

## 2. Parser intentional under-strictness

| Area | What the parser accepts | What it skips |
|------|------------------------|---------------|
| Flow params / return type | Anything between `(` and the opening `{` of the flow body | No validation of type syntax |
| Triple-quoted strings `"""..."""` | Consumed verbatim, no brace counting inside | Content is opaque |
| `external actor` body | Detects `accepts` blocks and `ask`/`tell` calls | `providers:` list is collected but not cross-checked |
| `enum` bodies | Collected as `BlockKind::Enum`, names tracked | Variant values not parsed |
| `invariant` / `target` bodies | Collected as blocks, names tracked | Body contents ignored |
| `controller` bodies | Collected as blocks, names tracked | Body contents ignored |
| Record type bodies (`type Foo { ... }`) | `has_float` is set if the literal word `float` appears | Field names not indexed |
| `actor` state fields | `name`, `type`, `default` parsed for one-liner `name: Type = default` | Multi-line defaults not handled |
| `at <expr>` schedules | Presence of `at` keyword triggers `has_every_or_at = true` | The `<expr>` is not validated |
| `uses:` multi-op lines | `feature X for OpA, OpB` split on comma | Works only when ops are on one line |

Recovery strategy: on unrecognised syntax inside a block body, the parser skips to the
matching closing `}` and continues. This means a malformed file produces fewer violations,
not a hard error.

---

## 3. TODOs left in code

| File | Line | Note |
|------|------|------|
| `src/lint/rules/underscores_in_keywords.rs` | 16 | `TODO(underscores)`: revisit whether payload field names should be flagged once GRAMMAR.md clarifies |
| `src/lint/parser.rs` | 8-13 | `#![allow(dead_code, ...)]` — several AST fields are collected for future rules (`type_refs`, `providers`, `raw_params`, `AcceptsDecl::external_calls`) but not consumed by any v0.1 rule |

---

## 4. Platforms and features skipped

- **`candy gen`** — stub only, exits 1 with "see issue #13". No code generation.
- **`candy test`** — stub only, exits 1 with "see issue #17". No spec test runner.
- **`candy fmt`** — stub only, exits 1 with "see issue #39". No formatter.
- **`linux-arm32`** — not in the release matrix. Only aarch64 is supported for Linux ARM.
- **`darwin-arm64` cross compilation** — uses `aarch64-apple-darwin` target on `macos-latest`
  runner natively (Apple Silicon runners), not via `cross`. This differs from linux-aarch64
  which uses the `cross` tool because GitHub's Linux runners are x86-only.
- **Windows ARM** — not in the release matrix.

---

## 5. Surprises found in `examples/*/`

### `auth.candy` — cross-file policy reference
`auth.candy` has `policies: [BearerAuth]` but `BearerAuth` is declared in `todo.candy`.
This is an intentional cross-file dependency. The `broken-symbol-ref` and
`policy-attachment-resolves` rules are therefore only enabled in project mode (directory
lint), not single-file lint, to avoid false positives. This was the main design driver for
the project-mode flag.

### `wallet.candy` — generic return types in flow signatures
`flow ScheduleTransfer(...) -> Result<{ schedule: Id }, ...>` contains a `{` inside a
generic type argument. The naive parser broke here because `{` at any depth triggered the
start of the block body. Fix required tracking `paren_depth`, `angle_depth`, and
`brace_depth` separately and only treating `{` as the block opener when all three are zero
and the parameter list has been consumed. See `read_params_and_return_line()` in
`parser.rs`.

### `wallet.candy` — multi-op `uses:` lines
`uses: feature listings for HoldSlot, ReleaseSlot` lists two ops on one line. The initial
implementation parsed the whole `"HoldSlot, ReleaseSlot"` string as a single op name.
Fix: `parse_uses_line()` now splits the op list on `,` and emits one `UsesDecl` per op.

### `auth.candy` — inline event payload syntax
Events like `event LoginSucceeded { payload: { user: Id, at: Timestamp }, delivery: eager }`
are written entirely on one line. The block body parser needed a special case: when the
`payload:` keyword appears with `{...}` on the same line, parse the comma-separated
`field: Type` pairs inline rather than waiting for subsequent lines.

### Schedules are not brace-delimited
`schedule` blocks use indented continuation lines rather than `{ }`. The parser handles
this by reading line-by-line until it encounters an unindented top-level keyword or EOF,
rather than tracking brace depth.
