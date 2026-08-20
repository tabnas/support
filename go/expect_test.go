// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// expect_test.go — reading the expected column, and comparing against it.
// ts/test/expect.test.js runs the same fixtures.

func TestExpectErrorSpec(t *testing.T) {
	spec := mustLoad(t, filepath.Join(specDir(t), "util", "expect-error.tsv"))
	if 0 == len(spec.Rows) {
		t.Fatal("no cases")
	}

	for _, row := range spec.Rows {
		cell := row.Named("expected")

		wantIsError, err := ParseExpect(row.Named("iserror"))
		if err != nil {
			t.Fatalf("%s: %v", row.Where(), err)
		}

		got := IsErrorExpect(cell)
		if got != wantIsError {
			t.Errorf("%s: IsErrorExpect(%q) = %v, want %v",
				row.Where(), cell, got, wantIsError)
			continue
		}

		// A cell that IS an error expectation but a malformed one: reading
		// it must fail rather than yield a position nobody meant.
		wantBad, err := ParseExpect(row.Named("bad"))
		if err != nil {
			t.Fatalf("%s: %v", row.Where(), err)
		}
		if true == wantBad {
			if _, err := ErrorExpect(cell); nil == err {
				t.Errorf("%s: ErrorExpect(%q) should reject a 0 position",
					row.Where(), cell)
			}
			if _, err := ErrorCode(cell); nil == err {
				t.Errorf("%s: ErrorCode(%q) should reject a 0 position",
					row.Where(), cell)
			}
			continue
		}

		code, codeErr := ErrorCode(cell)

		if true == wantIsError {
			if codeErr != nil {
				t.Errorf("%s: ErrorCode(%q): %v", row.Where(), cell, codeErr)
				continue
			}
			wantCode, err := ParseExpect(row.Named("code"))
			if err != nil {
				t.Fatalf("%s: %v", row.Where(), err)
			}
			if code != wantCode {
				t.Errorf("%s: ErrorCode(%q) = %q, want %q",
					row.Where(), cell, code, wantCode)
			}

			// The position channel. row/col are empty for a cell that
			// pins no position, and ParseExpect reads an empty cell as
			// nil — so wantPos below is false exactly when the cell pins
			// nothing, and both kinds of row are checked by the same
			// assertions. Neither kind can go unchecked.
			ee, eeErr := ErrorExpect(cell)
			if nil != eeErr {
				t.Errorf("%s: ErrorExpect(%q): %v", row.Where(), cell, eeErr)
				continue
			}
			wantRow, err := ParseExpect(row.Named("row"))
			if err != nil {
				t.Fatalf("%s: %v", row.Where(), err)
			}
			wantCol, err := ParseExpect(row.Named("col"))
			if err != nil {
				t.Fatalf("%s: %v", row.Where(), err)
			}

			wantPos := nil != wantRow
			if ee.HasPos != wantPos {
				t.Errorf("%s: ErrorExpect(%q).HasPos = %v, want %v",
					row.Where(), cell, ee.HasPos, wantPos)
				continue
			}
			if !wantPos {
				continue
			}

			// ParseExpect reads a JSON number as float64.
			if float64(ee.Row) != wantRow || float64(ee.Col) != wantCol {
				t.Errorf("%s: ErrorExpect(%q) = %d:%d, want %v:%v",
					row.Where(), cell, ee.Row, ee.Col, wantRow, wantCol)
			}
		} else {
			if nil == codeErr {
				t.Errorf("%s: ErrorCode(%q) should reject a non-error cell",
					row.Where(), cell)
			}
			if _, err := ErrorExpect(cell); nil == err {
				t.Errorf("%s: ErrorExpect(%q) should reject a non-error cell",
					row.Where(), cell)
			}
		}
	}
}

func TestParseExpect(t *testing.T) {
	// An empty cell is no value.
	if got, err := ParseExpect(""); nil != err || nil != got {
		t.Errorf("ParseExpect(\"\") = %v, %v", got, err)
	}

	// The cell is read as JSON, not as escaped text: the two characters
	// `\n` inside a JSON string are a newline by JSON's own rules.
	if got, _ := ParseExpect(`"a\nb"`); "a\nb" != got {
		t.Errorf("got %q", got)
	}
	if got, _ := ParseExpect("null"); nil != got {
		t.Errorf("got %v", got)
	}
	if got, _ := ParseExpect("0"); float64(0) != got {
		t.Errorf("got %v", got)
	}
	if got, _ := ParseExpect("false"); false != got {
		t.Errorf("got %v", got)
	}

	if !EqualValue(mustExpect(t, `{"a":[1,2]}`),
		map[string]any{"a": []any{1, 2}}) {
		t.Error("object")
	}
}

func TestParseExpectBadJSON(t *testing.T) {
	for _, cell := range []string{
		"{oops", "1 2", "[1,", "nope", `"unterminated`, "",
	} {
		if "" == cell {
			continue // An empty cell is "no value", not bad JSON.
		}
		_, err := ParseExpect(cell)
		if nil == err || !strings.Contains(err.Error(), "invalid expected JSON") {
			t.Errorf("ParseExpect(%q): err = %v", cell, err)
		}
	}
}

// TestParseExpectOverflow pins the behaviour canonical JSON.parse has:
// a number beyond float64 range reads as ±Inf. encoding/json rejects the
// literal outright, so ParseExpect goes out of its way to match — a
// fixture row that runs in TypeScript must not fail to LOAD in Go, which
// would be a divergence introduced by this package in the one place it
// least belongs.
func TestParseExpectOverflow(t *testing.T) {
	cases := []struct {
		cell string
		want any
	}{
		{"1e400", math.Inf(1)},
		{"-1e400", math.Inf(-1)},
		{"1e999", math.Inf(1)},
		{"[1e400]", []any{math.Inf(1)}},
		{`{"a":1e400}`, map[string]any{"a": math.Inf(1)}},
		{`{"a":[1,1e400]}`, map[string]any{"a": []any{float64(1), math.Inf(1)}}},

		// Nothing else changes: the ordinary path is untouched.
		{"1", float64(1)},
		{`{"a":1}`, map[string]any{"a": float64(1)}},
	}

	for _, c := range cases {
		got, err := ParseExpect(c.cell)
		if err != nil {
			t.Errorf("ParseExpect(%q): %v", c.cell, err)
			continue
		}
		if !EqualValue(got, c.want) {
			t.Errorf("ParseExpect(%q) = %v, want %v", c.cell, got, c.want)
		}
	}

	// Infinity compares equal to itself, so an overflow row can be pinned
	// in a fixture at all.
	inf, _ := ParseExpect("1e400")
	other, _ := ParseExpect("1e999")
	if !EqualValue(inf, other) {
		t.Error("Inf should equal Inf")
	}
	if EqualValue(inf, math.Inf(-1)) {
		t.Error("+Inf should not equal -Inf")
	}
}

// TestParseExpectBigInteger records a limit rather than a behaviour: the
// canonical runtime reads 9007199254740993 as ...992 and so does this, so
// neither side can tell such an integer from its neighbour. Pinning Go to
// exact integers would make it REJECT rows TypeScript accepts, which is
// the divergence this package exists to prevent.
func TestParseExpectBigInteger(t *testing.T) {
	got, err := ParseExpect("9007199254740993")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if want := float64(9007199254740992); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func mustExpect(t *testing.T, cell string) any {
	t.Helper()
	val, err := ParseExpect(cell)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return val
}

func TestEqualValueSpec(t *testing.T) {
	spec := mustLoad(t, filepath.Join(specDir(t), "util", "value-equal.tsv"))
	if 0 == len(spec.Rows) {
		t.Fatal("no cases")
	}

	for _, row := range spec.Rows {
		a := mustExpect(t, row.Named("a"))
		b := mustExpect(t, row.Named("b"))
		want := mustExpect(t, row.Named("equal"))

		if got := EqualValue(a, b); got != want {
			t.Errorf("%s: EqualValue(%s, %s) = %v, want %v",
				row.Where(), row.Named("a"), row.Named("b"), got, want)
		}

		// Equality is symmetric; a comparison that is not would make a
		// fixture's meaning depend on which column it was written in.
		if got := EqualValue(b, a); got != want {
			t.Errorf("%s: reversed = %v, want %v", row.Where(), got, want)
		}
	}
}

// TestEqualValueAcrossGoTypes covers what a fixture cannot say: the
// expected side always arrives from encoding/json as float64, but a Go
// grammar's result can be any numeric type, and a comparison that missed
// that would fail every integer row for the Go runtime alone.
func TestEqualValueAcrossGoTypes(t *testing.T) {
	cases := []struct {
		got, want any
		equal     bool
	}{
		// Every numeric type asFloat claims to widen — an untested claim
		// is a claim.
		{int(1), float64(1), true},
		{int8(1), float64(1), true},
		{int16(1), float64(1), true},
		{int32(1), float64(1), true},
		{int64(1), float64(1), true},
		{uint(1), float64(1), true},
		{uint8(1), float64(1), true},
		{uint16(1), float64(1), true},
		{uint32(1), float64(1), true},
		{uint64(1), float64(1), true},
		{float32(1.5), float64(1.5), true},
		{json.Number("1"), float64(1), true},
		{json.Number("1.5"), float64(1.5), true},
		{json.Number("nope"), float64(1), false},
		{int(2), float64(1), false},
		{[]string{"a", "b"}, []any{"a", "b"}, true},
		{[]int{1, 2}, []any{float64(1), float64(2)}, true},
		{[]int{1, 2}, []any{float64(1)}, false},
		{map[string]int{"a": 1}, map[string]any{"a": float64(1)}, true},
		{map[string]int{"a": 1}, map[string]any{"a": float64(2)}, false},
		{map[string]int{"a": 1}, map[string]any{"b": float64(1)}, false},

		// A number is never a string, and a bool is never a number —
		// however the runtime chose to store them.
		{int(1), "1", false},
		{true, int(1), false},
		{"true", true, false},

		// A typed nil slice behaves as the empty list it is, but is still
		// not the JSON value null.
		{[]any(nil), []any{}, true},
		{[]any(nil), nil, false},
		{map[string]any(nil), map[string]any{}, true},
	}

	for _, c := range cases {
		if got := EqualValue(c.got, c.want); got != c.equal {
			t.Errorf("EqualValue(%#v, %#v) = %v, want %v",
				c.got, c.want, got, c.equal)
		}
	}
}

// TestEqualValueDefinedKeyType covers the reflect hazard: MapIndex PANICS
// when handed a key that is not assignable to the other map's key type,
// which in a test binary takes down the whole run instead of failing one
// row. A grammar keying on a defined string type reaches exactly that.
func TestEqualValueDefinedKeyType(t *testing.T) {
	type tokenName string
	type otherName string
	type tokenID int

	cases := []struct {
		got, want any
		equal     bool
	}{
		{map[tokenName]any{"a": 1}, map[string]any{"a": float64(1)}, true},
		{map[string]any{"a": float64(1)}, map[tokenName]any{"a": 1}, true},
		{map[tokenName]any{"a": 1}, map[otherName]any{"a": 1}, true},
		{map[tokenName]any{"a": 1}, map[string]any{"a": float64(2)}, false},
		{map[tokenName]any{"a": 1}, map[string]any{"b": float64(1)}, false},

		// Go will convert an int to a string (as a rune), which would
		// make map[tokenID]any silently index a map[string]any. Kinds
		// have to match, not merely convert.
		{map[tokenID]any{97: 1}, map[string]any{"a": float64(1)}, false},
		{map[string]any{"a": float64(1)}, map[tokenID]any{97: 1}, false},
	}

	for _, c := range cases {
		got := EqualValue(c.got, c.want)
		if got != c.equal {
			t.Errorf("EqualValue(%#v, %#v) = %v, want %v",
				c.got, c.want, got, c.equal)
		}
	}
}

func TestEqualValueNaN(t *testing.T) {
	// Not expressible in a fixture — JSON has no NaN — but a grammar can
	// produce one, and == would report it unequal to itself.
	nan := math.NaN()
	if !EqualValue(nan, nan) {
		t.Error("NaN should equal NaN")
	}
	if EqualValue(nan, float64(0)) {
		t.Error("NaN should not equal 0")
	}
	if !EqualValue([]any{nan}, []any{nan}) {
		t.Error("[NaN] should equal [NaN]")
	}
}

func TestEqualValueWithNormalize(t *testing.T) {
	// The hook is how a runtime-specific container — an insertion-ordered
	// map, a reference wrapper — is unwrapped into the plain value the
	// fixture describes.
	type ordered struct{ Vals map[string]any }

	normalize := func(v any) any {
		if o, ok := v.(*ordered); ok {
			return o.Vals
		}
		return v
	}

	got := &ordered{Vals: map[string]any{
		"a": &ordered{Vals: map[string]any{"b": 1}},
	}}
	want := map[string]any{"a": map[string]any{"b": float64(1)}}

	if !EqualValueWith(got, want, normalize) {
		t.Error("normalized compare should match")
	}
	if EqualValueWith(got, map[string]any{"a": map[string]any{"b": 2}}, normalize) {
		t.Error("normalized compare should not match a different value")
	}
	if EqualValue(got, want) {
		t.Error("un-normalized compare should not match")
	}
}

func TestFormatValue(t *testing.T) {
	cases := []struct {
		val  any
		want string
	}{
		{map[string]any{"a": 1}, `{"a":1}`},
		{[]any{1, "x"}, `[1,"x"]`},
		{nil, "nil"},
		{[]any(nil), "null"},
		{"text", `"text"`},
		{1.5, "1.5"},

		// json.Marshal refuses NaN; a failure message still has to say
		// something, since throwing here would replace the real failure
		// with a mystery.
		{math.NaN(), "NaN"},
	}

	for _, c := range cases {
		if got := FormatValue(c.val); got != c.want {
			t.Errorf("FormatValue(%#v) = %q, want %q", c.val, got, c.want)
		}
	}
}

// Audit item S2. A lone surrogate escape in a SHARED expected cell is
// decoded differently by the two runtimes - U+FFFD here, preserved in
// TypeScript - so the cell asks them different questions and both pass.
// The runner refuses such a cell; this pins the detector it uses, and
// ts/test/expect.test.js runs the same rows.
//
// `cell` is the RAW cell text: Named does not escape-decode, which is
// what makes the two `\\ud800` rows meaningful. Those are an escaped
// backslash followed by the letters `ud800`, not an escape at all, and
// they are what a detector that merely searched for the text would get
// wrong.
func TestLoneSurrogateSpec(t *testing.T) {
	spec := mustLoad(t, filepath.Join(specDir(t), "util", "lone-surrogate.tsv"))
	if 0 == len(spec.Rows) {
		t.Fatal("no cases")
	}
	for _, row := range spec.Rows {
		cell := row.Named("cell")
		wantAny, err := ParseExpect(row.Named("at"))
		if nil != err {
			t.Fatalf("%s: bad at: %v", row.Where(), err)
		}
		want, ok := wantAny.(float64)
		if !ok {
			t.Fatalf("%s: at is not a number: %#v", row.Where(), wantAny)
		}
		if got := LoneSurrogateAt(cell); got != int(want) {
			t.Errorf("%s: LoneSurrogateAt(%q) = %d, want %d",
				row.Where(), cell, got, int(want))
		}
	}
}

// The detector is only half of it: what matters is that a fixture row
// carrying one FAILS rather than passing in both runtimes.
func TestRunnerRefusesLoneSurrogateCell(t *testing.T) {
	spec, err := ParseSpec("inline.tsv", "input\texpected\nx\t\"A\"", nil)
	if nil != err {
		t.Fatal(err)
	}
	row := spec.Rows[0]

	r := Runner{Parse: func(string) (any, error) { return "x", nil }}
	rerr := r.CheckRow(row, "x", `"\ud800"`)
	if nil == rerr {
		t.Fatal("a lone surrogate cell was accepted")
	}
	if !strings.Contains(rerr.Error(), "unpaired surrogate escape at code point 1") {
		t.Errorf("wrong message: %v", rerr)
	}

	// A PAIR is fine - both runtimes decode it to the same character, so
	// the shared column expresses it perfectly well. Without this the
	// refusal is also satisfied by rejecting every `\u` escape.
	pair := Runner{Parse: func(string) (any, error) {
		return "\U0001F600", nil
	}}
	if perr := pair.CheckRow(row, "x", `"\ud83d\ude00"`); nil != perr {
		t.Errorf("a surrogate PAIR was refused: %v", perr)
	}
}

// ParseExpected exists because a fixture's vocabulary can be wider than
// JSON, and a wider vocabulary need not read \uXXXX as an escape at all.
// A hook treating the cell as opaque text is asking a question both
// runtimes answer identically, so refusing it would be the guard
// inventing a problem. Raised in review.
//
// A hook whose syntax DOES use JSON escapes should call LoneSurrogateAt
// itself - which is why it is exported, and why this asserts the hook
// still SEES the raw cell.
func TestCustomParseExpectedKeepsItsOwnVocabulary(t *testing.T) {
	spec, err := ParseSpec("inline.tsv", "input\texpected\nx\t\"A\"", nil)
	if nil != err {
		t.Fatal(err)
	}
	row := spec.Rows[0]

	seen := ""
	r := Runner{
		Parse: func(string) (any, error) { return `RAW:\ud800`, nil },
		ParseExpected: func(cell string, _ *Row) (any, error) {
			seen = cell
			return cell, nil
		},
	}
	if rerr := r.CheckRow(row, "x", `RAW:\ud800`); nil != rerr {
		t.Fatalf("a custom hook's cell was refused: %v", rerr)
	}
	if seen != `RAW:\ud800` {
		t.Errorf("hook saw %q, want the raw cell", seen)
	}
}
