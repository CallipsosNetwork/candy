//! money-no-floats: forbid the keyword `float` in any `type` block body.
//!
//! GRAMMAR.md §hard-rules: "No floats for money. Money is integer minor units;
//! currency is pinned in the type declaration."
//!
//! The underlying primitives listed in GRAMMAR.md are: int, string, opaque,
//! bool, bytes, instant, decimal. `float` is NOT in that list. We flag any
//! `type` block whose parsed primitive is `float` or whose body contains the
//! word `float`.

use crate::lint::output::Violation;
use crate::lint::parser::{BlockKind, Project};

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();
    for block in project.all_blocks() {
        if block.kind != BlockKind::Type {
            continue;
        }
        if block.fields.has_float {
            violations.push(Violation {
                rule: "money-no-floats".to_string(),
                severity: "error".to_string(),
                file: block.span.file.to_string_lossy().to_string(),
                line: block.span.line,
                message: format!(
                    "type '{}' uses 'float' — use 'int' with unit:minor for money types",
                    block.name
                ),
                context: format!("type {} ...", block.name),
            });
        }
        if let Some(prim) = &block.fields.type_primitive {
            if prim == "float" {
                // already covered by has_float, but just in case
                if !block.fields.has_float {
                    violations.push(Violation {
                        rule: "money-no-floats".to_string(),
                        severity: "error".to_string(),
                        file: block.span.file.to_string_lossy().to_string(),
                        line: block.span.line,
                        message: format!(
                            "type '{}' declares primitive 'float' — use 'int' with unit:minor",
                            block.name
                        ),
                        context: format!("type {} float ...", block.name),
                    });
                }
            }
        }
    }
    violations
}
