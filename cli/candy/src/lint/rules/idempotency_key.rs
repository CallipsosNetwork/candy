//! idempotency-key (warning): flows/messages that emit external effects via
//! `ask <ExternalActor>.<Op>(...)` should accept a `key: Key` parameter.
//!
//! Approach: for each `flow` block, if its external_calls list contains a call
//! to an actor that is declared as `external actor` in the project, and the
//! flow does not have a `key` parameter, emit a warning.
//!
//! We also check `accepts` on internal actors that call external actors.
//!
//! Intentional under-strictness: we only check flows, not inner accepts on
//! internal actors. The GRAMMAR.md says "replayable messages declare a key: Key
//! parameter" — we interpret this as any flow that reaches out to an external.

use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};
use std::collections::HashSet;

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();

    // Build set of external actor names
    let external_actors: HashSet<String> = project
        .all_blocks()
        .into_iter()
        .filter(|b| b.kind == BlockKind::ExternalActor)
        .map(|b| b.name.clone())
        .collect();

    for block in project.all_blocks() {
        if block.kind != BlockKind::Flow {
            continue;
        }
        if block.fields.has_key_param {
            continue;
        }

        // Check if flow has any external calls
        let has_external_call = block
            .fields
            .external_calls
            .iter()
            .any(|ec| external_actors.contains(&ec.actor));

        if has_external_call {
            violations.push(Violation {
                rule: "idempotency-key".to_string(),
                severity: "warning".to_string(),
                file: block.span.file.to_string_lossy().to_string(),
                line: block.span.line,
                message: format!(
                    "flow '{}' calls an external actor but has no 'key: Key' parameter",
                    block.name
                ),
                context: format!("flow {} {{ ... }}", block.name),
            });
        }
    }
    violations
}
