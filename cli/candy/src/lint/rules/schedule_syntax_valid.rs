//! schedule-syntax-valid: every `schedule Name(...)` must have either
//! `every <duration>` or `at <expr>`, and a `for any X in Y where ...` clause.
//! Validate duration literal shape: `\d+(d|h|m|s|ms)`.

use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();

    for block in project.all_blocks() {
        if block.kind != BlockKind::Schedule {
            continue;
        }

        let info = match &block.fields.schedule_info {
            Some(i) => i,
            None => continue,
        };

        if !info.has_every_or_at {
            violations.push(Violation {
                rule: "schedule-syntax-valid".to_string(),
                severity: "error".to_string(),
                file: block.span.file.to_string_lossy().to_string(),
                line: block.span.line,
                message: format!(
                    "schedule '{}' is missing 'every <duration>' or 'at <expr>'",
                    block.name
                ),
                context: format!("schedule {}", block.name),
            });
        } else if !info.duration_valid {
            violations.push(Violation {
                rule: "schedule-syntax-valid".to_string(),
                severity: "error".to_string(),
                file: block.span.file.to_string_lossy().to_string(),
                line: block.span.line,
                message: format!(
                    "schedule '{}' has an invalid duration literal (expected: digits + d/h/m/s/ms)",
                    block.name
                ),
                context: format!("schedule {}", block.name),
            });
        }

        if !info.has_for_clause {
            violations.push(Violation {
                rule: "schedule-syntax-valid".to_string(),
                severity: "error".to_string(),
                file: block.span.file.to_string_lossy().to_string(),
                line: block.span.line,
                message: format!(
                    "schedule '{}' is missing a 'for any X in Y' clause",
                    block.name
                ),
                context: format!("schedule {}", block.name),
            });
        }
    }
    violations
}
