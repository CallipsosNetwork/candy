//! actor-state-defaults-typed (warning): warn if a state field default is
//! obviously the wrong primitive type.
//!
//! Conservative checks only:
//!   - declared type is `int`, `Money`, or any type with known int semantic,
//!     but default is a quoted string literal → warn
//!   - declared type is `string` or `Email` but default is a bare integer → warn
//!   - declared type is `bool` but default is something other than true/false → warn
//!
//! We do NOT try to resolve custom type aliases (e.g. `type Count int`) —
//! that would require type-resolution which is out of v0.1 scope.

use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();

    for block in project.all_blocks() {
        if block.kind != BlockKind::Actor {
            continue;
        }

        for sf in &block.fields.state_fields {
            let default = match &sf.default_value {
                Some(d) => d.trim().to_string(),
                None => continue,
            };
            let type_base = base_type_name(&sf.type_name);

            // bool fields: default must be true or false
            if type_base == "bool" && default != "true" && default != "false" {
                violations.push(Violation {
                    rule: "actor-state-defaults-typed".to_string(),
                    severity: "warning".to_string(),
                    file: sf.span.file.to_string_lossy().to_string(),
                    line: sf.span.line,
                    message: format!(
                        "field '{}' has type 'bool' but default '{}' is not true/false",
                        sf.name, default
                    ),
                    context: format!("{}: {} = {}", sf.name, sf.type_name, default),
                });
            }

            // int/numeric fields: default must not be a quoted string
            if matches!(type_base, "int" | "Money" | "Duration") && default.starts_with('"') {
                {
                    violations.push(Violation {
                        rule: "actor-state-defaults-typed".to_string(),
                        severity: "warning".to_string(),
                        file: sf.span.file.to_string_lossy().to_string(),
                        line: sf.span.line,
                        message: format!(
                            "field '{}' has numeric type '{}' but default is a string literal",
                            sf.name, sf.type_name
                        ),
                        context: format!("{}: {} = {}", sf.name, sf.type_name, default),
                    });
                }
            }

            // string fields: default must not be a bare integer
            if matches!(type_base, "string" | "Email" | "Password") {
                let is_bare_int =
                    default.chars().all(|c| c.is_ascii_digit()) && !default.is_empty();
                if is_bare_int {
                    violations.push(Violation {
                        rule: "actor-state-defaults-typed".to_string(),
                        severity: "warning".to_string(),
                        file: sf.span.file.to_string_lossy().to_string(),
                        line: sf.span.line,
                        message: format!(
                            "field '{}' has string type '{}' but default '{}' looks like an integer",
                            sf.name, sf.type_name, default
                        ),
                        context: format!("{}: {} = {}", sf.name, sf.type_name, default),
                    });
                }
            }
        }
    }
    violations
}

fn base_type_name(t: &str) -> &str {
    let t = t.trim().trim_end_matches('?');
    let t = if t.starts_with('[') {
        t.trim_start_matches('[').trim_end_matches(']')
    } else {
        t
    };
    t.split('<').next().unwrap_or(t).trim()
}
