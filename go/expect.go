// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
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
// It returns an error when handed a cell that is not an error expectation
// at all.
func ErrorCode(expected string) (string, error) {
	if !IsErrorExpect(expected) {
		return "", fmt.Errorf("not an error expectation: %q", expected)
	}
	if ErrorPrefix == expected {
		return "", nil
	}
	return expected[len(ErrorPrefix)+1:], nil
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
func ParseExpect(expected string) (any, error) {
	if "" == expected {
		return nil, nil
	}

	var val any
	if err := json.Unmarshal([]byte(expected), &val); err != nil {
		return nil, fmt.Errorf("invalid expected JSON: %q: %w", expected, err)
	}

	return val, nil
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
		for _, k := range ra.MapKeys() {
			// Key equality is by the key's own value, so map[string]any
			// and map[string]int compare on their contents. A key type
			// mismatch simply misses.
			bval := rb.MapIndex(k)
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
