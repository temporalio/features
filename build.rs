use std::{
    env, fs, io,
    path::{Path, PathBuf},
};

fn main() -> io::Result<()> {
    println!("cargo:rerun-if-changed=features");

    let mut features = Vec::new();
    find_features(Path::new("features"), &mut features)?;
    features.sort();

    let mut modules = String::new();
    for (index, feature) in features.iter().enumerate() {
        let feature = fs::canonicalize(feature)?;
        let feature = feature
            .to_str()
            .ok_or_else(|| io::Error::other("Rust feature path is not valid UTF-8"))?;
        modules.push_str(&format!(
            "#[path = {feature:?}]\npub mod feature_{index};\n"
        ));
    }

    let out_dir = PathBuf::from(env::var_os("OUT_DIR").expect("OUT_DIR is set by Cargo"));
    fs::write(out_dir.join("rust_features.rs"), modules)
}

fn find_features(dir: &Path, features: &mut Vec<PathBuf>) -> io::Result<()> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        if entry.file_type()?.is_dir() {
            find_features(&entry.path(), features)?;
        } else if entry.file_name() == "feature.rs" {
            features.push(entry.path());
        }
    }
    Ok(())
}
