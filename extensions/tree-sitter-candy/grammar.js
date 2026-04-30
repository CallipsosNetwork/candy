/**
 * tree-sitter grammar for candy — a specification language for stateful backends.
 * v0.1: parses examples/{hello,auth,todo}.candy and earbnb/preferences.candy
 * without ERROR nodes. Expression grammar is intentionally permissive — the goal
 * is structural parsing for highlighting, not a perfect arithmetic parser.
 */

module.exports = grammar({
  name: 'candy',

  word: $ => $.identifier,

  extras: $ => [
    /\s/,
    $.comment,
  ],

  externals: $ => [
    $.prose_string,
  ],

  conflicts: $ => [
    [$.identifier_type, $._expression],
    [$.invariant_decl, $._expression],
    [$.example_line, $.binary_expression],
  ],

  precedences: $ => [
    [
      'unary',
      'mul',
      'add',
      'compare',
      'contains',
      'and',
      'or',
      'time',
      'assign',
      'if',
    ],
  ],

  rules: {
    source_file: $ => repeat($._declaration),

    _declaration: $ => choice(
      $.type_decl,
      $.enum_decl,
      $.actor_decl,
      $.external_actor_decl,
      $.flow_decl,
      $.policy_decl,
      $.event_decl,
      $.controller_decl,
      $.target_decl,
      $.invariant_decl,
      $.prose_block,
    ),

    // ---------- comments / strings / numbers ----------

    comment: _ => token(seq('//', /[^\n]*/)),

    string: $ => seq(
      '"',
      repeat(choice(
        $.string_interpolation,
        $._string_escape,
        /[^"\\$\n]+/,
        '$',
      )),
      '"',
    ),

    _string_escape: _ => token.immediate(seq('\\', /[^\n]/)),

    string_interpolation: $ => seq(
      '${',
      $._expression,
      '}',
    ),

    boolean: _ => choice('true', 'false'),

    // duration must outrank integer so `7d` is one token, not int+id
    duration: _ => token(prec(2, /\d+(ms|s|m|h|d)/)),
    integer: _ => token(prec(1, /\d+/)),

    // ---------- identifiers ----------

    identifier: _ => /[a-z_][a-zA-Z0-9_]*/,
    type_identifier: _ => /[A-Z][a-zA-Z0-9]*/,
    library_name: _ => /[a-z][a-zA-Z0-9_-]*/,

    // ---------- type declarations ----------

    type_decl: $ => seq(
      'type',
      field('name', $.type_identifier),
      optional(field('underlying', $.primitive_type)),
      optional(field('body', $.meta_block)),
    ),

    primitive_type: _ => choice(
      'int', 'string', 'opaque', 'bool', 'bytes', 'instant', 'decimal',
    ),

    meta_block: $ => seq(
      '{',
      repeat(seq($.meta_field, optional(','))),
      '}',
    ),

    meta_field: $ => seq(
      field('key', $.identifier),
      ':',
      field('value', choice($._type, $._expression)),
    ),

    enum_decl: $ => seq(
      'enum',
      field('name', $.type_identifier),
      '{',
      sepEndBy(',', $.type_identifier),
      '}',
    ),

    // ---------- types (used in positions: field types, args, returns) ----------

    _type: $ => choice(
      $.primitive_type,
      $.unit_type,
      $.list_type,
      $.optional_type,
      $.generic_type,
      $.union_type,
      $.struct_type,
      $.identifier_type,
    ),

    unit_type: _ => 'unit',
    identifier_type: $ => $.type_identifier,

    list_type: $ => seq('[', $._type, ']'),

    optional_type: $ => prec(2, seq(
      choice($.identifier_type, $.generic_type, $.primitive_type),
      token.immediate('?'),
    )),

    generic_type: $ => prec(3, seq(
      $.type_identifier,
      '<',
      sepBy1(',', $._type),
      '>',
    )),

    union_type: $ => prec.left(1, seq(
      $._type, '|', $._type,
    )),

    struct_type: $ => seq(
      '{',
      sepEndBy(',', $.struct_field),
      '}',
    ),

    struct_field: $ => seq(
      field('name', $.identifier),
      ':',
      field('type', $._type),
    ),

    // ---------- actor ----------

    actor_decl: $ => seq(
      'actor',
      field('name', $.type_identifier),
      optional(seq('(', sepBy(',', $.parameter), ')')),
      field('body', $.actor_body),
    ),

    external_actor_decl: $ => seq(
      'external',
      'actor',
      field('name', $.type_identifier),
      optional(seq('(', sepBy(',', $.parameter), ')')),
      field('body', $.actor_body),
    ),

    parameter: $ => seq(
      field('name', $.identifier),
      ':',
      field('type', $._type),
    ),

    actor_body: $ => seq(
      '{',
      repeat($._actor_member),
      '}',
    ),

    _actor_member: $ => choice(
      $.state_block,
      $.derive_decl,
      $.invariant_decl,
      $.audit_block,
      $.accepts_decl,
      $.subscribe_decl,
      $.intent_field,
      $.examples_field,
      $.policies_field,
      $.field_decl,
    ),

    state_block: $ => seq(
      'state',
      '{',
      repeat($.state_field),
      '}',
    ),

    state_field: $ => seq(
      field('name', $.identifier),
      ':',
      field('type', $._type),
      optional(seq('=', field('default', $._expression))),
      optional(','),
    ),

    derive_decl: $ => seq(
      'derive',
      field('name', $.identifier),
      '=',
      field('value', $._expression),
    ),

    audit_block: $ => seq(
      'audit',
      field('name', $.identifier),
      $.block_body,
    ),

    accepts_decl: $ => seq(
      'accepts',
      field('name', $.type_identifier),
      '(', sepBy(',', $.parameter), ')',
      optional(seq('->', field('return_type', $._type))),
      field('body', $.message_body),
    ),

    message_body: $ => seq(
      '{',
      repeat($._message_member),
      '}',
    ),

    _message_member: $ => choice(
      $.intent_field,
      $.require_stmt,
      $.step_stmt,
      $.effect_stmt,
      $.emit_stmt,
      $.commit_stmt,
      $.tell_stmt,
      $.if_stmt,
    ),

    require_stmt: $ => seq(
      'require',
      optional(':'),
      field('predicate', $._expression),
      optional($.rescue_clause),
    ),

    rescue_clause: $ => seq(
      'rescue',
      sepBy1(';', $._rescue_action),
    ),

    _rescue_action: $ => choice(
      $.compensate_action,
      $.reject_action,
    ),

    compensate_action: $ => seq(
      'compensate',
      field('name', $.identifier),
    ),

    reject_action: $ => seq(
      'reject',
      field('variant', $.type_identifier),
      optional(field('payload', $._reject_payload)),
    ),

    _reject_payload: $ => choice(
      seq('(', sepBy(',', $._expression), ')'),
      $.identifier,
    ),

    step_stmt: $ => seq(
      'step',
      field('name', choice($.identifier, '_')),
      '=',
      field('value', $._expression),
      optional($.rescue_clause),
    ),

    effect_stmt: $ => seq(
      'effect',
      optional(':'),
      field('value', $._expression),
    ),

    emit_stmt: $ => seq(
      'emit',
      field('event', $.type_identifier),
      optional(field('payload', $.struct_literal_expr)),
    ),

    tell_stmt: $ => seq(
      'tell',
      field('target', $._expression),
    ),

    commit_stmt: $ => prec.right(seq(
      'commit',
      optional(field('value', $._expression)),
    )),

    if_stmt: $ => seq(
      'if',
      field('condition', $._expression),
      'then',
      field('consequence', $._expression),
      optional(seq('else', field('alternative', $._expression))),
    ),

    subscribe_decl: $ => seq(
      'subscribe',
      field('event', $.type_identifier),
      '->',
      field('action', $._expression),
    ),

    field_decl: $ => seq(
      field('name', $.identifier),
      ':',
      field('value', $._expression),
    ),

    // ---------- flow ----------

    flow_decl: $ => seq(
      'flow',
      field('name', $.type_identifier),
      '(', sepBy(',', $.parameter), ')',
      optional(seq('->', field('return_type', $._type))),
      field('body', $.flow_body),
    ),

    flow_body: $ => seq(
      '{',
      repeat($._flow_member),
      '}',
    ),

    _flow_member: $ => choice(
      $.intent_field,
      $.step_stmt,
      $.require_stmt,
      $.effect_stmt,
      $.emit_stmt,
      $.commit_stmt,
      $.tell_stmt,
      $.if_stmt,
    ),

    // ---------- controller ----------

    controller_decl: $ => seq(
      'controller',
      field('name', $.type_identifier),
      '{',
      repeat(choice($.route_decl, $.policies_field, $.intent_field)),
      '}',
    ),

    route_decl: $ => seq(
      field('method', $.http_method),
      field('path', $.route_path),
      '->',
      field('target', $._expression),
      field('body', $.route_body),
    ),

    http_method: _ => choice('GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'),

    route_path: $ => token(seq(
      '/',
      optional(seq(
        /[a-zA-Z0-9_:-]+/,
        repeat(seq('/', /[a-zA-Z0-9_:-]+/)),
      )),
    )),

    route_body: $ => seq(
      '{',
      repeat($._route_member),
      '}',
    ),

    _route_member: $ => choice(
      $.auth_field,
      $.body_field,
      $.map_field,
      $.intent_field,
      $.policies_field,
      $.field_decl,
    ),

    auth_field: $ => seq(
      'auth',
      ':',
      field('value', choice('none', 'bearer', 'basic', $.identifier)),
    ),

    body_field: $ => seq(
      'body',
      ':',
      field('value', $.struct_type),
    ),

    map_field: $ => seq(
      'map',
      ':',
      repeat1($.map_arm),
    ),

    map_arm: $ => prec.right(seq(
      field('pattern', $._map_pattern),
      '->',
      field('status', $.integer),
      optional(field('shape', $._expression)),
    )),

    _map_pattern: $ => choice(
      $.ok_pattern,
      $.err_pattern,
    ),

    ok_pattern: $ => seq(
      'ok',
      optional(seq('(', optional(choice($.identifier, '_')), ')')),
    ),

    err_pattern: $ => seq(
      'err',
      '(',
      field('variant', $.type_identifier),
      optional(field('binding', $.identifier)),
      ')',
    ),

    // ---------- policy ----------

    policy_decl: $ => seq(
      'policy',
      field('name', $.type_identifier),
      '{',
      repeat(choice(
        $.intent_field,
        $.examples_field,
        $.policies_field,
        $.field_decl,
      )),
      '}',
    ),

    // ---------- event ----------

    event_decl: $ => seq(
      'event',
      field('name', $.type_identifier),
      '{',
      sepEndBy(',', $.event_field),
      '}',
    ),

    event_field: $ => seq(
      field('name', $.identifier),
      ':',
      field('value', $._event_value),
    ),

    _event_value: $ => choice(
      $.struct_type,
      $.order_value,
      $._expression,
    ),

    order_value: $ => seq(
      'by',
      field('field', $.identifier),
    ),

    // ---------- target (preferences.candy) ----------

    target_decl: $ => seq(
      'target',
      field('name', $.identifier),
      '{',
      repeat(choice($.target_notes, $.target_when)),
      '}',
    ),

    target_notes: $ => seq(
      'notes',
      ':',
      field('value', $.string),
    ),

    target_when: $ => seq(
      'when',
      'need',
      field('concept', $.library_name),
      'use',
      field('library', $.library_name),
    ),

    // ---------- invariant (top-level form) ----------

    invariant_decl: $ => prec.right(choice(
      // top-level: `invariant Name: "prose"`
      seq(
        'invariant',
        field('name', $.type_identifier),
        ':',
        field('predicate', choice($.string, $.prose_string, $._expression)),
      ),
      // actor-local or system-wide bare prose/predicate
      seq(
        'invariant',
        field('predicate', choice($.string, $.prose_string, $._expression)),
      ),
    )),

    // ---------- prose block (feature scope) ----------

    prose_block: $ => seq(
      'prose',
      '{',
      repeat($._prose_member),
      '}',
    ),

    _prose_member: $ => choice(
      $.intent_field,
      $.exports_field,
      $.uses_field,
      $.policies_field,
      $.field_decl,
    ),

    exports_field: $ => seq(
      'exports',
      ':',
      repeat1($.exports_line),
    ),

    exports_line: $ => seq(
      field('kind', choice('actor', 'flow', 'type', 'event')),
      sepBy1(',', $.type_identifier),
    ),

    uses_field: $ => seq(
      'uses',
      ':',
      repeat1($.uses_line),
    ),

    uses_line: $ => seq(
      field('kind', choice('feature', 'external')),
      field('name', $.type_identifier),
      'for',
      sepBy1(',', $.type_identifier),
    ),

    policies_field: $ => seq(
      'policies',
      ':',
      choice(
        seq('[', sepBy(',', $.type_identifier), ']'),
        sepBy1(',', $.type_identifier),
      ),
    ),

    intent_field: $ => seq(
      'intent',
      ':',
      field('value', choice($.string, $.prose_string)),
    ),

    examples_field: $ => seq(
      'examples',
      ':',
      repeat1($.example_item),
    ),

    example_item: $ => prec.right(seq(
      '-',
      $.example_line,
      repeat($.example_line),
    )),

    example_line: $ => seq(
      field('key', $.identifier),
      ':',
      field('value', $._expression),
    ),

    // ---------- expressions ----------

    _expression: $ => choice(
      $.integer,
      $.duration,
      $.boolean,
      $.string,
      $.prose_string,
      $.identifier,
      $.type_identifier,
      $.field_access,
      $.index_expression,
      $.call_expression,
      $.struct_literal_expr,
      $.list_literal,
      $.binary_expression,
      $.unary_expression,
      $.if_expr,
      $.ask_expression,
      $.reject_expression,
      $.compensate_expression,
      $.where_expression,
      $.quantifier_expression,
      $.parenthesized,
    ),

    quantifier_expression: $ => prec.right(seq(
      'any',
      field('binding', $.identifier),
      'in',
      field('source', $._expression),
    )),

    parenthesized: $ => seq('(', $._expression, ')'),

    field_access: $ => prec(10, seq(
      $._expression,
      '.',
      field('field', choice($.identifier, $.type_identifier)),
    )),

    index_expression: $ => prec(11, seq(
      $._expression,
      '[',
      $._expression,
      ']',
    )),

    call_expression: $ => prec(12, seq(
      field('name', choice($.identifier, $.type_identifier, $.field_access)),
      '(',
      sepBy(',', $._expression),
      ')',
    )),

    struct_literal_expr: $ => prec(2, seq(
      optional(field('type', $.type_identifier)),
      '{',
      sepEndBy(',', $.struct_literal_entry),
      '}',
    )),

    struct_literal_entry: $ => choice(
      seq(field('key', $.identifier), ':', field('value', $._expression)),
      // shorthand: `email,` is `email: email`
      $.identifier,
    ),

    list_literal: $ => seq(
      '[',
      sepEndBy(',', $._expression),
      ']',
    ),

    binary_expression: $ => choice(
      prec.right('assign',  seq($._expression, '=', $._expression)),
      prec.left('or',       seq($._expression, '||', $._expression)),
      prec.left('and',      seq($._expression, '&&', $._expression)),
      prec.left('compare',  seq($._expression, choice('==', '!=', '<', '<=', '>', '>='), $._expression)),
      prec.left('add',      seq($._expression, choice('+', '-'), $._expression)),
      prec.left('mul',      seq($._expression, choice('*', '/'), $._expression)),
      prec.left('time',     seq($._expression, choice('after', 'before', 'until'), $._expression)),
      prec.left('contains', seq($._expression, 'in', $._expression)),
    ),

    unary_expression: $ => prec.right('unary', seq(
      choice('!', '-', 'not'),
      $._expression,
    )),

    if_expr: $ => prec.right('if', seq(
      'if',
      field('condition', $._expression),
      'then',
      field('consequence', $._expression),
      optional(seq('else', field('alternative', $._expression))),
    )),

    ask_expression: $ => prec.right(seq(
      'ask',
      $._expression,
    )),

    reject_expression: $ => prec.right(seq(
      'reject',
      field('variant', $.type_identifier),
      optional(field('payload', $._reject_payload)),
    )),

    compensate_expression: $ => prec.right(seq(
      'compensate',
      field('name', $.identifier),
    )),

    where_expression: $ => prec.left(seq(
      $._expression,
      'where',
      $._expression,
    )),

    // generic block body fallback (used by audit, etc.)
    block_body: $ => seq(
      '{',
      repeat($.field_decl),
      '}',
    ),
  },
});

function sepBy(sep, rule) {
  return optional(sepBy1(sep, rule));
}

function sepBy1(sep, rule) {
  return seq(rule, repeat(seq(sep, rule)));
}

function sepEndBy(sep, rule) {
  return optional(seq(rule, repeat(seq(sep, rule)), optional(sep)));
}
