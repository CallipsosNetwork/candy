//! broken-symbol-ref: type/event/actor/flow/policy names used in declarations
//! must resolve to declared blocks or built-in primitives.
//!
//! Checked locations:
//!   - `policies: [Name]` on any block
//!   - `state { field: TypeName }` in actors
//!   - `payload: { field: TypeName }` in events (covered by event-payload-types-resolve)
//!
//! We intentionally do NOT walk expressions inside flow bodies — that would
//! require a full expression parser and is out of scope for v0.1. The rules
//! that do need expression-level information (idempotency-key, etc.) do so
//! with targeted heuristics.

use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();
    let known = project.declared_names();

    for block in project.all_blocks() {
        // Check policy refs on every block
        for policy_ref in &block.fields.policies {
            let name = &policy_ref.name;
            if name.is_empty() {
                continue;
            }
            if !known.contains_key(name.as_str()) {
                violations.push(Violation {
                    rule: "broken-symbol-ref".to_string(),
                    severity: "error".to_string(),
                    file: policy_ref.span.file.to_string_lossy().to_string(),
                    line: policy_ref.span.line,
                    message: format!("policy '{}' referenced but not declared", name),
                    context: format!("policies: [{}]", name),
                });
            }
        }

        // Check state field types in actors
        if block.kind == BlockKind::Actor {
            for sf in &block.fields.state_fields {
                let base_type = base_type_name(&sf.type_name);
                if base_type.is_empty() || base_type.starts_with(|c: char| c.is_lowercase()) {
                    continue; // skip obvious primitives and keywords
                }
                if !known.contains_key(base_type) {
                    violations.push(Violation {
                        rule: "broken-symbol-ref".to_string(),
                        severity: "error".to_string(),
                        file: sf.span.file.to_string_lossy().to_string(),
                        line: sf.span.line,
                        message: format!(
                            "type '{}' referenced in state but not declared",
                            base_type
                        ),
                        context: format!("{}: {}", sf.name, sf.type_name),
                    });
                }
            }
        }
    }
    violations
}

/// Strip list/optional wrappers to get the base type name.
fn base_type_name(t: &str) -> &str {
    let t = t.trim();
    // [T] -> T
    let t = if t.starts_with('[') {
        t.trim_start_matches('[').trim_end_matches(']')
    } else {
        t
    };
    // T? -> T
    let t = t.trim_end_matches('?');
    // Option<T> -> T
    let t = if let Some(inner) = t.strip_prefix("Option<") {
        inner.trim_end_matches('>')
    } else {
        t
    };
    // Result<Ok, Err> — just take the name itself
    t.split('<').next().unwrap_or(t).trim()
}
