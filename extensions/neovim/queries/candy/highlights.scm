; tree-sitter-candy — highlights.
;
; Captures are axis-keyed (@keyword.entity / .action / .time / .condition /
; .intent) so a theme can color per word-axis. Editors that don't recognize
; the dotted form fall back to the parent (@keyword).

;; ---------- entity axis ----------
[
  "actor"
  "state"
  "enum"
  "type"
  "derive"
  "audit"
  "flow"
  "controller"
  "event"
  "policy"
  "target"
  "prose"
  "external"
  "invariant"
] @keyword.entity

;; ---------- action axis ----------
[
  "ask"
  "tell"
  "emit"
  "effect"
  "commit"
  "compensate"
  "reject"
  "step"
  "accepts"
  "subscribe"
  "use"
  "exports"
  "uses"
  "feature"
] @keyword.action

;; ---------- time axis ----------
[
  "after"
  "before"
  "until"
  "rescue"
  "then"
] @keyword.time

;; ---------- condition axis ----------
[
  "if"
  "else"
  "when"
  "require"
  "where"
  "any"
  "in"
  "need"
  "for"
  "by"
] @keyword.condition

;; ---------- intent axis ----------
[
  "intent"
  "examples"
  "notes"
  "policies"
] @keyword.intent

;; ---------- modifiers / connector words ----------
[
  "auth"
  "body"
  "map"
  "ok"
  "err"
  "none"
  "bearer"
  "basic"
  "not"
] @keyword

;; ---------- types ----------
(primitive_type) @type.builtin
(unit_type) @type.builtin
(type_identifier) @type

;; ---------- builtins ----------
((call_expression
   name: (identifier) @function.builtin)
 (#match? @function.builtin "^(generate|hash|verify|sum|length|last)$"))

(call_expression
  name: (identifier) @function.call)

(call_expression
  name: (field_access
    field: (identifier) @function.method))

;; ---------- literals ----------
(comment) @comment
(string) @string
(prose_string) @string
(string_interpolation) @punctuation.special
(integer) @number
(duration) @number
(boolean) @boolean

;; ---------- HTTP method ----------
(http_method) @function.macro

;; ---------- identifiers in field positions ----------
(parameter name: (identifier) @variable.parameter)
(state_field name: (identifier) @property)
(struct_field name: (identifier) @property)
(meta_field key: (identifier) @property)
(struct_literal_entry key: (identifier) @property)
(event_field name: (identifier) @property)
(field_access field: (identifier) @property)

;; ---------- target preferences ----------
(target_decl name: (identifier) @namespace)
(target_when concept: (library_name) @variable.builtin)
(target_when library: (library_name) @string.special)

;; ---------- routes ----------
(route_path) @string.special.path

;; ---------- punctuation ----------
[ "{" "}" "[" "]" "(" ")" ] @punctuation.bracket
[ "," ":" ";" "->" "=" "|" "?" ] @punctuation.delimiter

;; ---------- operators ----------
[
  "=="
  "!="
  "<"
  "<="
  ">"
  ">="
  "&&"
  "||"
  "+"
  "-"
  "*"
  "/"
  "!"
] @operator
