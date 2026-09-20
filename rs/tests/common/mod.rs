// Shared by every integration test binary. Cargo compiles this module
// into each one, so an item only one binary uses is dead code in the
// others; the allow keeps that from being a warning.
#![allow(dead_code)]

use std::path::{Path, PathBuf};

use tabnas_support::find_spec_dir;

/// The shared fixture directory, found by walking up from the crate.
pub fn spec_dir() -> PathBuf {
    find_spec_dir(Some(Path::new(env!("CARGO_MANIFEST_DIR")))).expect("a test/spec directory")
}
