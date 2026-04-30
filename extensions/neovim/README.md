# candy.nvim — Neovim syntax highlighter for candy

Vim-regex syntax for `*.candy` files. The five word-axes from
[`grammar.md`](../../grammar.md) — ENTITY, ACTION, TIME, CONDITION, INTENT —
each get a distinct highlight group so colorschemes can color by axis.
Triple-quoted prose blocks render as strings, durations and HTTP methods get
dedicated groups, and PascalCase identifiers fall through to `Type`.

## Install

### lazy.nvim / LazyVim (recommended)

1. Clone the candy repo somewhere stable, e.g. `~/code/candy`:

   ```sh
   git clone https://github.com/CallipsosNetwork/candy.git ~/code/candy
   ```

2. Create `~/.config/nvim/lua/plugins/candy.lua` with:

   ```lua
   return {
     dir = vim.fn.expand("~/code/candy/extensions/neovim"),
     name = "candy.nvim",
     ft = "candy",
   }
   ```

3. Restart Neovim. lazy.nvim picks the spec up automatically; the plugin loads when you first open a `.candy` buffer.

4. Verify with `nvim examples/hello.candy` (from the candy repo). Inside Neovim:

   ```
   :echo &filetype          " → candy
   :echo b:current_syntax   " → candy
   :highlight candyEntity   " → links to Structure
   ```

   If any of those are empty, run `:Lazy reload candy.nvim` and reopen the file.

> **Note for LazyVim users:** lazy.nvim resets `runtimepath`, so dropping the
> plugin into `~/.config/nvim/pack/*/start/` does **not** work — it gets
> stripped. Use the spec above.

### Other plugin managers

**packer.nvim** (after publishing the repo):

```lua
use { 'CallipsosNetwork/candy', rtp = 'extensions/neovim', ft = 'candy' }
```

**vim-plug**:

```vim
Plug 'CallipsosNetwork/candy', { 'rtp': 'extensions/neovim', 'for': 'candy' }
```

**Plain Neovim, no plugin manager**:

```sh
ln -s "$PWD/extensions/neovim/ftdetect/candy.vim" ~/.config/nvim/ftdetect/candy.vim
ln -s "$PWD/extensions/neovim/ftplugin/candy.vim" ~/.config/nvim/ftplugin/candy.vim
ln -s "$PWD/extensions/neovim/syntax/candy.vim"   ~/.config/nvim/syntax/candy.vim
```

## Highlight groups

| Group              | Axis / role                                                |
|--------------------|------------------------------------------------------------|
| `candyEntity`      | ENTITY — `actor`, `flow`, `state`, `type`, `controller`, … |
| `candyAction`      | ACTION — `ask`, `tell`, `emit`, `accepts`, `step`, …       |
| `candyTime`        | TIME — `now`, `after`, `until`, `expire`, `schedule`, …    |
| `candyCondition`   | CONDITION — `if`, `when`, `require`, `unless`, `need`, …   |
| `candyIntent`      | INTENT — `intent`, `examples`, `because`                   |
| `candyDuration`    | Duration literals like `7d`, `30s`, `500ms`                |
| `candyHttpMethod`  | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, …                 |

Override in your colorscheme:

```lua
vim.api.nvim_set_hl(0, 'candyEntity',    { fg = '#c678dd', bold = true })
vim.api.nvim_set_hl(0, 'candyAction',    { fg = '#61afef' })
vim.api.nvim_set_hl(0, 'candyTime',      { fg = '#e5c07b' })
vim.api.nvim_set_hl(0, 'candyCondition', { fg = '#56b6c2' })
vim.api.nvim_set_hl(0, 'candyIntent',    { fg = '#98c379', italic = true })
```

## Roadmap

A tree-sitter grammar is on the roadmap; tracked alongside issue #21.
