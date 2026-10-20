use std::{
    env,
    fmt::Write,
    fs,
    path::{Path, PathBuf},
};

fn discover(dir: &Path, sources: &mut Vec<PathBuf>) {
    for entry in fs::read_dir(dir).expect("read feature directory") {
        let entry = entry.expect("read feature entry");
        let path = entry.path();
        if entry.file_type().expect("feature file type").is_dir() {
            discover(&path, sources);
        } else if entry.file_name() == "feature.rs" {
            sources.push(path);
        }
    }
}

fn main() {
    // Both the checked-in package and prepared packages live beneath the repo.
    // Locate its feature tree by identity, not by a fixed number of parents.
    let manifest_dir = PathBuf::from(env::var_os("CARGO_MANIFEST_DIR").unwrap());
    let root = manifest_dir
        .ancestors()
        .find(|dir| dir.join("features").is_dir() && dir.join("harness/rust/Cargo.toml").is_file())
        .expect("Rust feature package must be beneath the features repository");
    let feature_dir = root.join("features");
    // Cargo recursively watches the tree, including newly added feature modules.
    println!("cargo:rerun-if-changed={}", feature_dir.display());
    let mut sources = Vec::new();
    discover(&feature_dir, &mut sources);
    sources.sort();
    let mut registry = String::new();
    let mut entries = String::new();
    for source in sources {
        let key = source
            .parent()
            .unwrap()
            .strip_prefix(&feature_dir)
            .unwrap()
            .components()
            .map(|part| part.as_os_str().to_str().expect("UTF-8 feature path"))
            .collect::<Vec<_>>()
            .join("/");
        // Hex encoding is injective, even for punctuation and Unicode paths.
        let mut module = String::from("feature_");
        for byte in key.bytes() {
            write!(module, "{byte:02x}").unwrap();
        }
        writeln!(
            registry,
            "#[path = {:?}]\npub mod {module};",
            source.to_str().unwrap()
        )
        .unwrap();
        writeln!(entries, "        ({key:?}, {module}::feature()),").unwrap();
    }
    write!(
        registry,
        "\npub fn features() -> crate::harness::Registry {{\n    vec![\n{entries}    ]\n}}\n"
    )
    .unwrap();
    fs::write(
        PathBuf::from(env::var_os("OUT_DIR").unwrap()).join("registry.rs"),
        registry,
    )
    .expect("write feature registry");
}
