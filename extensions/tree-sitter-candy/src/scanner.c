#include "tree_sitter/parser.h"

// Tokens produced by this scanner. Order must match the `externals` array
// in grammar.js.
enum TokenType {
  PROSE_STRING,
};

void *tree_sitter_candy_external_scanner_create(void) { return NULL; }
void tree_sitter_candy_external_scanner_destroy(void *p) { (void)p; }
unsigned tree_sitter_candy_external_scanner_serialize(void *p, char *b) {
  (void)p; (void)b; return 0;
}
void tree_sitter_candy_external_scanner_deserialize(void *p, const char *b, unsigned n) {
  (void)p; (void)b; (void)n;
}

static inline void advance(TSLexer *lexer) {
  lexer->advance(lexer, false);
}

bool tree_sitter_candy_external_scanner_scan(void *payload, TSLexer *lexer,
                                             const bool *valid_symbols) {
  (void)payload;
  if (!valid_symbols[PROSE_STRING]) return false;

  // Skip leading whitespace (tree-sitter handles extras, but be defensive)
  while (lexer->lookahead == ' ' || lexer->lookahead == '\t') {
    lexer->advance(lexer, true);
  }

  if (lexer->lookahead != '"') return false;
  advance(lexer);
  if (lexer->lookahead != '"') return false;
  advance(lexer);
  if (lexer->lookahead != '"') return false;
  advance(lexer);

  // Now consume until we see a closing """ — handle eof gracefully.
  for (;;) {
    if (lexer->eof(lexer)) return false;

    if (lexer->lookahead == '"') {
      advance(lexer);
      if (lexer->lookahead == '"') {
        advance(lexer);
        if (lexer->lookahead == '"') {
          advance(lexer);
          lexer->result_symbol = PROSE_STRING;
          return true;
        }
      }
      // not three quotes; continue scanning (we already advanced past the
      // ones we saw; loop continues with current lookahead).
      continue;
    }

    advance(lexer);
  }
}
