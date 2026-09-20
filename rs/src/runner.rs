// Copyright (c) 2026 tabnas, MIT License

// The table-driven runner: fixture rows in, failures out.
//
// Loading a fixture is only half of what every repo was duplicating. The
// other half is the loop: read the input column, parse it, branch on
// whether the expected column names a value or an error, compare, and
// report with enough context to find the row. That loop is the same
// everywhere, and `ts/src/runner.ts` and `go/runner.go` run the identical
// one against `node:test` and `testing.T`.
//
// Rust's test harness has no subtests, so this runner reports every
// failing row of a fixture at once, each line carrying `<file>:<line>`,
// rather than one case per row. That is the one shape difference, and it
// is recorded in `doc/reference.md`.

use std::cell::RefCell;
use std::fmt;
use std::path::Path;
use std::rc::Rc;

use crate::expect::{
    equal_value, equal_value_with, error_expect, format_value, is_error_expect, lone_surrogate_at,
    lone_surrogate_message, parse_expect,
};
use crate::spec::{load_spec, load_spec_dir, Column, Row, SpecFile, SpecOptions};
use crate::value::Value;
use crate::{Error, Result};

/// A parse that failed, as the runner sees it: the code the fixture pins,
/// the position it may pin, and the message a failure report quotes.
///
/// This crate cannot name the engine's error type (it takes no
/// dependencies), so a suite converts at the boundary, which is a single
/// closure: `.map_err(|e| Failure::new(e.code).at(e.row, e.col))`. That
/// stands in for the `errorCode` and `errorPos` hooks of the other two
/// runtimes, whose runners read those fields off the thrown error by
/// shape.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Failure {
    /// The error code, or `""` when the failure has none.
    pub code: String,
    /// The 1-based source position, when the error reports one.
    pub row: Option<usize>,
    pub col: Option<usize>,
    /// The rendered error, quoted in failure reports.
    pub message: String,
}

impl Failure {
    /// A failure carrying a code and no position.
    pub fn new(code: impl Into<String>) -> Self {
        let code = code.into();
        Failure {
            message: code.clone(),
            code,
            row: None,
            col: None,
        }
    }

    /// A failure with no code at all, only a message: what a parser whose
    /// errors are told apart by their text alone produces.
    pub fn message(message: impl Into<String>) -> Self {
        Failure {
            code: String::new(),
            row: None,
            col: None,
            message: message.into(),
        }
    }

    /// Record the 1-based source position the error reports.
    pub fn at(mut self, row: usize, col: usize) -> Self {
        self.row = Some(row);
        self.col = Some(col);
        self
    }

    /// Replace the message a report quotes.
    pub fn with_message(mut self, message: impl Into<String>) -> Self {
        self.message = message.into();
        self
    }
}

impl fmt::Display for Failure {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        if self.message.is_empty() {
            formatter.write_str(&self.code)
        } else {
            formatter.write_str(&self.message)
        }
    }
}

impl std::error::Error for Failure {}

type ParseFn = dyn Fn(&str, &Row) -> std::result::Result<Value, Failure>;
type ParseHook = Rc<ParseFn>;
type MatchErrorHook = Rc<dyn Fn(&Failure, &str, &Row) -> bool>;
pub(crate) type ParseExpectedFn = dyn Fn(&str, &Row) -> Result<Value>;
type ParseExpectedHook = Rc<ParseExpectedFn>;
pub(crate) type NormalizeFn = dyn Fn(Value) -> Value;
type NormalizeHook = Rc<NormalizeFn>;

/// Drives fixture rows through one parser. Reuse it across files.
///
/// Built with [`Runner::new`] (a parse hook taking the input) or
/// [`Runner::new_with_row`] (one taking the row as well, for a fixture
/// whose other columns take part in the parse, an `opts` column of plugin
/// options being the common one), then adjusted with the builder methods.
#[derive(Clone)]
pub struct Runner {
    parse: ParseHook,
    match_error: Option<MatchErrorHook>,
    parse_expected: Option<ParseExpectedHook>,
    normalize: Option<NormalizeHook>,
    input: Column,
    expected: Column,
    load: SpecOptions,
}

impl Runner {
    /// A runner over a parse hook that sees only the input.
    pub fn new(parse: impl Fn(&str) -> std::result::Result<Value, Failure> + 'static) -> Self {
        Self::new_with_row(move |input, _row| parse(input))
    }

    /// A runner over a parse hook that sees the row as well.
    pub fn new_with_row(
        parse: impl Fn(&str, &Row) -> std::result::Result<Value, Failure> + 'static,
    ) -> Self {
        Runner {
            parse: Rc::new(parse),
            match_error: None,
            parse_expected: None,
            normalize: None,
            input: Column::Position(0),
            expected: Column::Position(1),
            load: SpecOptions::default(),
        }
    }

    /// Decide whether a failure satisfies an `ERROR:<want>` row, when
    /// comparing a code cannot. It replaces the code comparison entirely.
    ///
    /// A code is the contract this crate prefers, but some grammars have
    /// no stable code to pin: a parser whose failures are distinguished
    /// only by their message, or a fixture that names a position rather
    /// than a kind. Those fixtures would otherwise have to weaken to a
    /// bare `ERROR`, which asserts nothing more than "it failed".
    ///
    /// A bare `ERROR` cell still means "any error", and does not reach
    /// here. Nor does a trailing `@<row>:<col>`: `want` is the code with
    /// any position expectation already stripped.
    pub fn match_error(mut self, hook: impl Fn(&Failure, &str, &Row) -> bool + 'static) -> Self {
        self.match_error = Some(Rc::new(hook));
        self
    }

    /// Read the expected cell, when the fixture's vocabulary is wider than
    /// JSON (JSON5's `NaN` and `Infinity`, or the `UNDEFINED` several repos
    /// use for "no value at all"). It replaces [`parse_expect`], and is
    /// reached only for a value row. Call `parse_expect` for the cells the
    /// hook does not claim, so the ordinary rows keep the ordinary rules.
    pub fn parse_expected(mut self, hook: impl Fn(&str, &Row) -> Result<Value> + 'static) -> Self {
        self.parse_expected = Some(Rc::new(hook));
        self
    }

    /// Rewrite values before comparison; see [`equal_value_with`].
    pub fn normalize(mut self, hook: impl Fn(Value) -> Value + 'static) -> Self {
        self.normalize = Some(Rc::new(hook));
        self
    }

    /// The column holding the parser input, by position or header name.
    /// Default: position 0.
    pub fn input(mut self, column: impl Into<Column>) -> Self {
        self.input = column.into();
        self
    }

    /// The column holding the expected result. Default: position 1.
    pub fn expected(mut self, column: impl Into<Column>) -> Self {
        self.expected = column.into();
        self
    }

    /// Fixture-file loading options, passed through to the loader.
    pub fn load(mut self, options: SpecOptions) -> Self {
        self.load = options;
        self
    }

    /// The loading options in force.
    pub fn load_options(&self) -> &SpecOptions {
        &self.load
    }

    /// The input column selector in force.
    pub fn input_column(&self) -> &Column {
        &self.input
    }

    pub(crate) fn parse_expected_hook(&self) -> Option<&ParseExpectedFn> {
        self.parse_expected.as_deref()
    }

    pub(crate) fn normalize_hook(&self) -> Option<&NormalizeFn> {
        self.normalize.as_deref()
    }

    /// A copy of this runner whose expected column is `column`.
    pub(crate) fn reading(&self, column: Column) -> Runner {
        let mut runner = self.clone();
        runner.expected = column;
        runner
    }

    /// A copy of this runner whose parse hook answers from ONE evaluation
    /// however many times it is asked. The register needs it: every
    /// comparison a row needs goes through the ordinary runner, but a
    /// parse hook that carries state could answer differently on a second
    /// call and turn a genuine regression into a reported "closed
    /// divergence", the opposite conclusion.
    pub(crate) fn parsing_once(&self) -> Runner {
        let inner = Rc::clone(&self.parse);
        let once: Rc<RefCell<Option<std::result::Result<Value, Failure>>>> =
            Rc::new(RefCell::new(None));
        let mut runner = self.clone();
        runner.parse = Rc::new(move |input, row| {
            let mut slot = once.borrow_mut();
            if slot.is_none() {
                *slot = Some(inner(input, row));
            }
            slot.as_ref().expect("filled above").clone()
        });
        runner
    }

    /// Why a fixture cannot be run, or `Ok` when it can. [`Runner::spec`]
    /// calls it and fails; it is public so the guard itself can be
    /// asserted.
    ///
    /// A fixture that loads but holds no rows is the case that matters: it
    /// is a silent pass, and a silent pass is indistinguishable from
    /// coverage that was never there.
    pub fn check_spec(&self, spec: &SpecFile) -> Result<()> {
        let Some(probe) = spec.rows.first() else {
            return Err(Error(format!("{}: no cases", spec.file)));
        };
        probe
            .resolve(&self.input)
            .map_err(|error| Error(format!("{}: input column: {error}", spec.file)))?;
        probe
            .resolve(&self.expected)
            .map_err(|error| Error(format!("{}: expected column: {error}", spec.file)))?;
        Ok(())
    }

    /// Run one row and return the failure rather than reporting it, for a
    /// suite that does its own reporting.
    pub fn check_row(&self, row: &Row, input: &str, expected: &str) -> Result<()> {
        let at = row.location();

        if is_error_expect(expected) {
            let want = error_expect(expected).map_err(|error| Error(format!("{at}: {error}")))?;

            let failure = match (self.parse)(input, row) {
                Ok(got) => {
                    return Err(Error(format!(
                        "{at}: parse({input:?}) should fail with {expected}, but returned {}",
                        format_value(&got)
                    )));
                }
                Err(failure) => failure,
            };

            if !want.code.is_empty() {
                match &self.match_error {
                    Some(matches) => {
                        if !matches(&failure, &want.code, row) {
                            return Err(Error(format!(
                                "{at}: parse({input:?}) failed, but the error does not match {:?}\n  error: {failure}",
                                want.code
                            )));
                        }
                    }
                    None => {
                        if failure.code != want.code {
                            return Err(Error(format!(
                                "{at}: parse({input:?}) failed with code {:?}, expected {:?}\n  error: {failure}",
                                failure.code, want.code
                            )));
                        }
                    }
                }
            }

            if let (Some(want_row), Some(want_col)) = (want.row, want.col) {
                // An error that names no position is a mismatch, not a
                // pass. The whole point of the channel is that an error
                // which cannot say where it happened has not met an
                // expectation that says where it must.
                let reported = match (failure.row, failure.col) {
                    (Some(row), Some(col)) => Some((row, col)),
                    _ => None,
                };
                if reported != Some((want_row, want_col)) {
                    let where_ = reported.map_or("no position".to_string(), |(row, col)| {
                        format!("{row}:{col}")
                    });
                    return Err(Error(format!(
                        "{at}: parse({input:?}) failed at {where_}, expected {want_row}:{want_col}\n  error: {failure}"
                    )));
                }
            }

            return Ok(());
        }

        // Refuse a shared cell that the runtimes would decode differently.
        // `lone_surrogate_at` says why; the short version is that this is
        // the one thing a shared expected column cannot express, and that
        // it fails SILENTLY. Only on the DEFAULT path: a wider vocabulary
        // need not read `\uXXXX` as an escape at all, and a hook whose
        // syntax does should call `lone_surrogate_at` itself.
        if self.parse_expected.is_none() {
            if let Some(index) = lone_surrogate_at(expected) {
                return Err(Error(format!(
                    "{at}: {}",
                    lone_surrogate_message(expected, index)
                )));
            }
        }

        let want = match &self.parse_expected {
            Some(read) => read(expected, row),
            None => parse_expect(expected),
        }
        .map_err(|error| Error(format!("{at}: {error}")))?;

        // A value row that failed is a failure like any other, and it
        // needs the same `<file>:<line>` prefix: an unadorned parser error
        // says nothing about which fixture row provoked it.
        let got = (self.parse)(input, row)
            .map_err(|failure| Error(format!("{at}: parse({input:?}) failed: {failure}")))?;

        let equal = match &self.normalize {
            Some(normalize) => equal_value_with(&got, &want, normalize.as_ref()),
            None => equal_value(&got, &want),
        };
        if !equal {
            return Err(Error(format!(
                "{at}: parse({input:?})\n  got:      {}\n  expected: {}",
                format_value(&got),
                format_value(&want)
            )));
        }

        Ok(())
    }

    /// Run every row of an already-loaded fixture. `Err` when the fixture
    /// cannot be run at all; otherwise the failures, one per failing row,
    /// empty when every row passed.
    pub fn run_spec(&self, spec: &SpecFile) -> Result<Vec<String>> {
        self.check_spec(spec)?;
        let probe = &spec.rows[0];
        let input_col = probe.resolve(&self.input)?;
        let expected_col = probe.resolve(&self.expected)?;

        Ok(spec
            .rows
            .iter()
            .filter_map(|row| {
                let input = row.unesc(input_col);
                let expected = row.col(expected_col);
                self.check_row(row, &input, expected)
                    .err()
                    .map(|error| error.0)
            })
            .collect())
    }

    /// Load one fixture file by path and run it; see [`Runner::run_spec`].
    pub fn run_file(&self, path: impl AsRef<Path>) -> Result<Vec<String>> {
        self.run_spec(&load_spec(path, &self.load)?)
    }

    /// Load and run every `*.tsv` in a directory; see [`Runner::run_spec`].
    /// An empty directory is an error: a runner that finds nothing to run
    /// must not report green.
    pub fn run_dir(&self, dir: impl AsRef<Path>) -> Result<Vec<String>> {
        let mut failures = Vec::new();
        for spec in load_spec_dir(dir, &self.load)? {
            failures.extend(self.run_spec(&spec)?);
        }
        Ok(failures)
    }

    /// Run an already-loaded fixture, panicking with every failing row at
    /// once when any fails or the fixture cannot be run.
    pub fn spec(&self, spec: &SpecFile) {
        report(self.run_spec(spec));
    }

    /// Load one fixture file by path and run it, panicking on failure.
    pub fn file(&self, path: impl AsRef<Path>) {
        report(self.run_file(path));
    }

    /// Load and run every `*.tsv` in a directory, panicking on failure.
    pub fn dir(&self, dir: impl AsRef<Path>) {
        report(self.run_dir(dir));
    }
}

/// Panic with every failure at once, naming file and line, rather than
/// stopping at the first; do nothing when there is nothing to report.
pub fn report(outcome: Result<Vec<String>>) {
    match outcome {
        Err(error) => panic!("{error}"),
        Ok(failures) if failures.is_empty() => {}
        Ok(failures) => panic!(
            "{} fixture row(s) failed:\n{}",
            failures.len(),
            failures.join("\n")
        ),
    }
}
