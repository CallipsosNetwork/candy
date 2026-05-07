mod actor_state_defaults_typed;
mod broken_cross_feature_ref;
mod broken_symbol_ref;
mod event_payload_types_resolve;
mod idempotency_key;
mod money_no_floats;
mod policy_attachment_resolves;
mod prose_required_intent;
mod schedule_syntax_valid;
mod underscores_in_keywords;

use crate::lint::output::Violation;
use crate::lint::parser::Project;

/// Run all lint rules against the project.
/// `is_project`: true when linting a directory (all project files loaded).
/// Cross-file resolution rules (broken-symbol-ref for policies,
/// policy-attachment-resolves, broken-cross-feature-ref) only run in project
/// mode because single-file linting can't resolve cross-file declarations.
pub fn run_all(project: &Project, is_project: bool) -> Vec<Violation> {
    let mut violations = Vec::new();
    violations.extend(prose_required_intent::check(project));
    violations.extend(money_no_floats::check(project));
    violations.extend(schedule_syntax_valid::check(project));
    violations.extend(underscores_in_keywords::check(project));
    violations.extend(event_payload_types_resolve::check(project));
    violations.extend(actor_state_defaults_typed::check(project));
    violations.extend(idempotency_key::check(project));

    if is_project {
        violations.extend(broken_cross_feature_ref::check(project));
        violations.extend(broken_symbol_ref::check(project));
        violations.extend(policy_attachment_resolves::check(project));
    }

    violations
}
