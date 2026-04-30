# tree-sitter-candy

Tree-sitter parser for `.candy` files — the candy specification language.
Produces an AST and ships highlight queries with axis-keyed captures so
themes can color each of candy's five word-axes (entity, action, time,
condition, intent) distinctly.

## What it is

candy is a specification language for stateful backends. This grammar parses
the language defined in `grammar.md` at the repo root: type and enum
declarations, actors with state and messages, multi-actor flows with
compensation, controllers exposing flows over HTTP, policies, events, and
target preference blocks.

The parser produces structural nodes for everything in the
[examples](../../examples/) and [earbnb](../../earbnb/) specs without ERROR
nodes. The expression grammar is intentionally permissive — the goal is
clean structural parsing for highlighting and navigation, not arithmetic
soundness.

## Building from source

```sh
cd extensions/tree-sitter-candy
npm install
npx tree-sitter generate
npx tree-sitter test
npx tree-sitter parse ../../examples/auth.candy
```

`tree-sitter generate` regenerates `src/parser.c` from `grammar.js`. The
generated files are committed so consumers don't need a tree-sitter CLI to
build.

## Using with Neovim

Two paths.

### Via nvim-treesitter

Point `parser_configs.candy.install_info.url` at this directory, then
`:TSInstall candy`. The sibling Neovim plugin at
`extensions/neovim/lua/candy/init.lua` does this for you:

```lua
require('candy').setup()
```

### Via plain runtimepath

Install the sibling plugin at `extensions/neovim/`. It ships a copy of
`queries/highlights.scm` under `queries/candy/highlights.scm` so the
queries are picked up by `:set rtp+=…/extensions/neovim`.

## Highlight captures

Captures are dotted-axis so themes can target a specific word-axis or fall
back to the parent.

| Capture            | Words                                                         |
|--------------------|---------------------------------------------------------------|
| `@keyword.entity`  | `actor`, `state`, `enum`, `type`, `derive`, `audit`, `self`, `id`, `flow`, `controller`, `event`, `policy`, `target`, `prose`, `external`, `invariant` |
| `@keyword.action`  | `ask`, `tell`, `emit`, `effect`, `commit`, `compensate`, `reject`, `step`, `accepts`, `subscribe`, `use`, `exports`, `uses`, `feature` |
| `@keyword.time`    | `now`, `then`, `after`, `before`, `until`, `rescue`, `at`     |
| `@keyword.condition` | `if`, `else`, `when`, `require`, `given`, `unless`, `where`, `any`, `in`, `need`, `for`, `by` |
| `@keyword.intent`  | `intent`, `examples`, `because`, `notes`, `policies`          |
| `@type.builtin`    | `int`, `string`, `opaque`, `bool`, `bytes`, `instant`, `decimal`, `unit` |
| `@type`            | PascalCase identifiers in type position                       |
| `@function.macro`  | HTTP methods (`GET`, `POST`, ...)                             |
| `@function.builtin`| `generate`, `hash`, `verify`, `sum`, `length`, `last`         |
| `@string`          | `"…"` and `"""…"""` strings                                   |
| `@number`          | integers and durations (`7d`, `500ms`, ...)                   |
| `@property`        | field names in struct fields, state fields, meta fields, struct literals |
| `@variable.parameter` | parameter names in actor / flow / accepts signatures      |

## Status

v0.1. Node names will evolve as the language stabilizes. This grammar parses
`examples/{hello,auth,todo}.candy` and `earbnb/preferences.candy` without
errors and runs the test corpus under `test/corpus/`.

The vim-regex syntax at `extensions/neovim/syntax/candy.vim` remains as a
fallback for users who don't run nvim-treesitter.
