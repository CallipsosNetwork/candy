if exists("b:did_ftplugin") | finish | endif
let b:did_ftplugin = 1

setlocal commentstring=//\ %s
setlocal comments=://
setlocal expandtab
setlocal shiftwidth=2
setlocal tabstop=2
setlocal softtabstop=2
setlocal formatoptions-=t
setlocal formatoptions+=croql
