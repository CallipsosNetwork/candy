# commons/

A repository of canonical candy specs — the same `Email`, `Money`,
`Hash`, `Token` definitions every project copy-pastes today, written
once.

This lives in-repo for now as a proof of concept. The intended endgame
is a separate published project that any candy project can opt into
via `candy.toml` `[deps]`. Nothing about that endgame is built yet —
this is the canonical content; the resolver, fetcher, and
versioning story all come later.

## Layout

```
commons/
  README.md                     — this file.
  types/
    <TypeName>.candy            — one spec block per file.
```

One file per type. The filename matches the spec name. Each file
contains exactly one top-level `spec` block plus its prose.

Currently shipped types:

| File              | Spec      | Underlying primitive | Pinned-by-consumer parameters |
|-------------------|-----------|-----------------------|--------------------------------|
| `types/Email.candy`    | `Email`    | `string`  | —          |
| `types/Money.candy`    | `Money`    | `int`     | `currency` |
| `types/Hash.candy`     | `Hash`     | `opaque`  | —          |
| `types/Token.candy`    | `Token`    | `opaque`  | —          |
| `types/Password.candy` | `Password` | `string`  | —          |
| `types/Phone.candy`    | `Phone`    | `string`  | —          |

## Status — provisional syntax

The `spec` block, the `use spec` line, and the `refines` extension
are not yet in `GRAMMAR.md`. This directory is the candidate content
for those constructs. A grammar ratification PR will land separately;
when it does, the linter will recognise these blocks and projects
can adopt them via `use spec Email, Hash, Token` and similar.

Until then, candy CLI (`candy lint`) silently ignores `spec` blocks
(unknown top-level keyword), so the directory is safe to ship without
breaking lint.

## Why these and not others

- `Email`, `Hash`, `Token`, `Password`, `Phone` — every shipped
  example declares them with byte-identical bodies. Unanimous
  candidates for shared canonical specs.
- `Money` — every example declares it; `currency` is the only
  parameter consumers vary, so it ships as a `parameter:` slot.
- `Id`, `Timestamp` — already built into the language
  (`GRAMMAR.md` §type "Built-in named types"). They never needed a
  spec; their canonical shape is the language's.
- `Key` — deliberately omitted. The idempotency-key parameter is
  conventionally typed `Key opaque { max: 128 }` (per
  `GRAMMAR.md` §Hard rules), but the type identifier is
  project-defined. Projects that want a different name (e.g.
  `IdempotencyKey`) shouldn't have to fight a commons spec.
- `Role` — every project's role enum is different (admin/manager/user
  vs guest/host/admin vs customer/admin). Project-declared, no
  commons candidate.

## How a project consumes commons (planned)

```candy
// project's types.candy

use spec Email, Hash, Token, Password, Phone   // unparameterized
use spec Money(currency: USD)                  // pin the parameter

// shadow when project semantics differ:
spec Email string refines {
  intent: """Internal-only — org domain required."""
  format: company-domain
}
```

Resolution rules (proposed):

1. A `spec X` declared in the project wins.
2. Otherwise, a `use spec X` line resolves against the candy
   projects listed in `candy.toml` `[deps]`. v0.1 will be local-only;
   v0.2 ships the fetcher.
3. Two deps that both export `X` resolve in `[deps]` declaration
   order.

## Future structure

Follow-ups when the grammar ratification lands:

- `commons/policies/` — shared rule clusters (e.g. `PasswordStrength`
  is identical across every example with auth).
- `commons/events/` — canonical events (e.g. `UserSignedUp`,
  `SessionRevoked`).
- `commons/externals/` — canonical external actors (`Postmark` for
  Email, `Polar` for Payments, ...).

Out of scope for this PR. File issues per category.

## Refs

- Discussion in PR #42 / session 2026-05-07.
- The shape of the `spec` block follows `policy` (`intent:` +
  `examples:`). See `GRAMMAR.md` §policy.
