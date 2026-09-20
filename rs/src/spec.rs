// Copyright (c) 2026 tabnas, MIT License

// The TSV spec-fixture loader.
//
// Every tabnas package keeps its cross-runtime conformance fixtures in
// `test/spec/*.tsv` at the repo root, above every runtime, so `ts/`,
// `go/` and `rs/` run the same files. Before this package each repo
// carried its own loader, and they had quietly drifted: one decoded `\t`
// and another did not, one skipped `#` comment lines and another crashed
// on them, one decoded escapes in every column and another only in the
// first. A row that means two different things in two runtimes cannot pin
// agreement on anything else, so there is one loader now, and this file
// mirrors `ts/src/spec.ts` and `go/spec.go`.
//
// The rules, in full:
//
// - Line 1 is a header naming the columns. Names are how a row is read by
//   `row.named("input")` rather than by position.
// - A blank line is skipped.
// - A line that starts with `#` and holds no tab is a comment and is
//   skipped. A `#`-leading line WITH a tab is data, so a fixture whose
//   input is a C preprocessor directive still works.
// - Columns are returned RAW. Escape decoding is per-column and explicit
//   (`row.unesc(i)`), because the `expected` column is normally JSON,
//   which carries its own escapes and must not be decoded twice.
// - Line numbers are the physical 1-based line in the file, so a failure
//   message points an editor at the offending row.

use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Arc;

use crate::escape::unescape;
use crate::{Error, Result};

/// How to interpret a fixture file. The defaults are the tabnas
/// convention; override them only for a fixture that genuinely differs.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SpecOptions {
    /// First line names the columns. Default true.
    pub header: bool,
    /// Skip `#`-leading lines that hold no tab. Default true.
    pub comment: bool,
    /// Reject a data row with fewer columns. Default 1.
    pub min_cols: usize,
}

impl Default for SpecOptions {
    fn default() -> Self {
        SpecOptions {
            header: true,
            comment: true,
            min_cols: 1,
        }
    }
}

/// A column selector: a position, or a header name.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Column {
    Position(usize),
    Name(String),
}

impl From<usize> for Column {
    fn from(position: usize) -> Self {
        Column::Position(position)
    }
}

impl From<&str> for Column {
    fn from(name: &str) -> Self {
        Column::Name(name.to_string())
    }
}

impl From<String> for Column {
    fn from(name: String) -> Self {
        Column::Name(name)
    }
}

/// One data row of a fixture file.
#[derive(Debug, Clone)]
pub struct Row {
    /// Base name of the file this row came from (`happy.tsv`).
    pub file: String,
    /// Physical 1-based line number within that file.
    pub line: usize,
    /// 0-based position among the file's DATA rows (header and skipped
    /// lines do not advance it).
    pub index: usize,
    /// The row's columns, exactly as they appear in the file.
    pub cols: Vec<String>,
    /// The file's header names, shared with every row of the file.
    pub header: Arc<Vec<String>>,
}

impl Row {
    /// Raw column by position. Out of range is `""`: a fixture with a
    /// trailing optional column should not need a length check at every
    /// use.
    pub fn col(&self, i: usize) -> &str {
        self.cols.get(i).map(String::as_str).unwrap_or("")
    }

    /// Escape-decoded column by position. This is what a parser input
    /// column goes through.
    pub fn unesc(&self, i: usize) -> String {
        unescape(self.col(i))
    }

    /// Position of a header name, or `None` when the file has no such
    /// column.
    pub fn index_of(&self, name: &str) -> Option<usize> {
        self.header.iter().position(|column| column == name)
    }

    /// Raw column by header name; `""` when there is no such column.
    pub fn named(&self, name: &str) -> &str {
        self.index_of(name).map_or("", |i| self.col(i))
    }

    /// Escape-decoded column by header name.
    pub fn unesc_named(&self, name: &str) -> String {
        unescape(self.named(name))
    }

    /// Resolve a column selector to a position. An unknown name is a
    /// defect in the caller, not a missing value, so it is an error rather
    /// than a silent read of an absent column.
    pub fn resolve(&self, selector: &Column) -> Result<usize> {
        match selector {
            Column::Position(i) => Ok(*i),
            Column::Name(name) => self.index_of(name).ok_or_else(|| {
                Error(format!(
                    "{}: no column named '{}' (header: {})",
                    self.file,
                    name,
                    self.header.join(", ")
                ))
            }),
        }
    }

    /// `<file>:<line>`, the prefix every failure message should carry.
    pub fn location(&self) -> String {
        format!("{}:{}", self.file, self.line)
    }
}

/// A loaded fixture file.
#[derive(Debug, Clone)]
pub struct SpecFile {
    /// Base name (`happy.tsv`).
    pub file: String,
    /// Path it was read from (empty when parsed from text).
    pub path: PathBuf,
    /// Column names, empty when `header` is off.
    pub header: Arc<Vec<String>>,
    /// Data rows, in file order.
    pub rows: Vec<Row>,
}

/// Parse fixture text that is already in memory. [`load_spec`] is this
/// plus a file read; tests of the loader itself use this form.
pub fn parse_spec(file: &str, text: &str, options: &SpecOptions) -> Result<SpecFile> {
    // A BOM ahead of the header would become part of the first column
    // name, and the lookup by that name would then fail in a way nothing
    // about the fixture explains.
    let text = text.strip_prefix('\u{FEFF}').unwrap_or(text);

    let name = base_name(file);
    let mut header: Arc<Vec<String>> = Arc::new(Vec::new());
    let mut rows = Vec::new();

    for (i, raw) in text.split('\n').enumerate() {
        // Drop the CR of a CRLF line, so a CRLF fixture feeds every
        // runtime the same bytes.
        let line = raw.strip_suffix('\r').unwrap_or(raw);
        let line_no = i + 1;

        if options.header && i == 0 {
            header = Arc::new(line.split('\t').map(str::to_string).collect());
            continue;
        }

        if line.is_empty() {
            continue;
        }

        // A comment needs no tab; a data row always has at least one, so
        // a `#`-leading source stays usable as input.
        if options.comment && line.starts_with('#') && !line.contains('\t') {
            continue;
        }

        let cols: Vec<String> = line.split('\t').map(str::to_string).collect();
        if cols.len() < options.min_cols {
            return Err(Error(format!(
                "{name}:{line_no}: expected at least {} tab-separated column(s), found {}",
                options.min_cols,
                cols.len()
            )));
        }

        rows.push(Row {
            file: name.clone(),
            line: line_no,
            index: rows.len(),
            cols,
            header: Arc::clone(&header),
        });
    }

    Ok(SpecFile {
        file: name,
        path: PathBuf::new(),
        header,
        rows,
    })
}

/// Load one fixture file by path.
pub fn load_spec(path: impl AsRef<Path>, options: &SpecOptions) -> Result<SpecFile> {
    let path = path.as_ref();
    let text = fs::read_to_string(path)
        .map_err(|error| Error(format!("spec file not found: {}: {error}", path.display())))?;
    let mut spec = parse_spec(&path.to_string_lossy(), &text, options)?;
    spec.path = path.to_path_buf();
    Ok(spec)
}

/// Load every `*.tsv` in a directory, sorted by name so every runtime and
/// every run visits them in the same order. Discovery by listing is
/// deliberate: adding a fixture then runs it without editing a runner.
pub fn load_spec_dir(dir: impl AsRef<Path>, options: &SpecOptions) -> Result<Vec<SpecFile>> {
    let dir = dir.as_ref();
    if !dir.is_dir() {
        return Err(Error(format!(
            "spec directory not found: {}",
            dir.display()
        )));
    }

    let entries = fs::read_dir(dir).map_err(|error| {
        Error(format!(
            "spec directory not found: {}: {error}",
            dir.display()
        ))
    })?;

    let mut names = Vec::new();
    for entry in entries {
        let entry = entry.map_err(|error| Error(format!("{}: {error}", dir.display())))?;
        let name = entry.file_name().to_string_lossy().into_owned();
        if !name.ends_with(".tsv") {
            continue;
        }
        // Files only, judged by what the name RESOLVES to: a DIRECTORY
        // named `foo.tsv` would otherwise be handed to the reader and
        // abort the run, and a symlinked fixture must be loaded rather
        // than dropped, since a row that runs in one runtime and not
        // another is the exact failure this crate exists to prevent. A
        // dangling link names a fixture that is not there, and skipping
        // it silently is the same silent pass this loader refuses
        // everywhere else, so it is an error.
        let metadata = fs::metadata(dir.join(&name)).map_err(|error| {
            Error(format!(
                "spec fixture cannot be read: {}: {error}",
                dir.join(&name).display()
            ))
        })?;
        if metadata.is_file() {
            names.push(name);
        }
    }
    names.sort();

    // A directory with no fixtures in it is the silent-pass failure mode
    // one level up: every runner over it reports green having run nothing.
    if names.is_empty() {
        return Err(Error(format!(
            "no .tsv fixtures in spec directory: {}",
            dir.display()
        )));
    }

    names
        .iter()
        .map(|name| load_spec(dir.join(name), options))
        .collect()
}

/// Walk up from `from` (the working directory when `None`) looking for a
/// `test/spec` directory, and return it.
///
/// This replaces the `../../test/spec` hop every suite would otherwise
/// hard-code, which has to be recounted whenever a test moves a
/// directory. A crate's suite passes `Some(env!("CARGO_MANIFEST_DIR"))`.
pub fn find_spec_dir(from: Option<&Path>) -> Result<PathBuf> {
    let start = match from {
        Some(path) => path.to_path_buf(),
        None => std::env::current_dir()
            .map_err(|error| Error(format!("cannot read the working directory: {error}")))?,
    };
    let start = start.canonicalize().unwrap_or(start);

    let mut dir = start.clone();
    loop {
        let candidate = dir.join("test").join("spec");
        if candidate.is_dir() {
            return Ok(candidate);
        }
        match dir.parent() {
            Some(parent) => dir = parent.to_path_buf(),
            None => break, // Filesystem root: there is nowhere left to look.
        }
    }

    Err(Error(format!(
        "no test/spec directory found at or above: {}",
        start.display()
    )))
}

fn base_name(file: &str) -> String {
    Path::new(file)
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_else(|| file.to_string())
}
