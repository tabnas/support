// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// spec_test.go — the fixture loader.
//
// ts/test/spec.test.js asserts the same row count and the same line
// numbers against the same file. Those two numbers are the whole
// agreement: if the runtimes disagree about what a row IS, nothing a row
// says can be trusted.

func loaderRows(t *testing.T) *File {
	t.Helper()
	return mustLoad(t, filepath.Join(specDir(t), "util", "loader-rows.tsv"))
}

func rowAtLine(t *testing.T, spec *File, line int) *Row {
	t.Helper()
	for _, row := range spec.Rows {
		if row.Line == line {
			return row
		}
	}
	t.Fatalf("%s: no row at line %d", spec.Name, line)
	return nil
}

func TestLoadSkipsHeaderBlanksAndComments(t *testing.T) {
	spec := loaderRows(t)

	if want := []string{"input", "expected"}; !reflect.DeepEqual(spec.Header, want) {
		t.Errorf("header: got %v, want %v", spec.Header, want)
	}
	if "loader-rows.tsv" != spec.Name {
		t.Errorf("name: got %q", spec.Name)
	}

	// The file's own layout, asserted by line number: header on 1,
	// comments on 2-4, then a row, a blank, and the rest.
	var lines, indexes []int
	for _, row := range spec.Rows {
		lines = append(lines, row.Line)
		indexes = append(indexes, row.Index)
	}
	if want := []int{5, 7, 8, 9, 10}; !reflect.DeepEqual(lines, want) {
		t.Errorf("lines: got %v, want %v", lines, want)
	}
	if want := []int{0, 1, 2, 3, 4}; !reflect.DeepEqual(indexes, want) {
		t.Errorf("indexes: got %v, want %v", indexes, want)
	}
}

func TestLoadHashLineWithTabIsData(t *testing.T) {
	// Otherwise a fixture whose input is a C preprocessor directive, or a
	// comment in the parsed language, could not be written at all.
	row := rowAtLine(t, loaderRows(t), 7)
	if "#hash" != row.Col(0) || "2" != row.Col(1) {
		t.Errorf("got %v", row.Cols)
	}
}

func TestLoadKeepsEmptyLeadingColumn(t *testing.T) {
	row := rowAtLine(t, loaderRows(t), 8)
	if "" != row.Col(0) || "3" != row.Col(1) {
		t.Errorf("got %v", row.Cols)
	}
}

func TestLoadReturnsRawColumns(t *testing.T) {
	row := rowAtLine(t, loaderRows(t), 9)

	// Raw: the two characters backslash and t.
	if want := `b\tc`; row.Col(0) != want {
		t.Errorf("Col(0) = %q, want %q", row.Col(0), want)
	}
	// Decoded: a real tab.
	if want := "b\tc"; row.Unesc(0) != want {
		t.Errorf("Unesc(0) = %q, want %q", row.Unesc(0), want)
	}
}

func TestLoadAllowsExtraColumns(t *testing.T) {
	row := rowAtLine(t, loaderRows(t), 10)
	if 3 != len(row.Cols) {
		t.Fatalf("cols: got %v", row.Cols)
	}
	if "6" != row.Col(2) {
		t.Errorf("Col(2) = %q", row.Col(2))
	}
}

func TestLoadColumnOutOfRangeIsEmpty(t *testing.T) {
	row := loaderRows(t).Rows[0]

	for _, got := range []string{
		row.Col(9), row.Col(-1), row.Unesc(9), row.Named("nosuch"),
	} {
		if "" != got {
			t.Errorf("out of range: got %q, want empty", got)
		}
	}
	if -1 != row.IndexOf("nosuch") {
		t.Errorf("IndexOf: got %d", row.IndexOf("nosuch"))
	}
}

func TestLoadColumnByName(t *testing.T) {
	row := loaderRows(t).Rows[0]

	if "a" != row.Named("input") || "1" != row.Named("expected") {
		t.Errorf("named: got %v", row.Cols)
	}
	if "a" != row.UnescNamed("input") {
		t.Errorf("UnescNamed: got %q", row.UnescNamed("input"))
	}
	if 1 != row.IndexOf("expected") {
		t.Errorf("IndexOf: got %d", row.IndexOf("expected"))
	}
}

func TestLoadWhere(t *testing.T) {
	if want := "loader-rows.tsv:5"; loaderRows(t).Rows[0].Where() != want {
		t.Errorf("Where() = %q, want %q", loaderRows(t).Rows[0].Where(), want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := LoadSpec(filepath.Join(specDir(t), "util", "does-not-exist.tsv"), nil)
	if nil == err || !strings.Contains(err.Error(), "spec file not found") {
		t.Errorf("err = %v", err)
	}
}

func TestParseSpecInline(t *testing.T) {
	spec := mustParse(t, "inline.tsv", "a\tb\n1\t2\n", nil)

	if want := []string{"a", "b"}; !reflect.DeepEqual(spec.Header, want) {
		t.Errorf("header: got %v", spec.Header)
	}
	if 1 != len(spec.Rows) {
		t.Fatalf("rows: got %d", len(spec.Rows))
	}
	if want := []string{"1", "2"}; !reflect.DeepEqual(spec.Rows[0].Cols, want) {
		t.Errorf("cols: got %v", spec.Rows[0].Cols)
	}
	if "" != spec.Path {
		t.Errorf("path: got %q", spec.Path)
	}
}

func TestParseSpecCRLF(t *testing.T) {
	// A fixture checked out on Windows must mean what it means anywhere
	// else — the CR is a line ending, not part of the last column.
	lf := mustParse(t, "x.tsv", "a\tb\n1\t2\n3\t4\n", nil)
	crlf := mustParse(t, "x.tsv", "a\tb\r\n1\t2\r\n3\t4\r\n", nil)

	if !reflect.DeepEqual(lf.Header, crlf.Header) {
		t.Errorf("header: %v vs %v", lf.Header, crlf.Header)
	}
	if len(lf.Rows) != len(crlf.Rows) {
		t.Fatalf("rows: %d vs %d", len(lf.Rows), len(crlf.Rows))
	}
	for i := range lf.Rows {
		if !reflect.DeepEqual(lf.Rows[i].Cols, crlf.Rows[i].Cols) {
			t.Errorf("row %d: %v vs %v", i, lf.Rows[i].Cols, crlf.Rows[i].Cols)
		}
	}
}

func TestParseSpecStripsBOM(t *testing.T) {
	spec := mustParse(t, "x.tsv", "\ufeffinput\texpected\na\t1\n", nil)

	if want := []string{"input", "expected"}; !reflect.DeepEqual(spec.Header, want) {
		t.Errorf("header: got %q", spec.Header)
	}
	if "a" != spec.Rows[0].Named("input") {
		t.Errorf("named: got %q", spec.Rows[0].Named("input"))
	}
}

func TestParseSpecNoTrailingNewline(t *testing.T) {
	if spec := mustParse(t, "x.tsv", "a\tb\n1\t2", nil); 1 != len(spec.Rows) {
		t.Errorf("rows: got %d", len(spec.Rows))
	}
}

func TestParseSpecHeaderOnly(t *testing.T) {
	spec := mustParse(t, "x.tsv", "a\tb\n", nil)

	if 0 != len(spec.Rows) {
		t.Errorf("rows: got %d", len(spec.Rows))
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(spec.Header, want) {
		t.Errorf("header: got %v", spec.Header)
	}
}

func TestParseSpecNoHeader(t *testing.T) {
	spec := mustParse(t, "x.tsv", "1\t2\n3\t4\n", &Options{Header: Bool(false)})

	if 0 != len(spec.Header) {
		t.Errorf("header: got %v", spec.Header)
	}
	if 2 != len(spec.Rows) {
		t.Fatalf("rows: got %d", len(spec.Rows))
	}
	if want := []string{"1", "2"}; !reflect.DeepEqual(spec.Rows[0].Cols, want) {
		t.Errorf("cols: got %v", spec.Rows[0].Cols)
	}
	if 1 != spec.Rows[0].Line {
		t.Errorf("line: got %d", spec.Rows[0].Line)
	}
}

func TestParseSpecKeepComments(t *testing.T) {
	spec := mustParse(t, "x.tsv", "a\n# kept\n", &Options{Comment: Bool(false)})

	if 1 != len(spec.Rows) {
		t.Fatalf("rows: got %d", len(spec.Rows))
	}
	if "# kept" != spec.Rows[0].Col(0) {
		t.Errorf("col: got %q", spec.Rows[0].Col(0))
	}
}

func TestParseSpecMinCols(t *testing.T) {
	_, err := ParseSpec("x.tsv", "a\tb\nonly-one\n", &Options{MinCols: 2})
	if nil == err || !strings.Contains(err.Error(), "x.tsv:2: expected at least 2") {
		t.Errorf("err = %v", err)
	}

	// One column is fine by default.
	spec := mustParse(t, "x.tsv", "a\nsolo\n", nil)
	if want := []string{"solo"}; !reflect.DeepEqual(spec.Rows[0].Cols, want) {
		t.Errorf("cols: got %v", spec.Rows[0].Cols)
	}
}

func TestLoadSpecDir(t *testing.T) {
	specs, err := LoadSpecDir(filepath.Join(specDir(t), "adder"), nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var names []string
	for _, spec := range specs {
		names = append(names, spec.Name)
		if 0 == len(spec.Rows) {
			t.Errorf("%s: no rows", spec.Name)
		}
		if "" == spec.Path {
			t.Errorf("%s: no path", spec.Name)
		}
	}
	if want := []string{"basic.tsv", "errors.tsv"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names: got %v, want %v", names, want)
	}
}

func TestLoadSpecDirIgnoresNonTSV(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"b.tsv": "a\n1\n", "a.tsv": "a\n1\n",
		"notes.md": "not a fixture", "x.tsv.bak": "a\n1\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	specs, err := LoadSpecDir(dir, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var names []string
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	if want := []string{"a.tsv", "b.tsv"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names: got %v, want %v", names, want)
	}
}

func TestLoadSpecDirMissing(t *testing.T) {
	_, err := LoadSpecDir(filepath.Join(specDir(t), "nosuchdir"), nil)
	if nil == err || !strings.Contains(err.Error(), "spec directory not found") {
		t.Errorf("err = %v", err)
	}
}

func TestFindSpecDir(t *testing.T) {
	want := specDir(t)

	// From the package directory, from its parent, and from inside the
	// fixture tree itself.
	for _, from := range []string{".", "..", filepath.Join(want, "adder")} {
		got, err := FindSpecDir(from)
		if err != nil {
			t.Fatalf("FindSpecDir(%q): %v", from, err)
		}
		if got != want {
			t.Errorf("FindSpecDir(%q) = %q, want %q", from, got, want)
		}
	}
}

func TestFindSpecDirNotFound(t *testing.T) {
	_, err := FindSpecDir(t.TempDir())
	if nil == err || !strings.Contains(err.Error(), "no test") {
		t.Errorf("err = %v", err)
	}
}
