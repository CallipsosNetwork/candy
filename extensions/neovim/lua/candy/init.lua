-- Optional helper: registers tree-sitter-candy with nvim-treesitter and
-- sets ft=candy on *.candy files.
--
-- Usage:
--   require('candy').setup()
--
-- After setup, run :TSInstall candy to compile the parser.
local M = {}

function M.setup(opts)
  opts = opts or {}

  local ok, parsers = pcall(require, 'nvim-treesitter.parsers')
  if not ok then
    return
  end

  local default_url = vim.fn.fnamemodify(
    debug.getinfo(1).source:sub(2),
    ':p:h:h:h:h'
  ) .. '/tree-sitter-candy'

  parsers.get_parser_configs().candy = {
    install_info = {
      url = opts.url or default_url,
      files = { 'src/parser.c', 'src/scanner.c' },
      branch = opts.branch or 'main',
      generate_requires_npm = false,
      requires_generate_from_grammar = false,
    },
    filetype = 'candy',
  }

  vim.filetype.add({
    extension = { candy = 'candy' },
  })
end

return M
