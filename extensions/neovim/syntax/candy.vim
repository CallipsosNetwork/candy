if exists("b:current_syntax") | finish | endif

" Comments — match before strings so // inside no string takes effect
syn match candyComment "//.*$" contains=@Spell
hi def link candyComment Comment

" Triple-quoted prose blocks (must come before single-quoted strings)
syn region candyProse start=+"""+ end=+"""+ contains=candyInterp
hi def link candyProse String

" Single-quoted/double-quoted strings with ${...} interpolation
syn region candyString start=+"+ skip=+\\"+ end=+"+ contains=candyInterp
syn match  candyInterp "\${[^}]*}" contained
hi def link candyString String
hi def link candyInterp Special

" Numbers and durations (durations like 7d, 10m, 1h, 30s, 500ms)
syn match candyDuration "\<\d\+\(ms\|s\|m\|h\|d\)\>"
syn match candyNumber   "\<\d\+\>"
hi def link candyDuration Number
hi def link candyNumber   Number

" Booleans
syn keyword candyBoolean true false
hi def link candyBoolean Boolean

" Word-axis: ENTITY (things that exist)
syn keyword candyEntity actor state config providers enum type derive journal audit self id
syn keyword candyEntity flow controller event policy target prose external invariant
hi def link candyEntity Structure

" Word-axis: ACTION (things that happen)
syn keyword candyAction ask tell emit emits effect commit compensate reject
syn keyword candyAction step accepts subscribe use exports uses
hi def link candyAction Statement

" Word-axis: TIME (when, in what order, for how long)
syn keyword candyTime now then after before until expire schedule at rescue
hi def link candyTime Special

" Word-axis: CONDITION (under what circumstances)
syn keyword candyCondition if else when require given unless where any in need
hi def link candyCondition Conditional

" Word-axis: INTENT (why this exists, what good looks like)
syn keyword candyIntent intent examples because
hi def link candyIntent PreProc

" Built-in primitive types
syn keyword candyPrimitive int string opaque bool bytes instant decimal unit
hi def link candyPrimitive Type

" Built-in functions
syn keyword candyBuiltin generate hash verify sum length last secret
hi def link candyBuiltin Function

" HTTP methods used in controller blocks
syn keyword candyHttpMethod GET POST PUT PATCH DELETE HEAD OPTIONS
hi def link candyHttpMethod Statement

" Result variants and Option
syn keyword candyResult ok err Ok Err Result Option Some None
hi def link candyResult Constant

" PascalCase identifiers as types (matched last so keywords win)
syn match candyType "\<[A-Z][A-Za-z0-9]*\>"
hi def link candyType Type

let b:current_syntax = "candy"
