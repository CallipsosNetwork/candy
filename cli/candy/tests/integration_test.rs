use std::path::PathBuf;
use std::process::Command;

fn candy_bin() -> PathBuf {
    let mut bin = std::env::current_exe().unwrap();
    // strip `deps` and test binary name, navigate to target/debug/candy
    bin.pop();
    if bin.ends_with("deps") {
        bin.pop();
    }
    bin.push("candy");
    bin
}

fn run_lint(path: &str, json: bool) -> (i32, String, String) {
    let mut cmd = Command::new(candy_bin());
    cmd.arg("lint").arg(path);
    if json {
        cmd.arg("--json");
    }
    let out = cmd.output().expect("failed to run candy");
    let stdout = String::from_utf8_lossy(&out.stdout).to_string();
    let stderr = String::from_utf8_lossy(&out.stderr).to_string();
    let code = out.status.code().unwrap_or(-1);
    (code, stdout, stderr)
}

fn fixture_path(rel: &str) -> String {
    let mut p = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    p.push(rel);
    p.to_str().unwrap().to_string()
}

fn examples_path(rel: &str) -> String {
    let mut p = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    p.pop(); // up from candy/
    p.pop(); // up from cli/
    p.push("examples");
    p.push(rel);
    p.to_str().unwrap().to_string()
}

// ── Pass corpus (examples/) ────────────────────────────────────────────────

#[test]
fn auth_lints_clean_single_file() {
    let (code, _, stderr) = run_lint(&examples_path("auth/auth.candy"), false);
    assert_eq!(code, 0, "auth.candy should lint clean\nstderr: {stderr}");
}

#[test]
fn todo_lints_clean_single_file() {
    let (code, _, stderr) = run_lint(&examples_path("todo/todo.candy"), false);
    assert_eq!(code, 0, "todo.candy should lint clean\nstderr: {stderr}");
}

#[test]
fn wallet_lints_clean_single_file() {
    let (code, _, stderr) = run_lint(&examples_path("wallet/wallet.candy"), false);
    assert_eq!(code, 0, "wallet.candy should lint clean\nstderr: {stderr}");
}

#[test]
fn hello_lints_clean() {
    let (code, _, stderr) = run_lint(&examples_path("hello.candy"), false);
    assert_eq!(code, 0, "hello.candy should lint clean\nstderr: {stderr}");
}

#[test]
fn all_examples_lint_clean() {
    let examples = examples_path("");
    let (code, stdout, stderr) = run_lint(examples.trim_end_matches('/'), false);
    assert_eq!(
        code, 0,
        "all examples should lint clean\nstdout: {stdout}\nstderr: {stderr}"
    );
}

// ── Negative fixtures: each rule fires ─────────────────────────────────────

fn assert_rule_fires(rule: &str) {
    let path = fixture_path(&format!("tests/fixtures/fail/{rule}/"));
    let (code, stdout, stderr) = run_lint(&path, true);
    assert!(
        code > 0,
        "rule {rule} should produce violations\nstdout: {stdout}\nstderr: {stderr}"
    );
    let output = format!("{stdout}{stderr}");
    assert!(
        output.contains(&format!("\"rule\":\"{rule}\"")),
        "rule {rule} violation should appear in output\noutput: {output}"
    );
}

#[test]
fn rule_prose_required_intent() {
    assert_rule_fires("prose-required-intent");
}

#[test]
fn rule_money_no_floats() {
    assert_rule_fires("money-no-floats");
}

#[test]
fn rule_underscores_in_keywords() {
    assert_rule_fires("underscores-in-keywords");
}

#[test]
fn rule_schedule_syntax_valid() {
    assert_rule_fires("schedule-syntax-valid");
}

#[test]
fn rule_idempotency_key() {
    assert_rule_fires("idempotency-key");
}

#[test]
fn rule_broken_cross_feature_ref() {
    assert_rule_fires("broken-cross-feature-ref");
}

#[test]
fn rule_broken_symbol_ref() {
    assert_rule_fires("broken-symbol-ref");
}

#[test]
fn rule_policy_attachment_resolves() {
    assert_rule_fires("policy-attachment-resolves");
}

#[test]
fn rule_event_payload_types_resolve() {
    assert_rule_fires("event-payload-types-resolve");
}

#[test]
fn rule_actor_state_defaults_typed() {
    assert_rule_fires("actor-state-defaults-typed");
}

// ── Exit code contract ─────────────────────────────────────────────────────

#[test]
fn exit_code_0_for_clean() {
    let (code, _, _) = run_lint(&examples_path("hello.candy"), false);
    assert_eq!(code, 0);
}

#[test]
fn exit_code_2_for_errors() {
    let path = fixture_path("tests/fixtures/fail/money-no-floats/");
    let (code, _, _) = run_lint(&path, false);
    assert_eq!(code, 2);
}

#[test]
fn json_output_contains_rule_field() {
    let path = fixture_path("tests/fixtures/fail/prose-required-intent/");
    let (_, stdout, stderr) = run_lint(&path, true);
    let output = format!("{stdout}{stderr}");
    assert!(
        output.contains("\"rule\""),
        "JSON output should have rule field"
    );
    assert!(
        output.contains("\"severity\""),
        "JSON output should have severity field"
    );
    assert!(
        output.contains("\"file\""),
        "JSON output should have file field"
    );
    assert!(
        output.contains("\"line\""),
        "JSON output should have line field"
    );
}
