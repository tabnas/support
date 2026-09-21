// Copyright (c) 2026 tabnas, MIT License

// Coverage and parity tripwires over error codes.
//
// Every tabnas package declares a set of error codes, renders each through
// a `{code: template}` message catalogue, and pins its rejections in
// fixtures as `ERROR:<code>` rows. Three lists, three ways to drift: a
// declared code no fixture ever exercises, two catalogues whose keys or
// templates have come apart, a fixture pinning a code nobody declares.
// These helpers compute those gaps from real loaded data.
//
// Every input arrives as an argument. Nothing here fetches a catalogue or
// imports the engine, which is exactly why these tripwires can live in the
// one crate every repo already shares.
//
// `ts/src/census.ts` and `go/census.go` mirror all of this.

use std::collections::{BTreeSet, HashMap};
use std::path::Path;

use crate::expect::{error_code, is_error_expect};
use crate::spec::{load_spec_dir, Row, SpecFile, SpecOptions};
use crate::{Error, Result};

/// Which column holds the expectation. The default is each row's last
/// column (the ordinary two-column `input`/`expected` fixture needs no
/// selection at all), but a wider fixture can keep the expectation
/// anywhere, which is why the selection exists.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CensusOptions {
    /// Expectation column by position.
    pub col: Option<usize>,
    /// Expectation column by header name; wins when set.
    pub name: Option<String>,
}

/// What a code-style expectation names: a bare lowercase token, which is
/// how every tabnas error code is spelt. `ERROR:bad token` and
/// `ERROR:1:8` are message- and position-style expectations, real
/// rejections but not codes, and a census that returned them would count
/// coverage that is not there.
fn is_code_token(code: &str) -> bool {
    let mut chars = code.chars();
    matches!(chars.next(), Some('a'..='z'))
        && chars.all(|c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == '_')
}

/// Collect every code named by a code-style expectation cell under a
/// fixture directory: sorted, unique. Message-style expectations and bare
/// `ERROR` cells assert a rejection without naming a code, so they are
/// not collected; a value row is not an expectation at all.
///
/// The directory is read with the shared loader, so an empty directory
/// fails here the way it fails everywhere else: a census over nothing
/// must not report "no codes" as if it had looked.
pub fn codes_in_spec_dir(dir: impl AsRef<Path>, options: &CensusOptions) -> Result<Vec<String>> {
    let mut codes = BTreeSet::new();

    for spec in load_spec_dir(dir, &SpecOptions::default())? {
        for row in &spec.rows {
            let cell = row.col(expectation_col(&spec, row, options)?);
            if !is_error_expect(cell) {
                continue;
            }
            let code = error_code(cell)?;
            if is_code_token(&code) {
                codes.insert(code);
            }
        }
    }

    // A `BTreeSet<String>` orders by bytes, which for valid UTF-8 is code
    // point order: the same order Go's sort.Strings and the TypeScript
    // half's code-point comparison produce.
    Ok(codes.into_iter().collect())
}

/// The expectation column for one row: the named column when `name` is
/// set, else the given position, else the row's own last column. An
/// unknown name is an error: that is a defect in the caller, and a census
/// silently reading the wrong column would report coverage nobody has.
fn expectation_col(spec: &SpecFile, row: &Row, options: &CensusOptions) -> Result<usize> {
    if let Some(name) = &options.name {
        return row.index_of(name).ok_or_else(|| {
            Error(format!(
                "{}: no column named {name:?} (header: {:?})",
                spec.file, spec.header
            ))
        });
    }
    if let Some(col) = options.col {
        return Ok(col);
    }
    Ok(row.cols.len().saturating_sub(1))
}

/// What [`compare_catalogues`] found. All three lists are sorted.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CatalogueDiff {
    /// Keys of `a` absent in `b`.
    pub missing: Vec<String>,
    /// Keys of `b` absent in `a`.
    pub extra: Vec<String>,
    /// Shared keys whose templates differ.
    pub template_mismatch: Vec<String>,
}

/// Diff two `{code: template}` catalogues: message catalogues, hint
/// catalogues, or one runtime's against the other's. Templates compare
/// byte for byte: two templates that merely "mean the same" have still
/// drifted, and the byte diff is what a maintainer has to reconcile.
pub fn compare_catalogues(
    a: &HashMap<String, String>,
    b: &HashMap<String, String>,
) -> CatalogueDiff {
    let mut diff = CatalogueDiff::default();

    for (code, template) in a {
        match b.get(code) {
            None => diff.missing.push(code.clone()),
            Some(other) if other != template => diff.template_mismatch.push(code.clone()),
            Some(_) => {}
        }
    }
    for code in b.keys() {
        if !a.contains_key(code) {
            diff.extra.push(code.clone());
        }
    }

    diff.missing.sort();
    diff.extra.sort();
    diff.template_mismatch.sort();
    diff
}

/// What [`coverage`] found. Both lists are sorted.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CoverageReport {
    /// Declared, but exercised by no fixture.
    pub uncovered: Vec<String>,
    /// Exercised, but declared by nobody.
    pub orphan: Vec<String>,
}

/// Compare the codes a package declares against the codes its fixtures
/// exercise (typically [`codes_in_spec_dir`]'s answer). Whether inherited
/// base codes count as declared is the caller's choice: pass them in or
/// leave them out.
///
/// An uncovered code is a rejection nobody has pinned; an orphan is a
/// fixture pinning a code the package does not declare, a misspelt code
/// or one that has since been removed. Both lists empty is what "the
/// fixtures and the declarations agree" means.
pub fn coverage(declared: &[String], exercised: &[String]) -> CoverageReport {
    let declared_set: BTreeSet<&String> = declared.iter().collect();
    let exercised_set: BTreeSet<&String> = exercised.iter().collect();

    CoverageReport {
        uncovered: declared_set
            .difference(&exercised_set)
            .map(|code| (*code).clone())
            .collect(),
        orphan: exercised_set
            .difference(&declared_set)
            .map(|code| (*code).clone())
            .collect(),
    }
}
