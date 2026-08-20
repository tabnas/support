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

func TestRunnerCheckSpecEmpty(t *testing.T) {
	// The silent-pass path. A fixture that loads but holds nothing runs
	// no assertions, and a runner that reported green over it would hide
	// every other failure this suite exists to catch. CheckSpec is
	// exported precisely so the guard can be asserted — reporting through
	// *testing.T cannot be.
	r := Runner{Parse: func(s string) (any, error) { return s, nil }}
	empty := mustParse(t, "empty.tsv", "input\texpected\n", nil)

	if 0 != len(empty.Rows) {
		t.Fatalf("rows: %d", len(empty.Rows))
	}

	err := r.CheckSpec(empty)
	if nil == err || !strings.Contains(err.Error(), "empty.tsv: no cases") {
		t.Errorf("err = %v", err)
	}
}

func TestRunnerCheckSpecUnknownColumn(t *testing.T) {
	spec := mustParse(t, "n.tsv", "input\texpected\na\t\"A\"", nil)

	in := Runner{
		Parse:     func(s string) (any, error) { return s, nil },
		InputName: "nosuch",
	}
	if err := in.CheckSpec(spec); nil == err ||
		!strings.Contains(err.Error(), "input column") {
		t.Errorf("err = %v", err)
	}

	out := Runner{
		Parse:        func(s string) (any, error) { return s, nil },
		ExpectedName: "nosuch",
	}
	if err := out.CheckSpec(spec); nil == err ||
		!strings.Contains(err.Error(), "expected column") {
		t.Errorf("err = %v", err)
	}

	// A fixture that IS runnable passes the same check.
	ok := Runner{
		Parse:        func(s string) (any, error) { return s, nil },
		InputName:    "input",
		ExpectedName: "expected",
	}
	if err := ok.CheckSpec(spec); err != nil {
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

func TestRunnerParseRow(t *testing.T) {
	// The row-taking hook: a fixture whose other columns take part in the
	// parse — an `opts` column of plugin options is the common one — needs
	// more than the input string. TypeScript's `parse` simply takes the row
	// as a second argument; Go has no optional parameter, so it is a
	// separate field.
	spec := mustParse(t, "o.tsv", strings.Join([]string{
		"input\texpected\topts",
		`a	"A"	upper`,
		`b	"b"	`,
	}, "\n"), nil)

	r := Runner{ParseRow: func(s string, row *Row) (any, error) {
		if "upper" == row.Named("opts") {
			return strings.ToUpper(s), nil
		}
		return s, nil
	}}

	for i := range spec.Rows {
		if err := check(t, r, spec, i); err != nil {
			t.Errorf("row %d: %v", i, err)
		}
	}
}

func TestRunnerParseHookRequired(t *testing.T) {
	// Neither hook set, and both hooks set, are both defects in the caller.
	// Both-set is an error rather than a precedence rule: the two say
	// different things about the same row, and running one of them quietly
	// would hide that the other never ran.
	spec := rowsFixture(t)
	parse := func(string) (any, error) { return nil, nil }
	parseRow := func(string, *Row) (any, error) { return nil, nil }

	for name, c := range map[string]struct {
		runner Runner
		want   string
	}{
		"neither": {Runner{}, "Runner.Parse or Runner.ParseRow is required"},
		"both": {
			Runner{Parse: parse, ParseRow: parseRow},
			"both set",
		},
	} {
		if err := check(t, c.runner, spec, 0); nil == err ||
			!strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: CheckRow err = %v", name, err)
		}
		// And said once at registration, rather than once per row.
		if err := c.runner.CheckSpec(spec); nil == err ||
			!strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: CheckSpec err = %v", name, err)
		}
	}
}

func TestRunnerMatchErrorHook(t *testing.T) {
	// For a grammar with no stable code to pin: the hook replaces the code
	// comparison, so `ERROR:bad_b` can be matched against the message.
	// Without it such a fixture would have to weaken to a bare `ERROR`.
	spec := rowsFixture(t)
	seen := ""

	r := Runner{
		Parse: func(string) (any, error) {
			return nil, errors.New("something bad_b happened")
		},
		// ErrorCode is deliberately set to a wrong answer: MatchError
		// replaces the code comparison entirely, so this must not be read.
		ErrorCode: func(error) string { return "WRONG" },
		MatchError: func(err error, want string, row *Row) bool {
			seen = row.Where()
			return strings.Contains(err.Error(), want)
		},
	}

	if err := check(t, r, spec, 1); err != nil {
		t.Errorf("got %v", err)
	}
	if "t.tsv:3" != seen {
		t.Errorf("MatchError row = %q", seen)
	}

	// A bare ERROR means "any error" and never reaches the hook.
	seen = ""
	if err := check(t, r, spec, 2); err != nil {
		t.Errorf("bare ERROR: %v", err)
	}
	if "" != seen {
		t.Errorf("bare ERROR consulted MatchError at %q", seen)
	}
}

func TestRunnerMatchErrorRejects(t *testing.T) {
	// And it must be able to FAIL a row: a hook that could only pass would
	// turn every error fixture into a silent one.
	r := Runner{
		Parse:      func(string) (any, error) { return nil, errors.New("other") },
		MatchError: func(err error, want string, _ *Row) bool { return false },
	}

	err := check(t, r, rowsFixture(t), 1)
	if nil == err {
		t.Fatal("expected a failure")
	}
	for _, want := range []string{"t.tsv:3", `does not match "bad_b"`, "other"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message missing %q:\n%v", want, err)
		}
	}
}

func TestRunnerParseExpectedHook(t *testing.T) {
	// For a fixture vocabulary wider than JSON. UNDEFINED is the spelling
	// several repos use for "the parse yielded no value at all", which is a
	// different result from null and which JSON cannot say. Go has no
	// undefined, so a sentinel stands in for it here — the point is that
	// the hook, not ParseExpect, decides what the cell means.
	type undef struct{}

	spec := mustParse(t, "w.tsv", strings.Join([]string{
		"input\texpected",
		"a\tUNDEFINED",
		`b	"B"`,
	}, "\n"), nil)

	r := Runner{
		Parse: func(s string) (any, error) {
			if "a" == s {
				return undef{}, nil
			}
			return strings.ToUpper(s), nil
		},
		ParseExpected: func(cell string, row *Row) (any, error) {
			if "UNDEFINED" == cell {
				return undef{}, nil
			}
			return ParseExpect(cell)
		},
	}

	if err := check(t, r, spec, 0); err != nil {
		t.Errorf("UNDEFINED row: %v", err)
	}
	// The cells the hook does not claim keep the ordinary rules.
	if err := check(t, r, spec, 1); err != nil {
		t.Errorf("JSON row: %v", err)
	}
}

func TestRunnerParseExpectedFails(t *testing.T) {
	// A hook that could only pass would turn every row it claims into a
	// silent one.
	type undef struct{}

	spec := mustParse(t, "w.tsv", "input\texpected\na\tUNDEFINED", nil)
	r := Runner{
		Parse: func(string) (any, error) { return nil, nil }, // nil is NOT undef
		ParseExpected: func(cell string, _ *Row) (any, error) {
			if "UNDEFINED" == cell {
				return undef{}, nil
			}
			return ParseExpect(cell)
		},
	}

	if err := check(t, r, spec, 0); nil == err ||
		!strings.Contains(err.Error(), "w.tsv:2") {
		t.Errorf("err = %v", err)
	}
}

func TestRunnerParseExpectedNotReachedForError(t *testing.T) {
	// An ERROR cell is an error expectation before it is anything else.
	seen := false
	r := Runner{
		Parse: func(string) (any, error) { return nil, &methodError{code: "bad_b"} },
		ParseExpected: func(cell string, _ *Row) (any, error) {
			seen = true
			return ParseExpect(cell)
		},
	}

	if err := check(t, r, rowsFixture(t), 1); err != nil {
		t.Errorf("got %v", err)
	}
	if seen {
		t.Error("ParseExpected was consulted for an ERROR row")
	}
}

// --- The position channel: ERROR:<code>@<row>:<col> ---
//
// A code alone does not pin a diagnostic. These tests exist because the
// channel is only worth having if a WRONG position fails: a check that
// cannot fail is the bug it was added to prevent.

// posErr carries a position the way *TabnasError does, as exported int
// fields, so the runner's default reader has to find them by reflection
// with no hook configured.
type posErr struct {
	Code string
	Row  int
	Col  int
}

func (e *posErr) Error() string { return e.Code }

// codeOnlyErr carries a code and NO position, which is what most error
// types in the fleet look like today.
type codeOnlyErr struct{ Code string }

func (e *codeOnlyErr) Error() string { return e.Code }

func posFixture(t *testing.T) *File {
	t.Helper()
	return mustParse(t, "t.tsv", strings.Join([]string{
		"input\texpected",
		"b\tERROR:bad_b@1:8",
		"b\tERROR:@1:8",
		"b\tERROR:bad_b",
	}, "\n"), nil)
}

func TestRunnerPassesPinnedPosition(t *testing.T) {
	r := Runner{Parse: func(string) (any, error) {
		return nil, &posErr{Code: "bad_b", Row: 1, Col: 8}
	}}
	if err := check(t, r, posFixture(t), 0); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerFailsWrongPosition(t *testing.T) {
	// The right code at the wrong place. This is the exact shape the fleet
	// audit found: every code row green, the positions disagreeing.
	r := Runner{Parse: func(string) (any, error) {
		return nil, &posErr{Code: "bad_b", Row: 1, Col: 9}
	}}

	err := check(t, r, posFixture(t), 0)
	if nil == err {
		t.Fatal("expected a failure: right code, wrong column")
	}
	if !strings.Contains(err.Error(), "1:9") ||
		!strings.Contains(err.Error(), "expected 1:8") {
		t.Errorf("failure should report both positions, got %v", err)
	}
}

func TestRunnerFailsMissingPosition(t *testing.T) {
	// An error that names no position has not met an expectation that says
	// where it must fail. Passing here would make the channel silently
	// optional, which is worse than not having it.
	// The right code, so the code check passes and the position check is
	// what has to reject this row.
	r := Runner{Parse: func(string) (any, error) {
		return nil, &codeOnlyErr{Code: "bad_b"}
	}}

	err := check(t, r, posFixture(t), 0)
	if nil == err {
		t.Fatal("expected a failure: error carries no position")
	}
	if !strings.Contains(err.Error(), "no position") {
		t.Errorf("failure should say the error had no position, got %v", err)
	}
}

func TestRunnerPositionWithoutCode(t *testing.T) {
	// `ERROR:@1:8` pins where it failed, not what it was called.
	r := Runner{Parse: func(string) (any, error) {
		return nil, &posErr{Code: "anything_at_all", Row: 1, Col: 8}
	}}
	if err := check(t, r, posFixture(t), 1); err != nil {
		t.Errorf("got %v", err)
	}

	r = Runner{Parse: func(string) (any, error) {
		return nil, &posErr{Code: "anything_at_all", Row: 2, Col: 8}
	}}
	if err := check(t, r, posFixture(t), 1); nil == err {
		t.Fatal("expected a failure: position pinned, code not")
	}
}

func TestRunnerUnpinnedPositionIsNotChecked(t *testing.T) {
	// Row 2 pins only the code. Every existing fixture in the fleet is this
	// shape, so a wrong position here must still PASS — the channel is
	// opt-in, and adding it must not turn 307 green error rows red.
	r := Runner{Parse: func(string) (any, error) {
		return nil, &posErr{Code: "bad_b", Row: 99, Col: 99}
	}}
	if err := check(t, r, posFixture(t), 2); err != nil {
		t.Errorf("an unpinned position must not be checked, got %v", err)
	}
}

func TestRunnerErrorPosHook(t *testing.T) {
	// For an error type that carries its position somewhere the default
	// reader cannot see.
	r := Runner{
		Parse: func(string) (any, error) {
			return nil, errors.New("bad_b at 1:8")
		},
		ErrorCode: func(err error) string { return "bad_b" },
		ErrorPos: func(err error) (int, int, bool) {
			return 1, 8, true
		},
	}
	if err := check(t, r, posFixture(t), 0); err != nil {
		t.Errorf("got %v", err)
	}
}

func TestRunnerPositionCheckedAlongsideMatchError(t *testing.T) {
	// MatchError replaces the CODE comparison, not the position one: a
	// suite that has to match its codes by hand should not thereby lose
	// the ability to pin a position. The hook must also see the code with
	// the position suffix already stripped.
	seen := ""
	r := Runner{
		Parse: func(string) (any, error) {
			return nil, &posErr{Code: "bad_b", Row: 2, Col: 2}
		},
		MatchError: func(_ error, want string, _ *Row) bool {
			seen = want
			return true
		},
	}

	err := check(t, r, posFixture(t), 0)
	if nil == err {
		t.Fatal("expected a failure: MatchError passed, position did not")
	}
	if "bad_b" != seen {
		t.Errorf("MatchError saw %q, want the code without the suffix", seen)
	}
}
