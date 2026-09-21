// Copyright (c) 2026 tabnas, MIT License

// A divergence register: the places two ports of one grammar DISAGREE,
// recorded in a fixture every port executes.
//
// The audit this exists for found 29 recorded divergence claims
// contradicted by execution, and one file that had been wrong in BOTH
// directions at once. Prose does not hold. What did hold was the one
// mechanism that ran: a register whose rows are executed, so a row that
// stops being true fails the build.
//
// The property that makes it work is not that a regression fails. It is
// that a FIX fails too. When a port is repaired to agree with the other,
// the register still claims they differ, so the suite goes red and names
// the row to delete. A register cannot quietly outlive the divergence it
// records, which is precisely how the 29 prose claims survived.
//
// `ts/src/register.ts` and `go/register.go` mirror all of this.

use std::path::Path;

use crate::expect::{equal_value, equal_value_with, error_code, is_error_expect, parse_expect};
use crate::runner::{report, Runner};
use crate::spec::{load_spec, Column, Row, SpecFile};
use crate::{Error, Result};

/// A fixture of recorded divergences, run by every port.
///
/// Each row gives an input and one cell per runtime, written in the same
/// vocabulary as an ordinary fixture's expected column: a JSON value, or
/// `ERROR:<code>`. Every row is checked three ways:
///
/// 1. The row must actually record a divergence. If every runtime cell
///    says the same thing, the row asserts nothing and would pass
///    forever; that is the shape of the claims this mechanism replaces.
/// 2. This runtime must still produce what the register says it does.
/// 3. When it does not, and it now produces what ANOTHER runtime's cell
///    says, the failure says the divergence is CLOSED and names the row
///    to delete, rather than reporting it as a regression, which is the
///    opposite conclusion.
#[derive(Clone)]
pub struct Register {
    /// Supplies the parse hook and every comparison rule. The register
    /// adds which column to read and what a mismatch means; it does not
    /// get its own idea of what "equal" is.
    runner: Runner,
    /// The column holding THIS runtime's answer: `rust` in a Rust suite.
    /// The suites of the ports run the same file and read different
    /// columns of it.
    runtime: String,
    /// Every runtime column in the file, this one included. Named rather
    /// than inferred from the header, because inferring would silently
    /// treat a `note` or `issue` column as a runtime and then report that
    /// the ports "agree" with a sentence.
    runtimes: Vec<String>,
}

impl Register {
    /// A register reading the `runtime` column of a file whose runtime
    /// columns are `runtimes`.
    pub fn new(runner: Runner, runtime: &str, runtimes: &[&str]) -> Self {
        Register {
            runner,
            runtime: runtime.to_string(),
            runtimes: runtimes.iter().map(|name| name.to_string()).collect(),
        }
    }

    /// Why this register cannot run at all, or `Ok`.
    fn check(&self) -> Result<()> {
        if self.runtime.is_empty() {
            return Err(Error("Register: runtime is required".to_string()));
        }
        if self.runtimes.len() < 2 {
            return Err(Error(format!(
                "Register: a divergence needs at least two runtimes, got {:?}",
                self.runtimes
            )));
        }
        if !self.runtimes.contains(&self.runtime) {
            return Err(Error(format!(
                "Register: runtimes must include {:?}, got {:?}",
                self.runtime, self.runtimes
            )));
        }
        Ok(())
    }

    /// A runtime column the file does not have. A typo in `runtimes`
    /// would otherwise surface as a missing-column error on some later
    /// row, or not at all.
    fn check_columns(&self, row: &Row) -> Result<()> {
        for name in &self.runtimes {
            if row.index_of(name).is_none() {
                return Err(Error(format!(
                    "no column named {name:?} (Register runtimes)"
                )));
            }
        }
        Ok(())
    }

    /// Do two expectation CELLS mean the same thing?
    ///
    /// Compared by meaning, not by bytes. `1` and `1.0`, or two objects
    /// written with their keys in a different order, are the same
    /// expectation to the runner, so a row whose cells differ only that
    /// way records no divergence, and comparing the raw strings would let
    /// it sit there passing in every port forever while describing a
    /// disagreement that does not exist.
    fn same_expectation(&self, a: &str, b: &str, row: &Row) -> bool {
        if a == b {
            return true;
        }

        if is_error_expect(a) || is_error_expect(b) {
            if !is_error_expect(a) || !is_error_expect(b) {
                return false;
            }
            return match (error_code(a), error_code(b)) {
                (Ok(code_a), Ok(code_b)) => code_a == code_b,
                _ => false,
            };
        }

        let read = |cell: &str| -> Result<_> {
            match self.runner.parse_expected_hook() {
                Some(hook) => hook(cell, row),
                None => parse_expect(cell),
            }
        };

        // A cell the fixture's own reader cannot parse is a defect the
        // runner will report with a better message when it runs the row.
        // Saying "these differ" here defers to that.
        let (Ok(value_a), Ok(value_b)) = (read(a), read(b)) else {
            return false;
        };

        match self.runner.normalize_hook() {
            Some(normalize) => equal_value_with(&value_a, &value_b, normalize),
            None => equal_value(&value_a, &value_b),
        }
    }

    /// Run one row and return the failure rather than reporting it.
    pub fn check_row(&self, row: &Row, input: &str) -> Result<()> {
        self.check()?;
        // Checked here too, not only in `run_spec`: this is public, and a
        // caller driving it directly would otherwise read a missing
        // column as an empty expectation.
        self.check_columns(row)
            .map_err(|error| Error(format!("{}: {error}", row.location())))?;

        let mine = row.named(&self.runtime).to_string();
        let others: Vec<(String, String)> = self
            .runtimes
            .iter()
            .filter(|name| **name != self.runtime)
            .map(|name| (name.clone(), row.named(name).to_string()))
            .collect();

        // 1. Does this row record a divergence at all?
        if others
            .iter()
            .all(|(_, cell)| self.same_expectation(cell, &mine, row))
        {
            return Err(Error(format!(
                "{}: every runtime column means {mine:?}, so this row records no divergence \
                 and can never fail meaningfully. Delete it, or correct the cells to what \
                 the runtimes actually do.",
                row.location()
            )));
        }

        // ONE parse per row, whatever it ends up being compared against.
        let runner = self
            .runner
            .reading(Column::Name(self.runtime.clone()))
            .parsing_once();

        // 2. Does this runtime still do what the register says?
        let Err(mismatch) = runner.check_row(row, input, &mine) else {
            return Ok(());
        };

        // 3. It does not. Which of the OTHERS does it now agree with?
        let converged: Vec<&(String, String)> = others
            .iter()
            .filter(|(_, cell)| runner.check_row(row, input, cell).is_ok())
            .collect();

        // 4. None. An ordinary regression; the runner's own message says
        //    what was produced and what was expected.
        if converged.is_empty() {
            return Err(mismatch);
        }

        let names: Vec<&str> = converged.iter().map(|(name, _)| name.as_str()).collect();

        // Converged with EVERY other runtime: the divergence is gone.
        if converged.len() == others.len() {
            return Err(Error(format!(
                "{}: this divergence is CLOSED. {} now produces what the {} column(s) record \
                 ({:?}), not its own ({mine:?}).\n  \
                 This is the register working: a fixed divergence fails as loudly as a \
                 regressed one, so the row cannot outlive it.\n  \
                 DELETE this row. Do not edit it to match: that would record a divergence \
                 that no longer exists, which is what this mechanism exists to prevent.",
                row.location(),
                self.runtime,
                names.join(", "),
                converged[0].1
            )));
        }

        // Converged with SOME. The row still records a live disagreement
        // between the runtimes that have not converged, so deleting it
        // would drop that coverage. Only this runtime's own column is
        // stale.
        let live: Vec<&str> = others
            .iter()
            .filter(|other| !converged.iter().any(|c| c.0 == other.0))
            .map(|(name, _)| name.as_str())
            .collect();

        Err(Error(format!(
            "{}: this divergence is PARTIALLY closed. {} now agrees with {}, but not with {}.\n  \
             Do NOT delete this row: it still records a live disagreement between the \
             runtimes that have not converged.\n  \
             UPDATE the {} column to what it now produces, instead of {mine:?}.",
            row.location(),
            self.runtime,
            names.join(", "),
            live.join(", "),
            self.runtime
        )))
    }

    /// Run every row of an already-loaded register. `Err` when it cannot
    /// run at all; otherwise the failures, one per failing row.
    pub fn run_spec(&self, spec: &SpecFile) -> Result<Vec<String>> {
        self.check()?;
        // Checked through a runner whose expectation column IS this
        // runtime's, because that is the column a register reads.
        let reading = self.runner.reading(Column::Name(self.runtime.clone()));
        reading.check_spec(spec)?;
        self.check_columns(&spec.rows[0])
            .map_err(|error| Error(format!("{}: {error}", spec.file)))?;

        let input_col = spec.rows[0].resolve(self.runner.input_column())?;
        Ok(spec
            .rows
            .iter()
            .filter_map(|row| {
                let input = row.unesc(input_col);
                self.check_row(row, &input).err().map(|error| error.0)
            })
            .collect())
    }

    /// Load one register file by path and run it; see [`Register::run_spec`].
    pub fn run_file(&self, path: impl AsRef<Path>) -> Result<Vec<String>> {
        self.check()?;
        self.run_spec(&load_spec(path, self.runner.load_options())?)
    }

    /// Run an already-loaded register, panicking with every failing row.
    pub fn spec(&self, spec: &SpecFile) {
        report(self.run_spec(spec));
    }

    /// Load one register file by path and run it, panicking on failure.
    pub fn file(&self, path: impl AsRef<Path>) {
        report(self.run_file(path));
    }
}

/// Declare that a repo records no divergences at all.
///
/// An empty register is legitimate; an empty FILE is not, because the
/// runner cannot tell "no rows" from "the loader read nothing". Call this
/// from a test so the claim is stated in the suite rather than assumed
/// from a file nobody notices is missing. It asserts nothing; its value is
/// that the suite names the claim out loud.
pub fn no_divergences(where_: &str) {
    let _ = where_.trim();
}
