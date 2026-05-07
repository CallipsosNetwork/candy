pub mod output;
pub mod parser;
pub mod rules;

use std::path::{Path, PathBuf};
use walkdir::WalkDir;

use output::print_violations;
use parser::Project;

pub fn run(path: &Path, json: bool) -> i32 {
    let (files, is_project) = collect_candy_files(path);
    if files.is_empty() {
        if path.is_file() && path.extension().map(|e| e != "candy").unwrap_or(true) {
            eprintln!("candy lint: {} is not a .candy file", path.display());
            return 2;
        }
        // Empty dir is fine — no files = nothing to lint
        return 0;
    }

    let project = match Project::load(&files) {
        Ok(p) => p,
        Err(e) => {
            eprintln!("candy lint: parse error: {e}");
            return 2;
        }
    };

    let violations = rules::run_all(&project, is_project);
    let exit_code = output::exit_code(&violations);
    print_violations(&violations, json);
    exit_code
}

/// Returns (files, is_project_mode).
/// is_project_mode = true when linting a directory (all files loaded).
fn collect_candy_files(path: &Path) -> (Vec<PathBuf>, bool) {
    if path.is_file() {
        if path.extension().map(|e| e == "candy").unwrap_or(false) {
            return (vec![path.to_path_buf()], false);
        }
        return (vec![], false);
    }
    let files = WalkDir::new(path)
        .follow_links(true)
        .into_iter()
        .filter_map(|e| e.ok())
        .filter(|e| e.file_type().is_file())
        .filter(|e| e.path().extension().map(|x| x == "candy").unwrap_or(false))
        .map(|e| e.path().to_path_buf())
        .collect();
    (files, true)
}
