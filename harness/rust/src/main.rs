pub mod harness;
mod options;
mod summary;

include!(concat!(env!("OUT_DIR"), "/registry.rs"));

#[tokio::main]
async fn main() -> std::process::ExitCode {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env().unwrap_or_else(|_| "warn".into()),
        )
        .with_writer(std::io::stderr)
        .init();
    match harness::run(std::env::args().skip(1), features()).await {
        Ok(true) => std::process::ExitCode::SUCCESS,
        Ok(false) => std::process::ExitCode::FAILURE,
        Err(error) => {
            eprintln!("Rust harness: {error:#}");
            std::process::ExitCode::FAILURE
        }
    }
}
