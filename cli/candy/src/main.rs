use clap::{Parser, Subcommand};

mod lint;

#[derive(Parser)]
#[command(name = "candy", version, about = "candy spec language toolchain")]
struct Cli {
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Lint candy spec files for errors and warnings
    Lint {
        /// File or directory to lint
        path: std::path::PathBuf,
        /// Output violations as NDJSON (one JSON object per line)
        #[arg(long)]
        json: bool,
    },
    /// Generate backend code from a candy spec (not yet implemented; see issue #13)
    Gen,
    /// Run conformance tests against a generated backend (not yet implemented; see issue #17)
    Test,
    /// Format candy spec files (not yet implemented; see issue #39)
    Fmt,
}

fn main() {
    let cli = Cli::parse();

    match cli.command {
        Command::Lint { path, json } => {
            std::process::exit(lint::run(&path, json));
        }
        Command::Gen => {
            eprintln!("gen: not yet implemented; see issue #13");
            std::process::exit(1);
        }
        Command::Test => {
            eprintln!("test: not yet implemented; see issue #17");
            std::process::exit(1);
        }
        Command::Fmt => {
            eprintln!("fmt: not yet implemented; see issue #39");
            std::process::exit(1);
        }
    }
}
