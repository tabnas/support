// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// spec.go — the TSV spec-fixture loader.
//
// Every tabnas package keeps its cross-runtime conformance fixtures in
// test/spec/*.tsv at the repo root, above both runtimes, so ts/ and go/
// run the same files. Before this package each repo carried its own
// loader, and they had quietly drifted: one decoded \t and another did
// not, one skipped # comment lines and another crashed on them, one
// decoded escapes in every column and another only in the first. A row
// that means two different things in two runtimes cannot pin agreement on
// anything else, so there is one loader now, and ts/src/spec.ts mirrors it.
//
// The rules, in full:
//
//   - Line 1 is a header naming the columns. Names are how a row is read
//     by row.Named("input") rather than by position.
//   - A blank line is skipped.
//   - A line that starts with # and holds no tab is a comment and is
//     skipped. A #-leading line WITH a tab is data, so a fixture whose
//     input is a C preprocessor directive still works.
//   - Columns are returned RAW. Escape decoding is per-column and explicit
//     (row.Unesc(i)), because the expected column is normally JSON, which
//     carries its own escapes and must not be decoded twice.
//   - Line numbers are the physical 1-based line in the file, so a failure
//     message points an editor at the offending row.

// Options says how to interpret a fixture file. The zero value is the
// tabnas convention; override a field only for a fixture that genuinely
// differs. The pointer fields distinguish "false" from "not set", the same
// way the parser's option structs do.
type Options struct {
	Header  *bool // First line names the columns. Default true.
	Comment *bool // Skip #-leading lines that hold no tab. Default true.
	MinCols int   // Reject a data row with fewer columns. Default 1.
}

func (o *Options) header() bool {
	if nil == o || nil == o.Header {
		return true
	}
	return *o.Header
}

func (o *Options) comment() bool {
	if nil == o || nil == o.Comment {
		return true
	}
	return *o.Comment
}

func (o *Options) minCols() int {
	if nil == o || 0 == o.MinCols {
		return 1
	}
	return o.MinCols
}

// Row is one data row of a fixture file.
type Row struct {
	// File is the base name of the file this row came from ("happy.tsv").
	File string

	// Line is the physical 1-based line number within that file.
	Line int

	// Index is the 0-based position among the file's DATA rows (the header
	// and any skipped line do not advance it).
	Index int

	// Cols holds the row's columns, exactly as they appear in the file.
	Cols []string

	// Header holds the file's column names, shared with every row of it.
	Header []string
}

// Col returns the raw column at position i. Out of range is "" — a fixture
// with a trailing optional column should not need a length check at every
// use.
func (r *Row) Col(i int) string {
	if 0 <= i && i < len(r.Cols) {
		return r.Cols[i]
	}
	return ""
}

// Unesc returns the escape-decoded column at position i. This is what a
// parser input column goes through.
func (r *Row) Unesc(i int) string { return Unescape(r.Col(i)) }

// IndexOf returns the position of a header name, or -1 when the file has
// no such column.
func (r *Row) IndexOf(name string) int {
	for i, h := range r.Header {
		if h == name {
			return i
		}
	}
	return -1
}

// Named returns the raw column with the given header name; "" when there
// is no such column.
func (r *Row) Named(name string) string { return r.Col(r.IndexOf(name)) }

// UnescNamed returns the escape-decoded column with the given header name.
func (r *Row) UnescNamed(name string) string { return Unescape(r.Named(name)) }

// Where returns "<file>:<line>" — the prefix every failure message about
// this row should carry.
func (r *Row) Where() string {
	return r.File + ":" + strconv.Itoa(r.Line)
}

// File is a loaded fixture file.
type File struct {
	Name   string   // Base name ("happy.tsv").
	Path   string   // Path it was read from ("" when parsed from text).
	Header []string // Column names, nil when Options.Header is false.
	Rows   []*Row   // Data rows, in file order.
}

// ParseSpec parses fixture text that is already in memory. LoadSpec is
// this plus a file read; tests of the loader itself use this form.
func ParseSpec(name, text string, opts *Options) (*File, error) {
	// A BOM ahead of the header would become part of the first column
	// name, and the lookup by that name would then fail in a way nothing
	// about the fixture explains.
	text = strings.TrimPrefix(text, "\ufeff")

	base := filepath.Base(name)
	spec := &File{Name: base}

	for i, line := range strings.Split(text, "\n") {
		// Drop the CR of a CRLF line: the TS loader splits on /\r?\n/ and
		// discards it, so keeping it would feed the runtimes different bytes.
		line = strings.TrimSuffix(line, "\r")
		lineNo := i + 1

		if opts.header() && 0 == i {
			spec.Header = strings.Split(line, "\t")
			continue
		}

		if "" == line {
			continue
		}

		// A comment needs no tab; a data row always has at least one, so a
		// #-leading source stays usable as input.
		if opts.comment() &&
			strings.HasPrefix(line, "#") && !strings.Contains(line, "\t") {
			continue
		}

		cols := strings.Split(line, "\t")
		if len(cols) < opts.minCols() {
			return nil, fmt.Errorf(
				"%s:%d: expected at least %d tab-separated column(s), found %d",
				base, lineNo, opts.minCols(), len(cols))
		}

		spec.Rows = append(spec.Rows, &Row{
			File:   base,
			Line:   lineNo,
			Index:  len(spec.Rows),
			Cols:   cols,
			Header: spec.Header,
		})
	}

	return spec, nil
}

// LoadSpec loads one fixture file by path.
func LoadSpec(path string, opts *Options) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("spec file not found: %s: %w", path, err)
	}

	spec, err := ParseSpec(filepath.Base(path), string(data), opts)
	if err != nil {
		return nil, err
	}
	spec.Path = path

	return spec, nil
}

// LoadSpecDir loads every *.tsv in a directory, sorted by name so both
// runtimes and successive runs visit them in the same order. Discovery by
// listing is deliberate: adding a fixture then runs it without editing a
// runner.
func LoadSpecDir(dir string, opts *Options) ([]*File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("spec directory not found: %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tsv") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	specs := make([]*File, 0, len(names))
	for _, name := range names {
		spec, err := LoadSpec(filepath.Join(dir, name), opts)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// FindSpecDir walks up from `from` looking for a test/spec directory, and
// returns it. An empty `from` starts at the working directory, which under
// `go test` is the package directory — so a suite in go/ or go/adder/
// finds the repo's fixtures without spelling out how many levels up they
// are.
//
// This replaces the filepath.Join("..", "test", "spec") that every repo
// hard-codes — a relative hop that has to be recounted whenever a test
// moves a directory, and that is spelt differently in ts/ anyway.
func FindSpecDir(from string) (string, error) {
	start := from
	if "" == start {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = wd
	}

	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, "test", "spec")
		if info, err := os.Stat(candidate); nil == err && info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Filesystem root: there is nowhere left to look.
		}
		dir = parent
	}

	return "", fmt.Errorf(
		"no %s directory found at or above: %s",
		filepath.Join("test", "spec"), start)
}
