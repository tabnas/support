// The crate's version is one of the release sites that must agree. A
// bump that updates ts/package.json and forgets these fails here rather
// than shipping a crate whose version disagrees with the package it is a
// port of. Mirrors ts/test/version.test.js and go/version_test.go.

use std::fs;
use std::path::Path;

use tabnas_support::Value;

fn repo_root() -> &'static Path {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("rs/ has a parent")
}

#[test]
fn version_looks_like_a_semver() {
    let parts: Vec<&str> = tabnas_support::VERSION.split('.').collect();
    assert_eq!(
        parts.len(),
        3,
        "VERSION is not x.y.z: {}",
        tabnas_support::VERSION
    );
    for part in parts {
        assert!(
            !part.is_empty() && part.chars().all(|c| c.is_ascii_digit()),
            "VERSION segment is not numeric: {}",
            tabnas_support::VERSION
        );
    }
}

#[test]
fn version_matches_cargo_toml() {
    let manifest = fs::read_to_string(Path::new(env!("CARGO_MANIFEST_DIR")).join("Cargo.toml"))
        .expect("the manifest is readable");
    let declared = manifest
        .lines()
        .find_map(|line| line.strip_prefix("version = \""))
        .and_then(|rest| rest.split('"').next())
        .expect("the manifest declares a version");
    assert_eq!(
        declared,
        tabnas_support::VERSION,
        "Cargo.toml disagrees with VERSION"
    );
}

#[test]
fn version_matches_package_json() {
    let package = fs::read_to_string(repo_root().join("ts").join("package.json"))
        .expect("ts/package.json is readable");
    let declared = Value::parse_json(&package).expect("ts/package.json is JSON");
    assert_eq!(
        declared.get("version").and_then(Value::as_str),
        Some(tabnas_support::VERSION),
        "ts/package.json disagrees with VERSION"
    );
}

#[test]
fn adder_crate_matches_too() {
    // The adder crate is a separate package; its version is the same
    // release site, so it is held to the same number.
    let manifest = fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("adder")
            .join("Cargo.toml"),
    )
    .expect("adder/Cargo.toml is readable");
    let declared = manifest
        .lines()
        .find_map(|line| line.strip_prefix("version = \""))
        .and_then(|rest| rest.split('"').next())
        .expect("adder/Cargo.toml declares a version");
    assert_eq!(
        declared,
        tabnas_support::VERSION,
        "adder/Cargo.toml disagrees with VERSION"
    );
}
