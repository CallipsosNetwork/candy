//! event-payload-types-resolve: every type used in an event payload must
//! resolve to a declared type/enum or a primitive.

use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();
    let known = project.declared_names();

    for block in project.all_blocks() {
        if block.kind != BlockKind::Event {
            continue;
        }

        for field in &block.fields.payload_fields {
            let base = base_type(&field.type_name);
            if base.is_empty() {
                continue;
            }
            // skip lowercase (obvious primitives like bool, string, int)
            if base.starts_with(|c: char| c.is_lowercase()) {
                continue;
            }
            if !known.contains_key(base) {
                violations.push(Violation {
                    rule: "event-payload-types-resolve".to_string(),
                    severity: "error".to_string(),
                    file: field.span.file.to_string_lossy().to_string(),
                    line: field.span.line,
                    message: format!(
                        "event '{}' payload field '{}' has unknown type '{}'",
                        block.name, field.name, field.type_name
                    ),
                    context: format!("{}: {}", field.name, field.type_name),
                });
            }
        }
    }
    violations
}

fn base_type(t: &str) -> &str {
    let t = t.trim().trim_end_matches('?');
    let t = if t.starts_with('[') {
        t.trim_start_matches('[').trim_end_matches(']')
    } else {
        t
    };
    t.split('<').next().unwrap_or(t).trim()
}
