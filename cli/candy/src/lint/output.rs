use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub struct Violation {
    pub rule: String,
    pub severity: String,
    pub file: String,
    pub line: usize,
    pub message: String,
    pub context: String,
}

pub fn exit_code(violations: &[Violation]) -> i32 {
    if violations.iter().any(|v| v.severity == "error") {
        return 2;
    }
    if violations.iter().any(|v| v.severity == "warning") {
        return 1;
    }
    0
}

pub fn print_violations(violations: &[Violation], json: bool) {
    for v in violations {
        if json {
            println!("{}", serde_json::to_string(v).unwrap_or_default());
        } else {
            let sev = if v.severity == "error" {
                "error"
            } else {
                "warning"
            };
            eprintln!("{}:{}: {}: {} [{}]", v.file, v.line, sev, v.message, v.rule);
        }
    }
}
