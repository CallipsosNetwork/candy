# candy.toml — v0.1 schema

`candy.toml` is the project manifest. Every candy project has exactly one,
at the project root. It names the project, pins the candy runtime version,
declares which target languages the project generates, and lists codegen
dependencies. The toolchain reads it first; humans read it to orient
themselves before opening any `.candy` file.

---

## Layout at a glance

```toml
[project]
name    = "myproject"
version = "0.1.0"

[candy]
runtime = "0.1"

[targets.go]
language  = "go"
framework = "chi"

[targets.typescript]
language  = "typescript"
framework = "hono"

[deps]
"candy/grammar"    = "0.1"
"candy/codegen-go" = "0.1"
"candy/codegen-ts" = "0.1"
```

---

## Sections

### `[project]` — required

| Field     | Type   | Required | Description                                  |
|-----------|--------|----------|----------------------------------------------|
| `name`    | string | yes      | Project identifier. Lowercase, no spaces.    |
| `version` | string | yes      | Project version. Semver (`MAJOR.MINOR.PATCH`).|

**`name`** — a short, lowercase identifier for the project. By convention
it matches the project directory name, but the toolchain does not enforce
this in v0.1. Examples from the repo: `"airbnb"`, `"auth"`, `"billing"`.

**`version`** — the project's own version in semver form. All examples in
this repo carry `"0.1.0"`. This field is informational in v0.1; no tooling
currently reads it to enforce compatibility with generated artifacts.

---

### `[candy]` — required

| Field     | Type   | Required | Description                                        |
|-----------|--------|----------|----------------------------------------------------|
| `runtime` | string | yes      | The candy runtime version this manifest targets.   |

**`runtime`** — a semver-like version string declaring which version of
the candy runtime (grammar, codegen prompts, and toolchain) this manifest
is compatible with. v0.1 manifests set this to `"0.1"`. When the runtime
version advances, this field is the migration signal: tooling compares
`runtime` against its own version before proceeding.

---

### `[targets.<name>]` — at least one required

One `[targets.<name>]` section per generated backend. The section key
`<name>` identifies the target and must match the name used in the
corresponding `target` block in `preferences.candy` (see GRAMMAR.md §
"target"). In v0.1, valid names are `go`, `rust`, `typescript`, and
`python`.

Generated code lands in `targets/<name>/` at the project root. Each
section tells the codegen which language to emit and which HTTP framework
to use.

| Field       | Type   | Required | Description                                       |
|-------------|--------|----------|---------------------------------------------------|
| `language`  | string | yes      | Target language. One of: `go`, `rust`,            |
|             |        |          | `typescript`, `python`.                           |
| `framework` | string | yes      | HTTP framework. Examples: `chi`, `axum`, `hono`,  |
|             |        |          | `fastapi`.                                        |

**`language`** — the language codegen should emit. The toolchain uses this
to select the appropriate codegen prompt and apply the idioms in
`preferences.candy` for this target.

**`framework`** — the HTTP framework the generated code should use. This
is the primary hint codegen uses when materializing `controller` blocks.
The value is a free-form string; the generator interprets it as a
preference, not a hard constraint.

Framework choices across the current examples: all seven projects use
`chi` for Go, `axum` for Rust, `hono` for TypeScript, and `fastapi` for
Python. These are not the only valid values — they are the defaults
established by the repo examples.

Declaring multiple targets is common; you may declare as few as one. The
examples/airbnb project is the canonical multi-target case, declaring all
four. Codegen runs once per target; the spec does not change.

---

### `[deps]` — required

A map of package name → version string. Each entry declares a codegen
dependency: a named skill, prompt bundle, or script the project relies on
during generation.

```toml
[deps]
"candy/grammar"      = "0.1"
"candy/codegen-go"   = "0.1"
"candy/codegen-rust" = "0.1"
"candy/codegen-ts"   = "0.1"
"candy/codegen-py"   = "0.1"
```

**In v0.1, the `[deps]` table is informational.** It declares intent and
communicates which codegen skills the project consumes, but no tooling
currently reads it to fetch or resolve packages. The `candy install`
command that would consume this table is a planned future feature (see
"What's planned for later versions").

All seven examples in this repo carry the same five entries at version
`"0.1"`. A project that only generates one or two targets would drop the
unused `codegen-*` entries; the comment in `airbnb/candy.toml` confirms
these are runtime deps ("prompts, skills, scripts").

---

## Conventions

- Project names are lowercase. Match the directory name where practical.
- `version` in `[project]` is semver. `runtime` in `[candy]` and dep
  versions in `[deps]` use shorter `MAJOR.MINOR` strings (e.g., `"0.1"`).
- Target section keys (`[targets.go]`, `[targets.typescript]`, …) are
  lowercase and match the identifiers used in `preferences.candy` target
  blocks. Mismatch silently breaks the preference lookup.
- Include only the `[deps]` entries for the codegen skills your declared
  targets actually need. An all-four-target project carries all five
  entries; a Go-only project needs only `candy/grammar` and
  `candy/codegen-go`.
- No custom or project-specific tables in v0.1. Only the four sections
  listed above are recognized.

---

## Relationship to other files

- **`preferences.candy`** — per-target library and idiom preferences.
  Where `candy.toml` says "generate TypeScript with Hono", `preferences.candy`
  says "for TypeScript, use drizzle for the database and argon2 for
  hashing". See GRAMMAR.md § "target" and architecture.md §
  "Substrate vs. spec".
- **`spec/*.candy`** — the actual system specification. Actors, flows,
  controllers, policies, types, events. `candy.toml` does not reference
  spec files; the toolchain discovers them by directory convention (see
  GRAMMAR.md § "Project layout").
- **`targets/<lang>/`** — generated output. Written by the codegen; not
  hand-edited. Each subdirectory matches a `[targets.<name>]` entry.

---

## What's frozen in v0.1

The following are guaranteed stable. A change to any of these is a
breaking change requiring a migration path:

- The four top-level section names: `[project]`, `[candy]`, `[targets.*]`,
  `[deps]`.
- The required fields in each section: `project.name`, `project.version`,
  `candy.runtime`, `targets.<name>.language`, `targets.<name>.framework`.
- The four valid `language` values: `"go"`, `"rust"`, `"typescript"`,
  `"python"`.
- The `runtime = "0.1"` version string as the current valid value.

---

## What's planned for later versions

- **`candy install`** — a dependency resolver that reads `[deps]` and
  fetches the declared codegen skills. In v0.1 the table is written but
  not consumed.
- **`[scripts]`** — project-level commands (generate, lint, conformance)
  runnable via a `candy run <script>` interface.
- **`[workspace]`** — multi-project support: a root manifest that
  references member projects for cross-project dependency resolution and
  shared generation passes.
- **Additional `language` values** — the four current targets are not
  the permanent ceiling; new language identifiers will be added with
  corresponding codegen skills.
- **Validation of `name`** — a defined naming regex (likely
  `[a-z][a-z0-9-]*`) enforced by the toolchain. Currently the field is
  free-form.

---

## Examples in this repo

- `examples/airbnb/candy.toml` — the canonical full-form manifest.
  Declares all four targets and all five deps. Use this as the reference
  shape.
- `examples/todo/candy.toml` — structurally identical to airbnb but
  without inline comments. Useful as a clean template baseline.
- `examples/auth/candy.toml`, `examples/wallet/candy.toml`,
  `examples/billing/candy.toml`, `examples/notifications/candy.toml`,
  `examples/rbac/candy.toml` — all carry the same shape. The seven
  examples currently share identical field values (same targets, same
  deps, same versions); only `project.name` differs.
