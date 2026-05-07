//! underscores-in-keywords: flag any identifier in "keyword position" that
//! contains an underscore.
//!
//! GRAMMAR.md hard rule: "No underscores in keywords. Compounds must find a
//! single word or compose two real ones. Underscores in keywords are drift."
//!
//! "Keyword position" means: block names (actor, flow, type, enum, event,
//! policy, controller, schedule), accepts message names, state field names,
//! and payload field names.
//!
//! We intentionally do NOT flag field names inside record types (e.g.
//! `type JournalEntry { counterpart: Id? }`) because snake_case field names
//! are common in payloads and the grammar doesn't explicitly forbid them.
//! We flag block-level names and message names as these are "keyword positions."
//!
//! TODO(underscores): revisit whether payload field names should be flagged
//! once GRAMMAR.md clarifies. Currently only block names and accepts names.

use crate::lint::output::Violation;
use crate::lint::parser::Project;

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();

    for block in project.all_blocks() {
        // Block name itself
        if has_underscore(&block.name) {
            violations.push(Violation {
                rule: "underscores-in-keywords".to_string(),
                severity: "error".to_string(),
                file: block.span.file.to_string_lossy().to_string(),
                line: block.span.line,
                message: format!(
                    "identifier '{}' in keyword position contains underscore",
                    block.name
                ),
                context: format!("{:?} {}", block.kind, block.name),
            });
        }

        // Accepts message names
        for acc in &block.fields.accepts {
            if has_underscore(&acc.name) {
                violations.push(Violation {
                    rule: "underscores-in-keywords".to_string(),
                    severity: "error".to_string(),
                    file: acc.span.file.to_string_lossy().to_string(),
                    line: acc.span.line,
                    message: format!("message name '{}' contains underscore", acc.name),
                    context: format!("accepts {}", acc.name),
                });
            }
        }
    }
    violations
}

fn has_underscore(s: &str) -> bool {
    s.contains('_') && !s.is_empty()
}
