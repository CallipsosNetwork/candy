//! Hand-written recursive-descent parser for the candy grammar.
//!
//! Produces an AST sufficient for the 10 v0.1 lint rules. Recovery: on
//! unrecognised syntax inside a block we skip to the next `}` and continue.
//! Spans record (file, line) for every node that needs lint output.
//!
//! Several AST fields are declared for future rules and are currently unused.
#![allow(
    dead_code,
    clippy::manual_strip,
    clippy::field_reassign_with_default,
    clippy::while_let_loop,
    clippy::collapsible_if
)]

use std::collections::HashMap;
use std::path::{Path, PathBuf};

use anyhow::Result;

/// A (file-path, 1-based line) location attached to AST nodes.
#[derive(Debug, Clone)]
pub struct Span {
    pub file: PathBuf,
    pub line: usize,
}

/// The project: all parsed files joined into a single declaration namespace.
#[derive(Debug, Default)]
pub struct Project {
    pub files: Vec<ParsedFile>,
}

#[derive(Debug)]
pub struct ParsedFile {
    pub path: PathBuf,
    pub blocks: Vec<Block>,
}

#[derive(Debug, Clone)]
pub struct Block {
    pub kind: BlockKind,
    pub name: String,
    pub span: Span,
    pub fields: BlockFields,
}

#[derive(Debug, Clone, PartialEq)]
pub enum BlockKind {
    Prose,
    Actor,
    ExternalActor,
    Flow,
    Controller,
    Policy,
    Event,
    Type,
    Enum,
    Invariant,
    Target,
    Schedule,
}

/// Extracted fields from a block body, used by lint rules.
#[derive(Debug, Clone, Default)]
pub struct BlockFields {
    /// `intent:` value (for prose-required-intent)
    pub intent: Option<String>,
    /// `exports:` items parsed from prose blocks
    pub exports: Vec<ExportItem>,
    /// `uses:` items parsed from prose blocks
    pub uses_decls: Vec<UsesDecl>,
    /// `policies:` list
    pub policies: Vec<PolicyRef>,
    /// `accepts` messages on actors/externals
    pub accepts: Vec<AcceptsDecl>,
    /// `providers:` list on external actors
    pub providers: Vec<String>,
    /// state fields (for actor-state-defaults-typed)
    pub state_fields: Vec<StateField>,
    /// For event blocks: payload fields
    pub payload_fields: Vec<PayloadField>,
    /// For flow/actor bodies: ask/tell calls to external actors
    pub external_calls: Vec<ExternalCall>,
    /// For schedule blocks
    pub schedule_info: Option<ScheduleInfo>,
    /// For type blocks: underlying primitive if any
    pub type_primitive: Option<String>,
    /// Whether `float` keyword appears in body (money-no-floats)
    pub has_float: bool,
    /// For flow bodies: whether `key: Key` parameter is present
    pub has_key_param: bool,
    /// Raw identifiers used in type/event/policy references (for broken-symbol-ref)
    pub type_refs: Vec<TypeRef>,
    /// Name of the block (duplicated for convenience in sub-structs)
    pub raw_params: String,
}

#[derive(Debug, Clone)]
pub struct ExportItem {
    pub kind: String, // "actor", "flow", "type", "event", "policy", "enum"
    pub name: String,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct UsesDecl {
    pub feature: String,
    pub op: String,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct PolicyRef {
    pub name: String,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct AcceptsDecl {
    pub name: String,
    pub span: Span,
    pub has_key_param: bool,
    pub external_calls: Vec<ExternalCall>,
}

#[derive(Debug, Clone)]
pub struct ExternalCall {
    pub actor: String,
    pub op: String,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct StateField {
    pub name: String,
    pub type_name: String,
    pub default_value: Option<String>,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct PayloadField {
    pub name: String,
    pub type_name: String,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct TypeRef {
    pub name: String,
    pub span: Span,
}

#[derive(Debug, Clone)]
pub struct ScheduleInfo {
    pub has_every_or_at: bool,
    pub has_for_clause: bool,
    pub duration_valid: bool,
    pub span: Span,
}

// ── Built-in primitives that are always resolved ──────────────────────────

const PRIMITIVES: &[&str] = &[
    "int", "string", "opaque", "bool", "bytes", "instant", "decimal",
    "float", // included so float refs resolve (the rule catches the declaration, not the use)
    "unit", "Id",
];

// ── Project loading ────────────────────────────────────────────────────────

impl Project {
    pub fn load(files: &[std::path::PathBuf]) -> Result<Self> {
        let mut project = Project::default();
        for path in files {
            let src = std::fs::read_to_string(path)?;
            let parsed = parse_file(path, &src);
            project.files.push(parsed);
        }
        Ok(project)
    }

    /// All declared block names by kind, across all files.
    pub fn all_blocks(&self) -> Vec<&Block> {
        self.files.iter().flat_map(|f| f.blocks.iter()).collect()
    }

    /// Look up a block by name (any kind).
    pub fn find_block(&self, name: &str) -> Option<&Block> {
        self.all_blocks().into_iter().find(|b| b.name == name)
    }

    /// Collect all declared names (actors, types, enums, events, policies, flows).
    pub fn declared_names(&self) -> HashMap<String, BlockKind> {
        let mut map = HashMap::new();
        for b in self.all_blocks() {
            map.insert(b.name.clone(), b.kind.clone());
        }
        // built-in primitives always resolve
        for p in PRIMITIVES {
            map.insert(p.to_string(), BlockKind::Type);
        }
        // built-in named types (GRAMMAR.md §type "Built-in named types"):
        // Id and Timestamp are universally built-in. Everything else
        // (Money, Email, Password, Key, Token, Role, ...) is project-declared.
        for bi in &["Id", "Timestamp"] {
            map.entry(bi.to_string()).or_insert(BlockKind::Type);
        }
        map
    }

    /// Feature export table: feature_name -> Vec<exported symbol names>
    pub fn feature_exports(&self) -> HashMap<String, Vec<String>> {
        let mut map: HashMap<String, Vec<String>> = HashMap::new();
        for file in &self.files {
            for block in &file.blocks {
                if block.kind == BlockKind::Prose {
                    let feature_name = feature_name_from_path(&file.path);
                    let exports = block
                        .fields
                        .exports
                        .iter()
                        .map(|e| e.name.clone())
                        .collect::<Vec<_>>();
                    map.entry(feature_name).or_default().extend(exports);
                }
            }
        }
        map
    }
}

fn feature_name_from_path(path: &Path) -> String {
    // For folder features: parent dir name. For single-file: file stem.
    if let Some(name) = path.file_stem() {
        if name == "prose" {
            // folder feature — use parent dir name
            if let Some(parent) = path.parent().and_then(|p| p.file_name()) {
                return parent.to_string_lossy().to_string();
            }
        }
        return name.to_string_lossy().to_string();
    }
    "unknown".to_string()
}

// ── Parser ─────────────────────────────────────────────────────────────────

struct Parser<'s> {
    path: PathBuf,
    src: &'s str,
    pos: usize,
    line: usize,
}

impl<'s> Parser<'s> {
    fn new(path: &Path, src: &'s str) -> Self {
        Parser {
            path: path.to_path_buf(),
            src,
            pos: 0,
            line: 1,
        }
    }

    fn span(&self) -> Span {
        Span {
            file: self.path.clone(),
            line: self.line,
        }
    }

    fn rest(&self) -> &'s str {
        &self.src[self.pos..]
    }

    fn peek(&self) -> Option<char> {
        self.rest().chars().next()
    }

    fn advance(&mut self) {
        if let Some(c) = self.rest().chars().next() {
            self.pos += c.len_utf8();
            if c == '\n' {
                self.line += 1;
            }
        }
    }

    fn skip_whitespace(&mut self) {
        while let Some(c) = self.peek() {
            if c.is_whitespace() {
                self.advance();
            } else if self.rest().starts_with("//") {
                // line comment
                while let Some(c) = self.peek() {
                    self.advance();
                    if c == '\n' {
                        break;
                    }
                }
            } else {
                break;
            }
        }
    }

    fn peek_word(&mut self) -> Option<&'s str> {
        self.skip_whitespace();
        let rest = self.rest();
        let end = rest
            .char_indices()
            .take_while(|(_, c)| c.is_alphanumeric() || *c == '_' || *c == '-')
            .map(|(i, c)| i + c.len_utf8())
            .last()
            .unwrap_or(0);
        if end == 0 {
            None
        } else {
            Some(&rest[..end])
        }
    }

    fn read_word(&mut self) -> Option<String> {
        self.skip_whitespace();
        let rest = self.rest();
        let end = rest
            .char_indices()
            .take_while(|(_, c)| c.is_alphanumeric() || *c == '_' || *c == '-')
            .map(|(i, c)| i + c.len_utf8())
            .last()
            .unwrap_or(0);
        if end == 0 {
            None
        } else {
            let word = rest[..end].to_string();
            for _ in 0..end {
                self.advance();
            }
            Some(word)
        }
    }

    fn expect_word(&mut self, expected: &str) -> bool {
        self.skip_whitespace();
        if self.rest().starts_with(expected) {
            let next_char = self.rest().chars().nth(expected.len());
            let boundary = next_char
                .map(|c| !c.is_alphanumeric() && c != '_' && c != '-')
                .unwrap_or(true);
            if boundary {
                for _ in expected.chars() {
                    self.advance();
                }
                return true;
            }
        }
        false
    }

    fn read_until_char(&mut self, stop: char) -> String {
        let mut result = String::new();
        while let Some(c) = self.peek() {
            if c == stop {
                break;
            }
            result.push(c);
            self.advance();
        }
        result
    }

    fn read_line(&mut self) -> String {
        let mut result = String::new();
        while let Some(c) = self.peek() {
            if c == '\n' {
                self.advance();
                break;
            }
            result.push(c);
            self.advance();
        }
        result
    }

    /// Read a triple-quoted string `"""..."""`
    fn read_triple_string(&mut self) -> String {
        // caller has consumed the first `"`; we need to check for `""`
        // Actually: at call site we check for `"""` then advance 3 chars
        let mut result = String::new();
        loop {
            if self.rest().starts_with("\"\"\"") {
                self.advance();
                self.advance();
                self.advance();
                break;
            }
            if let Some(c) = self.peek() {
                result.push(c);
                self.advance();
            } else {
                break;
            }
        }
        result.trim().to_string()
    }

    /// Read a single-quoted or double-quoted string (single line).
    fn read_quoted_string(&mut self) -> String {
        let quote = self.peek().unwrap_or('"');
        self.advance(); // consume opening quote
        let mut result = String::new();
        loop {
            match self.peek() {
                None => break,
                Some(c) if c == quote => {
                    self.advance();
                    break;
                }
                Some('\\') => {
                    self.advance();
                    if let Some(c2) = self.peek() {
                        result.push(c2);
                        self.advance();
                    }
                }
                Some(c) => {
                    result.push(c);
                    self.advance();
                }
            }
        }
        result
    }

    /// Skip forward until we consume a `{`, then track depth to find matching `}`.
    fn skip_block_body(&mut self) {
        // find opening brace
        while let Some(c) = self.peek() {
            if c == '{' {
                self.advance();
                break;
            }
            self.advance();
        }
        let mut depth = 1i32;
        while depth > 0 {
            match self.peek() {
                None => break,
                Some('{') => {
                    depth += 1;
                    self.advance();
                }
                Some('}') => {
                    depth -= 1;
                    self.advance();
                }
                Some('"') => {
                    self.read_quoted_or_triple();
                }
                _ => {
                    self.advance();
                }
            }
        }
    }

    fn read_quoted_or_triple(&mut self) -> String {
        if self.rest().starts_with("\"\"\"") {
            self.advance();
            self.advance();
            self.advance();
            self.read_triple_string()
        } else {
            self.read_quoted_string()
        }
    }

    /// Read the content of a block body `{ ... }`, tracking nesting.
    fn read_block_body(&mut self) -> (String, usize) {
        // skip to `{`
        while let Some(c) = self.peek() {
            if c == '{' {
                self.advance();
                break;
            }
            self.advance();
        }
        let start_line = self.line;
        let mut body = String::new();
        let mut depth = 1i32;
        while depth > 0 {
            match self.peek() {
                None => break,
                Some('{') => {
                    depth += 1;
                    body.push('{');
                    self.advance();
                }
                Some('}') => {
                    depth -= 1;
                    if depth > 0 {
                        body.push('}');
                    }
                    self.advance();
                }
                Some('"') => {
                    // include strings verbatim so we don't miscount braces inside strings
                    if self.rest().starts_with("\"\"\"") {
                        body.push_str("\"\"\"");
                        self.advance();
                        self.advance();
                        self.advance();
                        loop {
                            if self.rest().starts_with("\"\"\"") {
                                body.push_str("\"\"\"");
                                self.advance();
                                self.advance();
                                self.advance();
                                break;
                            }
                            if let Some(c) = self.peek() {
                                body.push(c);
                                self.advance();
                            } else {
                                break;
                            }
                        }
                    } else {
                        body.push('"');
                        self.advance();
                        loop {
                            match self.peek() {
                                None => break,
                                Some('"') => {
                                    body.push('"');
                                    self.advance();
                                    break;
                                }
                                Some('\\') => {
                                    body.push('\\');
                                    self.advance();
                                    if let Some(c) = self.peek() {
                                        body.push(c);
                                        self.advance();
                                    }
                                }
                                Some(c) => {
                                    body.push(c);
                                    self.advance();
                                }
                            }
                        }
                    }
                }
                Some(c) => {
                    body.push(c);
                    self.advance();
                }
            }
        }
        (body, start_line)
    }

    fn parse_blocks(&mut self) -> Vec<Block> {
        let mut blocks = Vec::new();
        loop {
            self.skip_whitespace();
            if self.rest().is_empty() {
                break;
            }

            // Try to parse a block keyword
            let block_span = self.span();
            let word = match self.peek_word() {
                Some(w) => w.to_string(),
                None => {
                    self.advance();
                    continue;
                }
            };

            match word.as_str() {
                "prose" => {
                    self.read_word();
                    let (body, _) = self.read_block_body();
                    let fields = parse_prose_body(&body, &self.path, block_span.line);
                    blocks.push(Block {
                        kind: BlockKind::Prose,
                        name: "prose".to_string(),
                        span: block_span,
                        fields,
                    });
                }
                "external" => {
                    self.read_word(); // "external"
                    if self.expect_word("actor") {
                        let name = self.read_word().unwrap_or_default();
                        let params = self.read_params_line();
                        let (body, _) = self.read_block_body();
                        let mut fields =
                            parse_external_actor_body(&body, &self.path, block_span.line);
                        fields.raw_params = params;
                        blocks.push(Block {
                            kind: BlockKind::ExternalActor,
                            name,
                            span: block_span,
                            fields,
                        });
                    } else {
                        self.skip_block_body();
                    }
                }
                "actor" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    let params = self.read_params_line();
                    let (body, _) = self.read_block_body();
                    let mut fields = parse_actor_body(&body, &self.path, block_span.line);
                    fields.raw_params = params;
                    blocks.push(Block {
                        kind: BlockKind::Actor,
                        name,
                        span: block_span,
                        fields,
                    });
                }
                "flow" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    let params_line = self.read_params_and_return_line();
                    let (body, _) = self.read_block_body();
                    let mut fields = parse_flow_body(&body, &self.path, block_span.line);
                    fields.has_key_param = params_line.contains("key:")
                        || params_line.contains("key :")
                        || params_line.contains(", key");
                    fields.raw_params = params_line;
                    blocks.push(Block {
                        kind: BlockKind::Flow,
                        name,
                        span: block_span,
                        fields,
                    });
                }
                "controller" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    let (body, _) = self.read_block_body();
                    let fields = parse_controller_body(&body, &self.path, block_span.line);
                    blocks.push(Block {
                        kind: BlockKind::Controller,
                        name,
                        span: block_span,
                        fields,
                    });
                }
                "policy" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    let (body, _) = self.read_block_body();
                    let fields = parse_policy_body(&body, &self.path, block_span.line);
                    blocks.push(Block {
                        kind: BlockKind::Policy,
                        name,
                        span: block_span,
                        fields,
                    });
                }
                "event" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    let (body, _) = self.read_block_body();
                    let fields = parse_event_body(&body, &self.path, block_span.line);
                    blocks.push(Block {
                        kind: BlockKind::Event,
                        name,
                        span: block_span,
                        fields,
                    });
                }
                "type" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    let rest_of_line = self.read_type_header();
                    let fields =
                        parse_type_header_and_body(&rest_of_line, &self.path, block_span.line);
                    blocks.push(Block {
                        kind: BlockKind::Type,
                        name,
                        span: block_span,
                        fields,
                    });
                }
                "enum" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    self.skip_block_body();
                    blocks.push(Block {
                        kind: BlockKind::Enum,
                        name,
                        span: block_span,
                        fields: BlockFields::default(),
                    });
                }
                "invariant" => {
                    self.read_word();
                    let rest = self.read_line();
                    let name = rest.trim().to_string();
                    blocks.push(Block {
                        kind: BlockKind::Invariant,
                        name,
                        span: block_span,
                        fields: BlockFields::default(),
                    });
                }
                "target" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    self.skip_block_body();
                    blocks.push(Block {
                        kind: BlockKind::Target,
                        name,
                        span: block_span,
                        fields: BlockFields::default(),
                    });
                }
                "schedule" => {
                    self.read_word();
                    let name = self.read_word().unwrap_or_default();
                    // read rest of schedule up to next top-level keyword or EOF
                    let sched_text = self.read_schedule_declaration();
                    let schedule_info =
                        parse_schedule_info(&sched_text, &self.path, block_span.line);
                    let mut fields = BlockFields::default();
                    fields.schedule_info = Some(schedule_info);
                    fields.raw_params = sched_text;
                    blocks.push(Block {
                        kind: BlockKind::Schedule,
                        name,
                        span: block_span,
                        fields,
                    });
                }
                _ => {
                    // Unknown token — advance past it
                    self.read_word();
                }
            }
        }
        blocks
    }

    /// Read text between `(` and `)` on the current line (inclusive).
    fn read_params_line(&mut self) -> String {
        self.skip_whitespace();
        if self.peek() == Some('(') {
            let mut result = String::new();
            let mut depth = 0i32;
            loop {
                match self.peek() {
                    None => break,
                    Some('(') => {
                        depth += 1;
                        result.push('(');
                        self.advance();
                    }
                    Some(')') => {
                        depth -= 1;
                        result.push(')');
                        self.advance();
                        if depth == 0 {
                            break;
                        }
                    }
                    Some(c) => {
                        result.push(c);
                        self.advance();
                    }
                }
            }
            result
        } else {
            String::new()
        }
    }

    /// Read params + return type line for flow declarations.
    /// Tracks `()`, `<>`, and `{}` depth to find the block-opening `{`.
    /// The block opener is a `{` at depth 0 across all bracket types.
    fn read_params_and_return_line(&mut self) -> String {
        self.skip_whitespace();
        let mut result = String::new();
        let mut paren_depth = 0i32;
        let mut angle_depth = 0i32;
        let mut brace_depth = 0i32;
        let mut past_params = false;

        loop {
            self.skip_whitespace();
            let total_depth = paren_depth + angle_depth + brace_depth;
            match self.peek() {
                None => break,
                // The real block opener: `{` when all depths are 0 and past params
                Some('{') if past_params && total_depth == 0 => break,
                Some('(') => {
                    paren_depth += 1;
                    result.push('(');
                    self.advance();
                }
                Some(')') => {
                    paren_depth -= 1;
                    result.push(')');
                    self.advance();
                    if paren_depth == 0 {
                        past_params = true;
                    }
                }
                Some('<') => {
                    angle_depth += 1;
                    result.push('<');
                    self.advance();
                }
                Some('>') => {
                    // Only count as close-angle if we have open angles
                    if angle_depth > 0 {
                        angle_depth -= 1;
                        result.push('>');
                        self.advance();
                    } else {
                        // Part of `->` or standalone `>` — just push it
                        result.push('>');
                        self.advance();
                    }
                }
                Some('{') => {
                    brace_depth += 1;
                    result.push('{');
                    self.advance();
                }
                Some('}') => {
                    brace_depth -= 1;
                    result.push('}');
                    self.advance();
                }
                Some(c) => {
                    result.push(c);
                    self.advance();
                }
            }
        }
        result
    }

    /// Read a type header: optional primitive word then optional `{...}` body.
    fn read_type_header(&mut self) -> String {
        // Consume only horizontal whitespace before the header — a newline
        // terminates a bodyless `type X primitive` declaration. Using the
        // newline-eating `skip_whitespace` here would silently absorb the
        // next type declaration into this one's header.
        self.skip_whitespace_no_newline();
        let mut result = String::new();
        // read until `{` or end of line if no `{`
        loop {
            self.skip_whitespace_no_newline();
            match self.peek() {
                None => break,
                Some('{') => {
                    let (body, _) = self.read_block_body();
                    result.push_str(" { ");
                    result.push_str(&body);
                    result.push_str(" }");
                    break;
                }
                Some('\n') => {
                    self.advance();
                    break;
                }
                Some(c) => {
                    result.push(c);
                    self.advance();
                }
            }
        }
        result
    }

    /// Read a schedule declaration (the `every/at/for` clauses, not a block body).
    /// Reads line by line until a top-level keyword, blank line followed by
    /// a top-level keyword, or EOF.
    fn read_schedule_declaration(&mut self) -> String {
        let top_level_keywords = [
            "prose",
            "actor",
            "external",
            "flow",
            "controller",
            "policy",
            "event",
            "type",
            "enum",
            "invariant",
            "target",
            "schedule",
        ];
        let mut result = String::new();
        // read params on the current line (up to \n)
        result.push_str(&self.read_params_line());
        // skip rest of line after params (shouldn't be anything)
        self.skip_whitespace_no_newline();
        if self.peek() == Some('\n') {
            self.advance();
        }

        // now read continuation lines
        loop {
            if self.rest().is_empty() {
                break;
            }

            // peek at this line without consuming
            let saved_pos = self.pos;
            let saved_line = self.line;
            // skip leading whitespace on the line (but NOT newlines between lines)
            while let Some(c) = self.peek() {
                if c == ' ' || c == '\t' {
                    self.advance();
                } else {
                    break;
                }
            }

            // check if this is an empty or comment line
            if self.peek() == Some('\n') || self.rest().starts_with("//") {
                // blank/comment line — skip it and check the next one
                self.read_line(); // skip line content
                continue;
            }

            // check if this line starts with a top-level keyword
            let first_word = self.peek_word().unwrap_or("").to_string();
            if top_level_keywords.contains(&first_word.as_str()) || self.rest().is_empty() {
                // done — restore to start of this line
                self.pos = saved_pos;
                self.line = saved_line;
                break;
            }

            // read the full line
            let line = self.read_line();
            result.push('\n');
            result.push_str(line.trim());
        }
        result
    }

    fn skip_whitespace_no_newline(&mut self) {
        while let Some(c) = self.peek() {
            if c == ' ' || c == '\t' || c == '\r' {
                self.advance();
            } else {
                break;
            }
        }
    }
}

pub fn parse_file(path: &Path, src: &str) -> ParsedFile {
    let mut parser = Parser::new(path, src);
    let blocks = parser.parse_blocks();
    ParsedFile {
        path: path.to_path_buf(),
        blocks,
    }
}

// ── Sub-parsers for block bodies ──────────────────────────────────────────

fn parse_prose_body(body: &str, path: &Path, start_line: usize) -> BlockFields {
    let mut fields = BlockFields::default();
    let mut line_num = start_line;
    let mut lines = body.lines().peekable();

    while let Some(line) = lines.next() {
        line_num += 1;
        let trimmed = line.trim();

        if trimmed.starts_with("intent:") {
            let rest = trimmed["intent:".len()..].trim();
            if rest.starts_with("\"\"\"") {
                // multi-line intent
                let mut intent_text = rest["\"\"\"".len()..].to_string();
                if !intent_text.contains("\"\"\"") {
                    loop {
                        if let Some(l) = lines.next() {
                            line_num += 1;
                            if l.contains("\"\"\"") {
                                let idx = l.find("\"\"\"").unwrap();
                                intent_text.push('\n');
                                intent_text.push_str(&l[..idx]);
                                break;
                            } else {
                                intent_text.push('\n');
                                intent_text.push_str(l);
                            }
                        } else {
                            break;
                        }
                    }
                } else {
                    let idx = intent_text.find("\"\"\"").unwrap();
                    intent_text = intent_text[..idx].to_string();
                }
                fields.intent = Some(intent_text.trim().to_string());
            } else {
                let val = rest.trim_matches('"').trim().to_string();
                if !val.is_empty() {
                    fields.intent = Some(val);
                }
            }
        } else if trimmed == "exports:" {
            // parse export items until we hit another keyword or empty
            while let Some(l) = lines.peek() {
                let lt = l.trim();
                if lt.is_empty() || lt.ends_with(':') || is_prose_section_header(lt) {
                    break;
                }
                let line = lines.next().unwrap();
                line_num += 1;
                let lt = line.trim();
                // pattern: `actor Name1, Name2` or `flow Name1, Name2`
                if let Some((kind, rest)) = parse_export_line(lt) {
                    for name in rest.split(',') {
                        let n = name.trim().to_string();
                        if !n.is_empty() {
                            fields.exports.push(ExportItem {
                                kind: kind.clone(),
                                name: n,
                                span: Span {
                                    file: path.to_path_buf(),
                                    line: line_num,
                                },
                            });
                        }
                    }
                }
            }
        } else if trimmed == "uses:" {
            while let Some(l) = lines.peek() {
                let lt = l.trim();
                if lt.is_empty() || is_prose_section_header(lt) {
                    break;
                }
                let line = lines.next().unwrap();
                line_num += 1;
                let lt = line.trim();
                fields
                    .uses_decls
                    .extend(parse_uses_line(lt, path, line_num));
            }
        } else if trimmed.starts_with("policies:") {
            let rest = trimmed["policies:".len()..].trim();
            fields
                .policies
                .extend(parse_policy_list(rest, path, line_num));
        }
    }
    fields
}

fn is_prose_section_header(s: &str) -> bool {
    matches!(
        s,
        "exports:" | "uses:" | "policies:" | "intent:" | "examples:"
    ) || s.ends_with(':') && s.len() < 20
}

fn parse_export_line(line: &str) -> Option<(String, String)> {
    let keywords = [
        "actor", "flow", "type", "event", "policy", "enum", "external",
    ];
    for kw in &keywords {
        if line.starts_with(kw) {
            let rest = line[kw.len()..].trim();
            return Some((kw.to_string(), rest.to_string()));
        }
    }
    None
}

fn parse_uses_line(line: &str, path: &Path, line_num: usize) -> Vec<UsesDecl> {
    // `feature X for OpName` or `feature X for OpA, OpB` or `feature X for event E`
    // or `external X for OpName`
    let (feature, ops_str) = if let Some(rest) = line.strip_prefix("feature ") {
        let parts: Vec<&str> = rest.splitn(2, " for ").collect();
        if parts.len() != 2 {
            return vec![];
        }
        let feature = parts[0].trim().to_string();
        let mut ops = parts[1].trim().to_string();
        if ops.starts_with("event ") {
            ops = ops["event ".len()..].trim().to_string();
        }
        (feature, ops)
    } else if let Some(rest) = line.strip_prefix("external ") {
        let parts: Vec<&str> = rest.splitn(2, " for ").collect();
        if parts.len() != 2 {
            return vec![];
        }
        let feature = parts[0].trim().to_string();
        let ops = parts[1].trim().to_string();
        (feature, ops)
    } else {
        return vec![];
    };

    // Split comma-separated op names
    ops_str
        .split(',')
        .map(|op| op.trim().to_string())
        .filter(|op| !op.is_empty())
        .map(|op| UsesDecl {
            feature: feature.clone(),
            op,
            span: Span {
                file: path.to_path_buf(),
                line: line_num,
            },
        })
        .collect()
}

fn parse_policy_list(s: &str, path: &Path, line_num: usize) -> Vec<PolicyRef> {
    // `[A, B, C]` or `A, B` etc.
    let inner = s.trim_start_matches('[').trim_end_matches(']');
    inner
        .split(',')
        .map(|p| {
            // strip params like `RoleGated(Admin)`
            let name = p.trim().split('(').next().unwrap_or("").trim().to_string();
            PolicyRef {
                name,
                span: Span {
                    file: path.to_path_buf(),
                    line: line_num,
                },
            }
        })
        .filter(|p| !p.name.is_empty())
        .collect()
}

fn parse_actor_body(body: &str, path: &Path, start_line: usize) -> BlockFields {
    let mut fields = BlockFields::default();
    let mut line_num = start_line;
    let mut in_state = false;
    let mut in_accepts = false;
    let mut accepts_depth = 0i32;
    let mut current_accepts: Option<AcceptsDecl> = None;
    for line in body.lines() {
        line_num += 1;
        let trimmed = line.trim();

        // Track accepts brace depth
        for c in trimmed.chars() {
            if c == '{' {
                accepts_depth += if in_accepts { 1 } else { 0 };
            }
            if c == '}' {
                if in_accepts {
                    accepts_depth -= 1;
                }
            }
        }

        if trimmed.starts_with("intent:") {
            let rest = trimmed["intent:".len()..]
                .trim()
                .trim_matches('"')
                .to_string();
            if !rest.starts_with("\"\"\"") {
                fields.intent = Some(rest);
            } else {
                fields.intent = Some(rest.replace("\"\"\"", "").trim().to_string());
            }
        } else if trimmed == "state {" || trimmed == "state{" {
            in_state = true;
        } else if in_state && trimmed == "}" {
            in_state = false;
        } else if in_state {
            if let Some(sf) = parse_state_field_line(trimmed, path, line_num) {
                fields.state_fields.push(sf);
            }
        } else if trimmed.starts_with("accepts ") {
            in_accepts = true;
            accepts_depth = 0;
            let name_part = &trimmed["accepts ".len()..];
            let name = name_part.split('(').next().unwrap_or("").trim().to_string();
            let has_key = name_part.contains("key:") || name_part.contains(", key");
            current_accepts = Some(AcceptsDecl {
                name,
                span: Span {
                    file: path.to_path_buf(),
                    line: line_num,
                },
                has_key_param: has_key,
                external_calls: Vec::new(),
            });
        } else if trimmed.starts_with("policies:") {
            let rest = trimmed["policies:".len()..].trim();
            fields
                .policies
                .extend(parse_policy_list(rest, path, line_num));
        }

        // detect closing of accepts block
        if in_accepts && accepts_depth < 0 {
            in_accepts = false;
            if let Some(acc) = current_accepts.take() {
                fields.accepts.push(acc);
            }
            accepts_depth = 0;
        }
    }
    // flush
    if let Some(acc) = current_accepts.take() {
        fields.accepts.push(acc);
    }

    fields
}

fn parse_state_field_line(line: &str, path: &Path, line_num: usize) -> Option<StateField> {
    // `fieldname: Type = default` or `fieldname: Type`
    // Strip inline comments first
    let line = if let Some(idx) = line.find("//") {
        line[..idx].trim()
    } else {
        line
    };
    if line.starts_with("//") || line.is_empty() {
        return None;
    }
    let (name_part, rest) = line.split_once(':')?;
    let name = name_part.trim().to_string();
    if name.contains(' ') {
        return None;
    } // not a field line
    let (type_part, default_part) = if let Some((t, d)) = rest.split_once('=') {
        (t, Some(d.trim().to_string()))
    } else {
        (rest, None)
    };
    let type_name = type_part.trim().to_string();
    if type_name.is_empty() {
        return None;
    }
    Some(StateField {
        name,
        type_name,
        default_value: default_part,
        span: Span {
            file: path.to_path_buf(),
            line: line_num,
        },
    })
}

fn parse_external_actor_body(body: &str, path: &Path, start_line: usize) -> BlockFields {
    let mut fields = BlockFields::default();
    let mut line_num = start_line;

    for line in body.lines() {
        line_num += 1;
        let trimmed = line.trim();
        if trimmed.starts_with("intent:") {
            let rest = trimmed["intent:".len()..]
                .trim()
                .trim_matches('"')
                .to_string();
            fields.intent = Some(rest);
        } else if trimmed.starts_with("providers:") {
            let rest = trimmed["providers:".len()..].trim();
            let inner = rest.trim_start_matches('[').trim_end_matches(']');
            fields.providers = inner
                .split(',')
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty())
                .collect();
        } else if trimmed.starts_with("accepts ") {
            let name_part = &trimmed["accepts ".len()..];
            let name = name_part.split('(').next().unwrap_or("").trim().to_string();
            let has_key = name_part.contains("key:") || name_part.contains(", key");
            fields.accepts.push(AcceptsDecl {
                name,
                span: Span {
                    file: path.to_path_buf(),
                    line: line_num,
                },
                has_key_param: has_key,
                external_calls: Vec::new(),
            });
        }
    }
    fields
}

fn parse_flow_body(body: &str, path: &Path, start_line: usize) -> BlockFields {
    let mut fields = BlockFields::default();
    let mut line_num = start_line;

    for line in body.lines() {
        line_num += 1;
        let trimmed = line.trim();
        if trimmed.starts_with("intent:") {
            let rest = trimmed["intent:".len()..]
                .trim()
                .trim_matches('"')
                .to_string();
            if !rest.is_empty() && !rest.starts_with("\"\"\"") {
                fields.intent = Some(rest);
            }
        } else if trimmed.starts_with("policies:") {
            let rest = trimmed["policies:".len()..].trim();
            fields
                .policies
                .extend(parse_policy_list(rest, path, line_num));
        } else if trimmed.contains("ask ") || trimmed.contains("tell ") {
            if let Some(ec) = parse_external_call_line(trimmed, path, line_num) {
                fields.external_calls.push(ec);
            }
        }
    }
    fields
}

fn parse_external_call_line(line: &str, path: &Path, line_num: usize) -> Option<ExternalCall> {
    // match `ask ActorName.Op(` or `ask ActorName[Tag].Op(`
    let after_ask = if let Some(r) = line.strip_prefix("ask ") {
        r.trim()
    } else if let Some(r) = line.find("ask ").map(|i| &line[i + 4..]) {
        r.trim()
    } else {
        return None;
    };
    // strip provider tag: ActorName[Tag].Op or ActorName.Op
    let actor_and_op = after_ask.split('(').next()?;
    let actor_and_op = if let Some(bracket) = actor_and_op.find('[') {
        // ActorName[Tag].Op — skip tag
        let actor = &actor_and_op[..bracket];
        let rest = actor_and_op
            .find(']')
            .map(|i| &actor_and_op[i + 1..])
            .unwrap_or("");
        if let Some(dot_pos) = rest.find('.') {
            let op = &rest[dot_pos + 1..];
            return Some(ExternalCall {
                actor: actor.trim().to_string(),
                op: op.trim().to_string(),
                span: Span {
                    file: path.to_path_buf(),
                    line: line_num,
                },
            });
        }
        return None;
    } else {
        actor_and_op
    };

    // ActorName.Op or ActorName(id).Op
    if let Some(dot_pos) = actor_and_op.rfind('.') {
        let actor_part = &actor_and_op[..dot_pos];
        // strip instance id: Actor(id) -> Actor
        let actor = actor_part.split('(').next()?.trim().to_string();
        let op = actor_and_op[dot_pos + 1..].trim().to_string();
        Some(ExternalCall {
            actor,
            op,
            span: Span {
                file: path.to_path_buf(),
                line: line_num,
            },
        })
    } else {
        None
    }
}

fn parse_controller_body(body: &str, path: &Path, start_line: usize) -> BlockFields {
    let mut fields = BlockFields::default();
    let mut line_num = start_line;

    for line in body.lines() {
        line_num += 1;
        let trimmed = line.trim();
        if trimmed.starts_with("policies:") {
            let rest = trimmed["policies:".len()..].trim();
            fields
                .policies
                .extend(parse_policy_list(rest, path, line_num));
        }
    }
    fields
}

fn parse_policy_body(body: &str, _path: &Path, _start_line: usize) -> BlockFields {
    let mut fields = BlockFields::default();

    for line in body.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("intent:") {
            let rest = trimmed["intent:".len()..]
                .trim()
                .trim_matches('"')
                .to_string();
            if !rest.is_empty() {
                fields.intent = Some(rest);
            }
        }
    }
    fields
}

fn parse_event_body(body: &str, path: &Path, start_line: usize) -> BlockFields {
    let mut fields = BlockFields::default();
    let mut line_num = start_line;
    let mut in_payload = false;

    for line in body.lines() {
        line_num += 1;
        let trimmed = line.trim();

        if trimmed.starts_with("payload:") {
            in_payload = true;
            // Handle inline: `payload: { f: T, ... }` or `payload: { f: T, ... }, delivery: ...`
            let after_payload = trimmed["payload:".len()..].trim();
            if let Some(inner_start) = after_payload.find('{') {
                let inner = &after_payload[inner_start + 1..];
                // find the closing `}`
                let inner = if let Some(end) = inner.find('}') {
                    &inner[..end]
                } else {
                    inner
                };
                for part in inner.split(',') {
                    let part = part.trim();
                    if part.is_empty() {
                        continue;
                    }
                    if let Some((fname, ftype)) = part.split_once(':') {
                        let ftype = ftype
                            .trim()
                            .trim_end_matches(',')
                            .trim_end_matches('?')
                            .to_string();
                        if !ftype.is_empty() {
                            fields.payload_fields.push(PayloadField {
                                name: fname.trim().to_string(),
                                type_name: ftype,
                                span: Span {
                                    file: path.to_path_buf(),
                                    line: line_num,
                                },
                            });
                        }
                    }
                }
                in_payload = false; // inline — done
            }
        } else if in_payload {
            if trimmed == "}" || (trimmed.starts_with("delivery") || trimmed.starts_with("order")) {
                in_payload = false;
            } else if trimmed.starts_with('{') {
                // opening of payload inline record
            } else if !trimmed.is_empty() && !trimmed.starts_with("//") {
                // field: Type
                if let Some((name, type_name)) = trimmed.split_once(':') {
                    let type_name = type_name.trim().trim_end_matches(',').to_string();
                    // clean up type (remove trailing `?`, etc. for resolution)
                    let base_type = type_name
                        .trim_end_matches('?')
                        .split('[')
                        .next()
                        .unwrap_or("")
                        .trim()
                        .to_string();
                    fields.payload_fields.push(PayloadField {
                        name: name.trim().to_string(),
                        type_name: base_type,
                        span: Span {
                            file: path.to_path_buf(),
                            line: line_num,
                        },
                    });
                }
            }
        }
    }
    fields
}

fn parse_type_header_and_body(header: &str, path: &Path, line_num: usize) -> BlockFields {
    let mut fields = BlockFields::default();
    // header is: optional primitive + optional `{ ... }` body
    let trimmed = header.trim();

    // check if float appears anywhere
    fields.has_float = trimmed.split_whitespace().any(|w| w == "float");

    // extract the primitive (first word before `{`)
    let before_brace = trimmed.split('{').next().unwrap_or("").trim();
    let primitive = before_brace
        .split_whitespace()
        .next()
        .unwrap_or("")
        .to_string();
    if !primitive.is_empty() && !primitive.starts_with('{') {
        fields.type_primitive = Some(primitive.clone());
    }

    // look for policies inside body
    if let Some(body_start) = trimmed.find('{') {
        let body = &trimmed[body_start + 1..];
        if let Some(body_end) = body.rfind('}') {
            let inner = &body[..body_end];
            for line in inner.lines() {
                let lt = line.trim();
                if lt.starts_with("policies:") {
                    let rest = lt["policies:".len()..].trim();
                    fields
                        .policies
                        .extend(parse_policy_list(rest, path, line_num));
                }
            }
        }
    }

    fields
}

fn parse_schedule_info(text: &str, path: &Path, line_num: usize) -> ScheduleInfo {
    let has_every = text.contains("every ");
    let has_at = text.contains("\nat ") || text.trim_start().starts_with("at ");
    let has_for = text.contains("for any ") || text.contains("for each ");

    // validate duration if `every` present
    let duration_valid = if has_every {
        if let Some(idx) = text.find("every ") {
            let after = &text[idx + 6..];
            let dur_str = after.split_whitespace().next().unwrap_or("");
            is_valid_duration(dur_str)
        } else {
            true
        }
    } else {
        true
    };

    ScheduleInfo {
        has_every_or_at: has_every || has_at,
        has_for_clause: has_for,
        duration_valid,
        span: Span {
            file: path.to_path_buf(),
            line: line_num,
        },
    }
}

pub fn is_valid_duration(s: &str) -> bool {
    // Valid: `\d+(d|h|m|s|ms)`
    if s.is_empty() {
        return false;
    }
    let (num_part, unit_part) = if let Some(i) = s.rfind(|c: char| c.is_ascii_digit()) {
        (&s[..=i], &s[i + 1..])
    } else {
        return false;
    };
    if !num_part.chars().all(|c| c.is_ascii_digit()) {
        return false;
    }
    matches!(unit_part, "d" | "h" | "m" | "s" | "ms")
}
