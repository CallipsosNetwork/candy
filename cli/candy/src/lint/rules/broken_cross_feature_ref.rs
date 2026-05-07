//! broken-cross-feature-ref: every `uses: feature X for Op` must resolve to
//! an export from feature X.
//!
//! Interpretation: we build a table of (feature_name -> exported symbols) by
//! scanning all prose blocks. A "feature name" is the file stem (for
//! single-file features) or the directory name (for folder features). We then
//! check that every `uses: feature X for Op` has `Op` in X's export list.
//!
//! Ambiguity: GRAMMAR.md allows `uses: external X for Op` which references an
//! external actor, not a feature. We skip `external` uses lines here (they
//! would false-positive). Only `feature X for Op` is checked.

use crate::lint::output::Violation;
use crate::lint::parser::Project;

pub fn check(project: &Project) -> Vec<Violation> {
    let mut violations = Vec::new();
    let exports = project.feature_exports();

    for file in &project.files {
        for block in &file.blocks {
            for uses in &block.fields.uses_decls {
                // Only check `feature X for Op`; external refs are skipped.
                if let Some(feature_exports) = exports.get(&uses.feature) {
                    if !feature_exports.contains(&uses.op) {
                        violations.push(Violation {
                            rule: "broken-cross-feature-ref".to_string(),
                            severity: "error".to_string(),
                            file: uses.span.file.to_string_lossy().to_string(),
                            line: uses.span.line,
                            message: format!(
                                "feature '{}' does not export '{}'",
                                uses.feature, uses.op
                            ),
                            context: format!("uses: feature {} for {}", uses.feature, uses.op),
                        });
                    }
                }
                // If the feature doesn't exist at all in the project, we skip it —
                // it may be an external feature not included in the lint path.
            }
        }
    }
    violations
}
