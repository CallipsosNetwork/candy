# candy.nvim — Neovim syntax highlighter for candy

Vim-regex syntax for `*.candy` files. The five word-axes from
[`grammar.md`](../../grammar.md) — ENTITY, ACTION, TIME, CONDITION, INTENT —
each get a distinct highlight group so colorschemes can color by axis.
Triple-quoted prose blocks render as strings, durations and HTTP methods get
dedicated groups, and PascalCase identifiers fall through to `Type`.

## Install

### lazy.nvim

Local checkout:

```lua
{ dir = "/path/to/candy/extensions/neovim", ft = "candy" }
```

From a remote repo (replace `yourorg/candy` with the real slug):

```lua
{
  "yourorg/candy",
  ft = "candy",
  config = function()
    vim.opt.runtimepath:append(vim.fn.stdpath("data") .. "/lazy/candy/extensions/neovim")
  end,
}
```

### packer.nvim

```lua
use { 'yourorg/candy', rtp = 'extensions/neovim', ft = 'candy' }
```

### vim-plug

```vim
Plug 'yourorg/candy', { 'rtp': 'extensions/neovim', 'for': 'candy' }
```

### Plain runtimepath (hacking locally)

Copy or symlink the contents of `extensions/neovim/` into `~/.config/nvim/`:

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
