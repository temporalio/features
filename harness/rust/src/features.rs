// The build script discovers and includes every features/**/feature.rs file.
include!(concat!(env!("OUT_DIR"), "/rust_features.rs"));
