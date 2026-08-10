// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runner_test.go — the runner's own failure behaviour.
//
// The runner's passing path is exercised by every other suite here simply
// by being green. What needs its own test is the other path: that a wrong
// answer, a missing failure, a wrong error code and an empty fixture each
// FAIL. A runner that quietly passes is the one bug that hides every
// other one.
//
// These call CheckRow, which returns the failure instead of reporting it,
// so a failing case can be asserted without failing this test.

func rowsFixture(t *testing.T) *File {
	t.Helper()
	return mustParse(t, "t.tsv", strings.Join([]string{
		"input\texpected",
		`a	"A"`,
		"b\tERROR:bad_b",
		"c\tERROR",
	}, "\n"), nil)
}

func check(t *testing.T, r Runner, spec *File, i int) error {
	t.Helper()
	row := spec.Rows[i]
	return r.CheckRow(row, row.Unesc(0), row.Col(1))
}

func TestRunnerPassesMatchingValue(t *testing.T) {
	r := Runner{Parse: func(s string) (any, error) {
		return strings.ToUpper(s), nil
	}}
	if err := check(t, r, rowsFixture(t), 0); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerFailsMismatchedValue(t *testing.T) {
	r := Runner{Parse: func(string) (any, error) { return "WRONG", nil }}

	err := check(t, r, rowsFixture(t), 0)
	if nil == err {
		t.Fatal("expected a failure")
	}
	for _, want := range []string{"t.tsv:2", `got:      "WRONG"`, `expected: "A"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message missing %q:\n%v", want, err)
		}
	}
}

func TestRunnerFailsValueRowThatErrored(t *testing.T) {
	r := Runner{Parse: func(string) (any, error) {
		return nil, errors.New("boom")
	}}

	err := check(t, r, rowsFixture(t), 0)
	if nil == err || !strings.Contains(err.Error(), "boom") {
		t.Errorf("got %v", err)
	}
}

func TestRunnerPassesErrorRowWithNamedCode(t *testing.T) {
	r := Runner{Parse: func(string) (any, error) {
		return nil, &codeError{Code: "bad_b", Msg: "x"}
	}}
	if err := check(t, r, rowsFixture(t), 1); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerFailsErrorRowThatSucceeded(t *testing.T) {
	r := Runner{Parse: func(string) (any, error) { return "fine", nil }}

	err := check(t, r, rowsFixture(t), 1)
	if nil == err {
		t.Fatal("expected a failure")
	}
	for _, want := range []string{
		"should fail with ERROR:bad_b", `returned "fine"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message missing %q:\n%v", want, err)
		}
	}
}

func TestRunnerFailsErrorRowWithWrongCode(t *testing.T) {
	// The point of the code being in the fixture: rejecting the input for
	// the wrong reason is not agreement.
	r := Runner{Parse: func(string) (any, error) {
		return nil, &codeError{Code: "other", Msg: "x"}
	}}

	err := check(t, r, rowsFixture(t), 1)
	if nil == err || !strings.Contains(err.Error(), `code "other", expected "bad_b"`) {
		t.Errorf("got %v", err)
	}
}

func TestRunnerBareErrorRowAcceptsAnyCode(t *testing.T) {
	r := Runner{Parse: func(string) (any, error) {
		return nil, errors.New("no code at all")
	}}
	if err := check(t, r, rowsFixture(t), 2); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerReadsCodeFromMethodOrField(t *testing.T) {
	// *tabnas.TabnasError carries a Code field; another error type may
	// expose a Code() method. Both are read without importing either.
	field := Runner{Parse: func(string) (any, error) {
		return nil, &codeError{Code: "bad_b"}
	}}
	method := Runner{Parse: func(string) (any, error) {
		return nil, &methodError{code: "bad_b"}
	}}

	for name, r := range map[string]Runner{"field": field, "method": method} {
		if err := check(t, r, rowsFixture(t), 1); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRunnerErrorCodeHook(t *testing.T) {
	r := Runner{
		Parse:     func(string) (any, error) { return nil, errors.New("bad_b") },
		ErrorCode: func(err error) string { return err.Error() },
	}
	if err := check(t, r, rowsFixture(t), 1); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerNormalizeHook(t *testing.T) {
	type boxed struct{ Val any }

	r := Runner{
		Parse: func(s string) (any, error) {
			return &boxed{Val: strings.ToUpper(s)}, nil
		},
		Normalize: func(v any) any {
			if b, ok := v.(*boxed); ok {
				return b.Val
			}
			return v
		},
	}
	if err := check(t, r, rowsFixture(t), 0); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerDecodesInputColumn(t *testing.T) {
	spec := mustParse(t, "e.tsv", "input\texpected\na\\tb\t2", nil)
	r := Runner{Parse: func(s string) (any, error) {
		return len(strings.Split(s, "\t")), nil
	}}
	if err := check(t, r, spec, 0); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerSelectsColumnsByName(t *testing.T) {
	spec := mustParse(t, "n.tsv", "expected\tinput\n\"A\"\ta", nil)
	r := Runner{
		Parse:        func(s string) (any, error) { return strings.ToUpper(s), nil },
		InputName:    "input",
		ExpectedName: "expected",
	}
	r.Spec(t, spec)
}

func TestRunnerUnknownColumnName(t *testing.T) {
	// A misspelt column name must be a loud failure, not a silent read of
	// column 0 that then compares the input against itself.
	spec := mustParse(t, "n.tsv", "input\texpected\na\t\"A\"", nil)
	r := Runner{Parse: func(s string) (any, error) { return s, nil }}

	if _, err := r.column(spec, nil, "nosuch", 0); nil == err ||
		!strings.Contains(err.Error(), `no column named "nosuch"`) {
		t.Errorf("err = %v", err)
	}
}

func TestRunnerColumnDefaults(t *testing.T) {
	// The default is column 0 for input and column 1 for expected — and an
	// explicit column 0 for expected must stay column 0, which is why the
	// positions are pointers.
	spec := mustParse(t, "n.tsv", "input\texpected\na\t\"A\"", nil)
	r := Runner{Parse: func(s string) (any, error) { return s, nil }}

	for _, c := range []struct {
		pos  *int
		def  int
		want int
	}{
		{nil, 0, 0},
		{nil, 1, 1},
		{Int(0), 1, 0},
		{Int(2), 1, 2},
	} {
		got, err := r.column(spec, c.pos, "", c.def)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if got != c.want {
			t.Errorf("column(%v, def %d) = %d, want %d", c.pos, c.def, got, c.want)
		}
	}
}

func TestRunnerDirFileAndRow(t *testing.T) {
	// The three entry points a consuming repo actually calls. They report
	// through *testing.T, so what is asserted here is that a passing
	// fixture passes through each of them — the failing path is CheckRow's,
	// covered above.
	dir := t.TempDir()
	for name, body := range map[string]string{
		"one.tsv": "input\texpected\na\t\"A\"\n",
		"two.tsv": "input\texpected\nb\t\"B\"\n# comment\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	r := Runner{Parse: func(s string) (any, error) {
		return strings.ToUpper(s), nil
	}}

	r.Dir(t, dir)
	r.File(t, filepath.Join(dir, "one.tsv"))

	spec := mustLoad(t, filepath.Join(dir, "two.tsv"))
	row := spec.Rows[0]
	r.Row(t, row, row.Unesc(0), row.Col(1))
}

func TestRunnerCaseName(t *testing.T) {
	spec := mustParse(t, "n.tsv", "input\texpected\na\t\"A\"", nil)
	named := false

	r := Runner{
		Parse: func(s string) (any, error) { return strings.ToUpper(s), nil },
		CaseName: func(row *Row, input string) string {
			named = true
			return "custom " + row.Where() + " " + input
		},
	}
	r.Spec(t, spec)

	if !named {
		t.Error("CaseName was not consulted")
	}
}

func TestRunnerBadExpectedJSON(t *testing.T) {
	spec := mustParse(t, "bad.tsv", "input\texpected\na\t{oops", nil)
	r := Runner{Parse: func(s string) (any, error) { return s, nil }}

	err := check(t, r, spec, 0)
	if nil == err || !strings.Contains(err.Error(), "invalid expected JSON") {
		t.Errorf("got %v", err)
	}
}
