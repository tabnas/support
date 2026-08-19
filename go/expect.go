// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// expect.go — reading the expected column, and comparing a parse result
// against it.
//
// A fixture's expected cell is one of two things: a JSON value the parse
// must produce, or an error the parse must raise, written ERROR:<code>.
// The code is part of the contract — "it threw" is not enough, since two
// runtimes that reject the same input for different reasons have not
// actually agreed on anything.
//
// ts/src/expect.ts mirrors all of this.

// ErrorPrefix is the prefix marking an expected-failure cell.
const ErrorPrefix = "ERROR"

// IsErrorExpect reports whether an expected cell is an error expectation.
//
// Exactly "ERROR", or "ERROR:" followed by a code. Note the colon: a bare
// prefix test would read a legitimate "ERRORS" or "ERROR_LIST" expected
// value as a failure expectation and then never check the parse result at
// all — a fixture row that silently tests nothing.
func IsErrorExpect(expected string) bool {
	return ErrorPrefix == expected ||
		strings.HasPrefix(expected, ErrorPrefix+":")
}

// ErrorCode returns the code from an error expectation: "ERROR:unexpected"
// gives "unexpected", and a bare "ERROR" gives "" (meaning "any code").
// A trailing "@<row>:<col>" is a position expectation and is NOT part of
// the code — see ErrorExpect.
// It returns an error when handed a cell that is not an error expectation
// at all.
func ErrorCode(expected string) (string, error) {
	ee, err := ErrorExpect(expected)
	if nil != err {
		return "", err
	}
	return ee.Code, nil
}

// ErrorExpectation is an error expectation, split into the parts a runner
// checks separately.
type ErrorExpectation struct {
	// Code is the expected code, or "" for "any code" (a bare ERROR, or a
	// cell that pins only a position).
	Code string

	// Row and Col are the 1-based source position the error must report,
	// when the cell pins one. HasPos is false otherwise, which is what
	// tells a pinned 0:0 apart from an unpinned position.
	Row    int
	Col    int
	HasPos bool
}

// positionSuffix matches a trailing "@<row>:<col>", anchored at the end
// and digits-only.
//
// "@" rather than another colon because a code is not always a bare
// identifier: the fleet's fixtures already carry "ERROR:a:b" and whole
// diagnostic sentences with embedded colons, so "ERROR:x:1:8" could not be
// split without guessing. Anchoring at the end and requiring digits keeps
// a message that merely CONTAINS an "@" intact.
var positionSuffix = regexp.MustCompile(`@(\d+):(\d+)$`)

// ErrorExpect reads an error expectation.
//
//	ERROR                     any error
//	ERROR:unexpected          that code, position unchecked
//	ERROR:unexpected@1:8      that code, reported at row 1 col 8
//	ERROR:@1:8                any code, reported at row 1 col 8
//
// The position channel exists because a code alone does not pin a
// diagnostic. Two runtimes can agree on "unexpected" and disagree on where
// they say it happened — which is exactly what the fleet audit found, in
// several repos at once, with every code row green. A fixture that pins
// the position makes that disagreement a failing row instead of a
// difference nobody is looking at.
//
// It returns an error when handed a cell that is not an error expectation
// at all.
//
// ts/src/expect.ts mirrors this.
func ErrorExpect(expected string) (ErrorExpectation, error) {
	if !IsErrorExpect(expected) {
		return ErrorExpectation{},
			fmt.Errorf("not an error expectation: %q", expected)
	}

	code := ""
	if ErrorPrefix != expected {
		code = expected[len(ErrorPrefix)+1:]
	}

	loc := positionSuffix.FindStringSubmatchIndex(code)
	if nil == loc {
		return ErrorExpectation{Code: code}, nil
	}

	// Atoi cannot fail here: the regexp already constrained both groups to
	// digits. A very long run of them can overflow, which Atoi reports and
	// which is a malformed fixture, not a mismatch.
	row, err := strconv.Atoi(code[loc[2]:loc[3]])
	if nil != err {
		return ErrorExpectation{}, fmt.Errorf(
			"invalid row in error expectation %q: %w", expected, err)
	}
	col, err := strconv.Atoi(code[loc[4]:loc[5]])
	if nil != err {
		return ErrorExpectation{}, fmt.Errorf(
			"invalid column in error expectation %q: %w", expected, err)
	}

	return ErrorExpectation{
		Code:   code[:loc[0]],
		Row:    row,
		Col:    col,
		HasPos: true,
	}, nil
}

// ParseExpect parses an expected cell as JSON. An empty cell is nil — the
// fixture convention for "no value", as in a utility whose result is
// nothing at all.
//
// The cell is NOT escape-decoded first: it is JSON, and JSON has its own
// escape rules. Decoding it here would turn the two characters \n inside a
// JSON string into a real newline, which is not valid JSON.
//
// Unlike the TypeScript side, an empty cell and a literal "null" both give
// nil — Go has no separate undefined. A cross-runtime fixture should
// therefore write "null" rather than leaving the cell empty when the value
// really is null.
//
// A number too large for float64 (1e400) becomes ±Inf, matching what
// canonical JSON.parse produces. encoding/json rejects it outright, which
// would mean a fixture row that runs in TypeScript and fails to load at
// all in Go — a divergence introduced by this package, in the one place
// it least belongs. A JSON parser's own fixtures reach here: the
// must-accept half of nst/JSONTestSuite includes numbers that overflow.
//
// What neither runtime can do is carry an integer beyond 2^53 exactly;
// JSON.parse reads 9007199254740993 as ...992 and so does this. That
// limit is the canonical runtime's, so it is shared rather than papered
// over — do not pin such an integer in a fixture and expect either side
// to tell it from its neighbour.
func ParseExpect(expected string) (any, error) {
	if "" == expected {
		return nil, nil
	}

	var val any
	err := json.Unmarshal([]byte(expected), &val)
	if nil == err {
		return val, nil
	}

	// Retry with the numbers left as text, which is the only way to tell
	// "out of float64 range" from "not JSON at all". Everything else
	// fails again here, with the same message it would have had.
	if val, ok := parseOverflowing(expected); ok {
		return val, nil
	}

	return nil, fmt.Errorf("invalid expected JSON: %q: %w", expected, err)
}

// parseOverflowing re-reads a cell keeping numbers as text, then widens
// each one with strconv, which answers ±Inf for an out-of-range literal
// instead of failing. It reports false for anything that is not valid
// JSON, so the caller's original error is what the user sees.
func parseOverflowing(expected string) (any, bool) {
	dec := json.NewDecoder(strings.NewReader(expected))
	dec.UseNumber()

	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, false
	}

	// Decode stops at the end of the first value; Unmarshal does not.
	// Without this, trailing content ("1 2") would be accepted here after
	// being rejected above.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}

	return widenNumbers(raw), true
}

// widenNumbers converts the json.Number values left by UseNumber into
// float64, taking strconv's ±Inf for an out-of-range literal. A number
// that is not parseable at all cannot occur — the decoder already
// validated it as JSON.
func widenNumbers(v any) any {
	switch val := v.(type) {
	case json.Number:
		f, err := strconv.ParseFloat(val.String(), 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return val.String()
		}
		return f
	case map[string]any:
		for k, e := range val {
			val[k] = widenNumbers(e)
		}
		return val
	case []any:
		for i, e := range val {
			val[i] = widenNumbers(e)
		}
		return val
	default:
		return v
	}
}

// EqualValue compares two values with JSON semantics: structural,
// key-order independent, -0 equal to 0, an integer equal to the float of
// the same magnitude (Go grammars produce both, encoding/json produces
// only float64), and NaN equal to itself — which == is not, and which a
// fixture cannot express in JSON but an in-language case can.
func EqualValue(got, expected any) bool {
	return equalValue(got, expected, nil)
}

// EqualValueWith is EqualValue with a normalize hook, applied to every
// node on both sides, outermost first. This is where a runtime-specific
// container — an insertion-ordered map, a reference wrapper — is unwrapped
// into the plain value the fixture's JSON describes.
func EqualValueWith(got, expected any, normalize func(any) any) bool {
	return equalValue(got, expected, normalize)
}

func equalValue(a, b any, norm func(any) any) bool {
	if nil != norm {
		a = norm(a)
		b = norm(b)
	}

	if nil == a || nil == b {
		// Both untyped nil, or one is and the other is not. A typed nil —
		// a nil slice or map inside an interface — is not nil here, and is
		// handled below as the empty container it behaves like.
		return nil == a && nil == b
	}

	// Numbers first: the comparison that has to cross Go's numeric types.
	if an, aok := asFloat(a); aok {
		bn, bok := asFloat(b)
		if !bok {
			return false
		}
		if math.IsNaN(an) && math.IsNaN(bn) {
			return true
		}
		return an == bn
	}

	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	}

	ra, rb := reflect.ValueOf(a), reflect.ValueOf(b)

	switch ra.Kind() {
	case reflect.Slice, reflect.Array:
		// A grammar may build []any, []string or []float64; the fixture
		// says only that it is a list of these values.
		if rb.Kind() != reflect.Slice && rb.Kind() != reflect.Array {
			return false
		}
		if ra.Len() != rb.Len() {
			return false
		}
		for i := 0; i < ra.Len(); i++ {
			if !equalValue(ra.Index(i).Interface(), rb.Index(i).Interface(), norm) {
				return false
			}
		}
		return true

	case reflect.Map:
		if rb.Kind() != reflect.Map {
			return false
		}
		if ra.Len() != rb.Len() {
			return false
		}

		// The key must be assignable to the other map's key type before
		// MapIndex sees it — reflect PANICS otherwise, which in a test
		// binary takes down the whole run instead of failing one row. A
		// grammar keying on a defined string type (`map[TokenName]any`)
		// against a JSON expectation (`map[string]any`) reaches exactly
		// that, so convert when the kinds allow it and miss when they
		// do not.
		bkey := rb.Type().Key()
		for _, k := range ra.MapKeys() {
			// The converted key indexes the OTHER map; `k` itself still
			// has to index its own.
			bk := k
			if !k.Type().AssignableTo(bkey) {
				// Kinds must match as well as convert: Go will happily
				// convert an int to a string (as a rune), and a
				// map[int]any is not a map[string]any by any reading.
				if k.Kind() != bkey.Kind() || !k.Type().ConvertibleTo(bkey) {
					return false
				}
				bk = k.Convert(bkey)
			}

			bval := rb.MapIndex(bk)
			if !bval.IsValid() {
				return false
			}
			if !equalValue(ra.MapIndex(k).Interface(), bval.Interface(), norm) {
				return false
			}
		}
		return true

	case reflect.Ptr, reflect.Interface:
		if ra.IsNil() || rb.Kind() != ra.Kind() || rb.IsNil() {
			return ra.IsNil() && rb.Kind() == ra.Kind() && rb.IsNil()
		}
		return equalValue(ra.Elem().Interface(), rb.Elem().Interface(), norm)
	}

	// Anything else — a struct, a channel, a func — is compared as Go
	// compares it. Reaching here means the fixture is pinning a value the
	// JSON model does not describe, which a normalize hook should have
	// unwrapped first.
	return reflect.DeepEqual(a, b)
}

// asFloat widens any Go numeric type to float64. It deliberately excludes
// bool and string: JSON says 1 and "1" are different values, and so does
// every fixture written against it.
func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, nil == err
	}
	return 0, false
}

// FormatValue renders a value for a failure message. JSON where possible,
// so the text lines up with how the fixture wrote it; a readable fallback
// otherwise (NaN, a cycle, a channel).
func FormatValue(val any) string {
	if nil == val {
		return "nil"
	}
	if b, err := json.Marshal(val); nil == err {
		return string(b)
	}
	return fmt.Sprintf("%v", val)
}
