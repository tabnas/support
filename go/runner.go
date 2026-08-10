// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"fmt"
	"reflect"
	"testing"
)

// runner.go — the table-driven runner: fixture rows in, subtests out.
//
// Loading a fixture is only half of what every repo was duplicating. The
// other half is the loop — read the input column, parse it, branch on
// whether the expected column names a value or an error, compare, and
// report with enough context to find the row. That loop is the same
// everywhere, and ts/src/runner.ts runs the identical one against
// node:test.

// Runner drives fixture rows through one parser. Reuse it across files.
type Runner struct {
	// Parse parses one input. It must return an error for input the
	// grammar must not accept. Required.
	Parse func(input string) (any, error)

	// ErrorCode extracts the code from a parse error. Only needed for
	// fixtures with ERROR:<code> rows. The default reads a `Code() string`
	// or `Code string`, which is what *TabnasError carries.
	ErrorCode func(err error) string

	// Normalize rewrites values before comparison — see EqualValueWith.
	Normalize func(any) any

	// Input and Expected select columns by position; InputName and
	// ExpectedName select them by header name and win when set. The
	// defaults are column 0 and column 1.
	//
	// The positions are pointers because 0 is a real column: a plain int
	// could not tell "the expected column is column 0" from "not set",
	// and the one it would have to guess wrong is the default.
	Input        *int
	InputName    string
	Expected     *int
	ExpectedName string

	// Load holds the fixture-loading options, passed to the loader.
	Load *Options

	// CaseName names a subtest. The default is "row <line>: <input>",
	// which keeps the file's own line numbers in the test output.
	CaseName func(row *Row, input string) string
}

// Dir loads and runs every *.tsv in a directory. An empty directory fails:
// a runner that finds nothing to run must not report green.
func (r Runner) Dir(t *testing.T, dir string) {
	t.Helper()

	// LoadSpecDir rejects a directory with no fixtures in it, so a run
	// that would have been green having done nothing fails here instead.
	specs, err := LoadSpecDir(dir, r.Load)
	if err != nil {
		t.Fatalf("%v", err)
	}

	for _, spec := range specs {
		r.Spec(t, spec)
	}
}

// File loads one fixture file by path and runs it.
func (r Runner) File(t *testing.T, path string) {
	t.Helper()

	spec, err := LoadSpec(path, r.Load)
	if err != nil {
		t.Fatalf("%v", err)
	}
	r.Spec(t, spec)
}

// Spec runs every row of an already-loaded fixture, as a subtest named for
// the file.
func (r Runner) Spec(t *testing.T, spec *File) {
	t.Helper()

	t.Run("spec: "+spec.Name, func(t *testing.T) {
		if err := r.CheckSpec(spec); err != nil {
			t.Fatalf("%v", err)
		}

		inCol, _ := r.column(spec, r.Input, r.InputName, 0)
		outCol, _ := r.column(spec, r.Expected, r.ExpectedName, 1)

		for _, row := range spec.Rows {
			input := row.Unesc(inCol)
			expected := row.Col(outCol)

			name := fmt.Sprintf("row %d: %q", row.Line, input)
			if nil != r.CaseName {
				name = r.CaseName(row, input)
			}

			t.Run(name, func(t *testing.T) {
				if err := r.CheckRow(row, input, expected); err != nil {
					t.Error(err)
				}
			})
		}
	})
}

// CheckSpec reports why a fixture cannot be run, or nil when it can.
// Spec calls it and fails the test; it is exported so the guard itself
// can be asserted, which reporting through *testing.T does not allow.
//
// A fixture that loads but holds no rows is the case that matters: it is
// a silent pass, and a silent pass is indistinguishable from coverage
// that was never there.
func (r Runner) CheckSpec(spec *File) error {
	if 0 == len(spec.Rows) {
		return fmt.Errorf("%s: no cases", spec.Name)
	}

	if _, err := r.column(spec, r.Input, r.InputName, 0); err != nil {
		return fmt.Errorf("%s: input column: %w", spec.Name, err)
	}
	if _, err := r.column(spec, r.Expected, r.ExpectedName, 1); err != nil {
		return fmt.Errorf("%s: expected column: %w", spec.Name, err)
	}

	return nil
}

// Row runs one row and reports any failure on t.
func (r Runner) Row(t *testing.T, row *Row, input, expected string) {
	t.Helper()

	if err := r.CheckRow(row, input, expected); err != nil {
		t.Error(err)
	}
}

// CheckRow runs one row and returns the failure rather than reporting it,
// for a suite that does its own reporting.
func (r Runner) CheckRow(row *Row, input, expected string) error {
	// Said plainly, rather than as a nil-func panic from inside the
	// comparison, where the cause is much less obvious.
	if nil == r.Parse {
		return fmt.Errorf("%s: Runner.Parse is required", row.Where())
	}

	if IsErrorExpect(expected) {
		want, err := ErrorCode(expected)
		if err != nil {
			return fmt.Errorf("%s: %w", row.Where(), err)
		}

		got, parseErr := r.Parse(input)
		if nil == parseErr {
			return fmt.Errorf(
				"%s: Parse(%q) should fail with %s, but returned %s",
				row.Where(), input, expected, FormatValue(got))
		}

		if "" != want {
			code := errCode(parseErr)
			if nil != r.ErrorCode {
				code = r.ErrorCode(parseErr)
			}
			if code != want {
				return fmt.Errorf(
					"%s: Parse(%q) failed with code %q, expected %q\n  error: %v",
					row.Where(), input, code, want, parseErr)
			}
		}

		return nil
	}

	want, err := ParseExpect(expected)
	if err != nil {
		return fmt.Errorf("%s: %w", row.Where(), err)
	}

	got, parseErr := r.Parse(input)
	if parseErr != nil {
		return fmt.Errorf("%s: Parse(%q) failed: %v",
			row.Where(), input, parseErr)
	}

	if !EqualValueWith(got, want, r.Normalize) {
		return fmt.Errorf(
			"%s: Parse(%q)\n  got:      %s\n  expected: %s",
			row.Where(), input, FormatValue(got), FormatValue(want))
	}

	return nil
}

// column resolves a column selector — a name when given, else a position,
// else the default — against the file's header. An unknown name is a
// defect in the caller, not a missing value, so it is an error rather than
// a silent read of column -1.
func (r Runner) column(spec *File, pos *int, name string, def int) (int, error) {
	if "" == name {
		if nil != pos {
			return *pos, nil
		}
		return def, nil
	}

	for i, h := range spec.Header {
		if h == name {
			return i, nil
		}
	}

	return -1, fmt.Errorf("no column named %q (header: %v)", name, spec.Header)
}

// errCode pulls a parse error's code out without importing the parser:
// this module has no dependencies, so it asks the error for its own code
// through the two shapes tabnas errors use — a Code() method, or a Code
// field on a struct.
func errCode(err error) string {
	if nil == err {
		return ""
	}

	if c, ok := err.(interface{ Code() string }); ok {
		return c.Code()
	}

	if c, ok := err.(interface{ GetCode() string }); ok {
		return c.GetCode()
	}

	return fieldCode(err)
}

// fieldCode reads a string `Code` field off an error struct (or a pointer
// to one). *tabnas.TabnasError exposes its code that way, and a Runner
// that could not read it would need every caller to supply an ErrorCode
// hook for the single most common case.
func fieldCode(err error) string {
	v := reflect.ValueOf(err)

	if reflect.Ptr == v.Kind() {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	if reflect.Struct != v.Kind() {
		return ""
	}

	f := v.FieldByName("Code")
	if !f.IsValid() || reflect.String != f.Kind() {
		return ""
	}

	return f.String()
}
