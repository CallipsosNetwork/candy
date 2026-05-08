use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();
    for block in project.all_blocks() {
        if block.kind != BlockKind::Prose {
            continue;
        }
        let missing = match &block.fields.intent {
            None => true,
            Some(s) if s.trim().is_empty() => true,
            _ => false,
        };
        if missing {
            violations.push(Violation {
                rule: "prose-required-intent".to_string(),
                severity: "error".to_string(),
                file: block.span.file.to_string_lossy().to_string(),
                line: block.span.line,
                message: "prose block is missing a non-empty intent:".to_string(),
                context: "prose { ... }".to_string(),
            });
        }
    }
    violations
}
