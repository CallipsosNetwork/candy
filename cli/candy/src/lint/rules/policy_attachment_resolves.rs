//! policy-attachment-resolves: every `policies: [A, B]` must reference a
//! declared `policy` block in the project.

use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};
use std::collections::HashSet;

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();

    let declared_policies: HashSet<String> = project
        .all_blocks()
        .into_iter()
        .filter(|b| b.kind == BlockKind::Policy)
        .map(|b| b.name.clone())
        .collect();

    for block in project.all_blocks() {
        for policy_ref in &block.fields.policies {
            if policy_ref.name.is_empty() {
                continue;
            }
            if !declared_policies.contains(&policy_ref.name) {
                violations.push(Violation {
                    rule: "policy-attachment-resolves".to_string(),
                    severity: "error".to_string(),
                    file: policy_ref.span.file.to_string_lossy().to_string(),
                    line: policy_ref.span.line,
                    message: format!(
                        "policy '{}' attached but not declared in project",
                        policy_ref.name
                    ),
                    context: format!("policies: [{}]", policy_ref.name),
                });
            }
        }
    }
    violations
}
