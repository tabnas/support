// Copyright (c) 2026 tabnas, MIT License

//! Shared test-support utilities for the tabnas parser system.
//!
//! Every tabnas package ships a canonical TypeScript implementation and
//! ports of it, and proves they agree by running **one** set of TSV
//! fixtures from `test/spec/` in every runtime. This crate is the
//! machinery that reads those fixtures in Rust, so there is one loader
//! with one set of rules instead of a copy per repository quietly
//! drifting from its TypeScript and Go twins.
//!
//! The TypeScript half is `@tabnas/support` and the Go half is
//! `github.com/tabnas/support/go`. The three are written to behave
//! identically: same escape codec, same comment and blank-line handling,
//! same `ERROR:<code>` contract, same value comparison. Where a
//! difference is unavoidable it is documented at the point it appears,
//! and collected in `doc/reference.md`.
//!
//! A typical suite is a few lines:
//!
//! ```no_run
//! use tabnas_support::{find_spec_dir, Failure, Runner, Value};
//!
//! let dir = find_spec_dir(None).expect("a test/spec directory above the crate");
//! Runner::new(|input| {
//!     // Parse with your grammar, then hand back a `Value` or a `Failure`
//!     // carrying the error code the fixture pins.
//!     let _ = input;
//!     Err(Failure::new("unexpected"))
//! })
//! .dir(dir.join("happy"));
//! ```
//!
//! This crate has **no required dependencies**, and never will: every
//! tabnas Rust crate would take it as a dev-dependency, so anything it
//! required would land in all of them. The adder grammar that exercises
//! it end to end therefore lives in the separate `adder/` crate, which
//! may depend on the engine.

use std::fmt;

mod census;
mod escape;
mod expect;
mod register;
mod runner;
mod spec;
mod value;

/// The release version, kept in step with `ts/package.json`. See
/// `tests/version_test.rs`, which fails the build when they drift.
pub const VERSION: &str = "0.3.4";

pub use census::{
    codes_in_spec_dir, compare_catalogues, coverage, CatalogueDiff, CensusOptions, CoverageReport,
};
pub use escape::{escape, unescape};
pub use expect::{
    equal_value, equal_value_with, error_code, error_expect, format_value, is_error_expect,
    lone_surrogate_at, lone_surrogate_message, parse_expect, ErrorExpect, ERROR_PREFIX,
};
pub use register::{no_divergences, Register};
pub use runner::{report, Failure, Runner};
pub use spec::{
    find_spec_dir, load_spec, load_spec_dir, parse_spec, Column, Row, SpecFile, SpecOptions,
};
pub use value::Value;

/// A defect in a fixture, a suite or a runner: an unreadable file, a
/// malformed expectation, a runner built without a parser. Carries the
/// same sentence the TypeScript and Go halves raise for the same
/// condition, so a failure reads the same whichever runtime reported it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Error(pub String);

impl fmt::Display for Error {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.0)
    }
}

impl std::error::Error for Error {}

impl From<String> for Error {
    fn from(message: String) -> Self {
        Error(message)
    }
}

impl From<&str> for Error {
    fn from(message: &str) -> Self {
        Error(message.to_string())
    }
}

/// The result type every fallible function here returns.
pub type Result<T> = std::result::Result<T, Error>;
