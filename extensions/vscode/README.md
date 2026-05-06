# candy — VS Code syntax highlighter

TextMate grammar for `*.candy` files. The five word-axes from
[GRAMMAR.md](../../GRAMMAR.md) — ENTITY, ACTION, TIME, CONDITION, INTENT —
each get a distinct scope so colorschemes can color by axis.
Triple-quoted prose blocks render as strings, durations and HTTP methods get
dedicated scopes, and PascalCase identifiers fall through to `entity.name.type`.

## Install

### Local development

```sh
cd extensions/vscode
code --install-extension .
```

Or package first with `vsce` then install the `.vsix`:

```sh
npm install -g @vscode/vsce
vsce package
code --install-extension candy-language-0.1.0.vsix
```

Open a `.candy` file to verify — keywords, prose blocks, and durations should
all pick up distinct colors from your active theme.

### From marketplace

TODO once published.

## Highlight scopes

| Scope                            | Covers                                                     |
|----------------------------------|------------------------------------------------------------|
| `comment.line.double-slash.candy`| `//` line comments                                         |
| `string.quoted.triple.candy`     | `"""..."""` prose blocks                                   |
| `string.quoted.double.candy`     | `"..."` strings                                            |
| `meta.embedded.expression.candy` | `${...}` string interpolation                              |
| `constant.numeric.duration.candy`| Duration literals: `7d`, `30s`, `500ms`                   |
| `constant.numeric.candy`         | Plain integer literals                                     |
| `constant.language.boolean.candy`| `true`, `false`                                            |
| `keyword.control.entity.candy`   | ENTITY axis — `actor`, `flow`, `state`, `type`, `controller`, … |
| `keyword.control.action.candy`   | ACTION axis — `ask`, `tell`, `emit`, `accepts`, `step`, … |
| `keyword.control.time.candy`     | TIME axis — `now`, `after`, `until`, `expire`, `schedule`, … |
| `keyword.control.condition.candy`| CONDITION axis — `if`, `when`, `require`, `unless`, `need`, … |
| `keyword.control.intent.candy`   | INTENT axis — `intent`, `examples`, `because`             |
| `support.type.primitive.candy`   | Built-in primitives: `int`, `string`, `opaque`, `bool`, … |
| `support.function.builtin.candy` | Built-in functions: `generate`, `hash`, `verify`, `sum`, … |
| `keyword.other.http.candy`       | HTTP verbs: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, …    |
| `support.constant.result.candy`  | Result/Option variants: `ok`, `err`, `Result`, `Some`, …  |
| `entity.name.type.candy`         | PascalCase identifiers falling through to type             |

### Override colors

Add `editor.tokenColorCustomizations` to your VS Code `settings.json`:

```json
"editor.tokenColorCustomizations": {
  "textMateRules": [
    {
      "scope": "keyword.control.entity.candy",
      "settings": { "foreground": "#c678dd", "fontStyle": "bold" }
    },
    {
      "scope": "keyword.control.action.candy",
      "settings": { "foreground": "#61afef" }
    },
    {
      "scope": "keyword.control.time.candy",
      "settings": { "foreground": "#e5c07b" }
    },
    {
      "scope": "keyword.control.condition.candy",
      "settings": { "foreground": "#56b6c2" }
    },
    {
      "scope": "keyword.control.intent.candy",
      "settings": { "foreground": "#98c379", "fontStyle": "italic" }
    }
  ]
}
```

## Roadmap

A tree-sitter grammar is on the roadmap alongside the Neovim tree-sitter work
(tracked in issue #21). Scope names are kept consistent with the Neovim capture
names so themes written for one highlighter apply cleanly to the other.
