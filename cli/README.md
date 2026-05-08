# candy CLI

The `candy` binary is the toolchain for the candy spec language. It ships as a single static Rust binary distributed via GitHub releases and as an npm wrapper (`@tensorkit/candy`).

## Subcommands

| Command | Status |
|---------|--------|
| `candy lint <file-or-dir>` | Implemented (v0.1, 10 rules) |
| `candy gen` | Planned — see issue #13 |
| `candy test` | Planned — see issue #17 |
| `candy fmt` | Planned — see issue #39 |

## Build from source

```sh
cd cli
cargo build --release
# binary at: cli/target/release/candy
```

## Run the linter

```sh
# lint a single file
candy lint examples/auth/auth.candy

# lint all files in a directory (project mode — enables cross-file checks)
candy lint examples/

# machine-readable output (NDJSON, one violation per line)
candy lint examples/ --json
```

Exit codes: `0` clean, `1` warnings only, `2` errors present.

## Lint rules (v0.1)

| Rule | Severity | Description |
|------|----------|-------------|
| `prose-required-intent` | error | Every `prose {}` block must have a non-empty `intent:` |
| `broken-cross-feature-ref` | error | `uses: feature X for Op` must resolve to an export of feature X |
| `broken-symbol-ref` | error | Type/policy references must resolve to declared blocks |
| `money-no-floats` | error | `float` is forbidden in `type` bodies — use `int` with `unit: minor` |
| `idempotency-key` | warning | Flows calling external actors should declare `key: Key` |
| `schedule-syntax-valid` | error | Every `schedule` must have `every`/`at` and a `for any` clause |
| `policy-attachment-resolves` | error | `policies: [Name]` must reference a declared `policy` block |
| `event-payload-types-resolve` | error | Event payload field types must resolve |
| `actor-state-defaults-typed` | warning | State field defaults should match the declared type |
| `underscores-in-keywords` | error | Block names and message names must not contain `_` |

Cross-file rules (`broken-cross-feature-ref`, `broken-symbol-ref`, `policy-attachment-resolves`) only run in project mode (linting a directory), not when linting a single file.

## Adding a rule

1. Create `cli/candy/src/lint/rules/<rule-name>.rs`
2. Implement `pub fn check(project: &Project) -> Vec<Violation>`
3. Add `mod <rule_name>;` and a `violations.extend(...)` call in `rules/mod.rs`
4. Add a negative fixture at `tests/fixtures/fail/<rule-name>/`
5. Add a test case to `tests/integration_test.rs`

## Project structure

```
cli/
  Cargo.toml             # workspace
  candy/
    Cargo.toml           # bin: candy
    src/
      main.rs            # clap dispatch
      lint/
        mod.rs           # lint entry point, file collection
        parser.rs        # AST parser
        output.rs        # Violation type, JSON/human printers
        rules/
          mod.rs         # rule registry
          *.rs           # one file per rule
    tests/
      integration_test.rs
      fixtures/
        fail/<rule>/     # one negative fixture per rule
```
