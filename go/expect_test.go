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
		} else if nil == codeErr {
			t.Errorf("%s: ErrorCode(%q) should reject a non-error cell",
				row.Where(), cell)
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
	_, err := ParseExpect("{oops")
	if nil == err || !strings.Contains(err.Error(), "invalid expected JSON") {
		t.Errorf("err = %v", err)
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
