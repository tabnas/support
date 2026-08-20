// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
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

// LoneSurrogateAt returns the position of the first UNPAIRED \uXXXX
// surrogate escape in an expected cell, counted in CODE POINTS, or -1.
//
// Code points because this number crosses the two runtimes. The natural
// index here is a BYTE offset and in ts/src/expect.ts it is a UTF-16
// offset, and those disagree the moment anything non-ASCII precedes the
// escape: for `"é\ud800"` they are 3 and 2. A helper whose whole purpose
// is to keep the two ports saying the same thing cannot report a number
// that depends on which port asked. A code-point count is the same in
// both by definition, and it is also what someone counting characters in
// a TSV cell would arrive at.
//
// WHY THIS IS NOT A CURIOSITY. The two runtimes decode such an escape
// differently, and neither is wrong: JavaScript's JSON.parse preserves
// it, because a JS string is UTF-16 and may hold one; this port's
// encoding/json replaces it with U+FFFD, because a Go string is UTF-8
// and cannot. That is parser/DIVERGENCE.md's first entry, deliberate and
// permanent.
//
// Measured on the same cells:
//
//	cell             TypeScript            Go
//	"\ud800"         1 unit, d800          3 bytes, ef bf bd
//	"a\ud800b"       61 d800 62            61 ef bf bd 62
//	"\ud83d\ude00"  d83d de00             f0 9f 98 80   (a PAIR - agree)
//
// So a SHARED expected cell holding one asks the two runtimes different
// questions and reports agreement either way. It is the one thing a
// shared fixture cannot express, and it fails silently, which is why the
// runner refuses it rather than leaving it to be noticed. Audit item S2.
//
// A PER-RUNTIME column is a different matter: there each runtime reads
// its own cell, so writing the two decodings out explicitly is exactly
// how a divergence register records this one. That path does not go
// through this check.
//
// Only the ESCAPE form is detected, because it is the only one that can
// occur: a fixture file is UTF-8, and a lone surrogate has no UTF-8
// encoding, so it cannot appear literally in one.
func LoneSurrogateAt(cell string) int {
	hex4 := func(at int) int {
		if at+4 > len(cell) {
			return -1
		}
		v := 0
		for _, c := range []byte(cell[at : at+4]) {
			v <<= 4
			switch {
			case '0' <= c && c <= '9':
				v |= int(c - '0')
			case 'a' <= c && c <= 'f':
				v |= int(c-'a') + 10
			case 'A' <= c && c <= 'F':
				v |= int(c-'A') + 10
			default:
				return -1
			}
		}
		return v
	}

	for i := 0; i < len(cell); {
		if '\\' != cell[i] {
			i++
			continue
		}

		// A run of backslashes escapes itself in pairs; only an ODD run
		// leaves a live escape, whose introducer is the byte after the
		// whole run. `\\ud800` is a literal backslash then `ud800`.
		j := i
		for j < len(cell) && '\\' == cell[j] {
			j++
		}
		if 0 == (j-i)%2 {
			i = j
			continue
		}

		start := j - 1
		if j >= len(cell) || 'u' != cell[j] {
			i = j + 1
			continue
		}

		cp := hex4(j + 1)
		if cp < 0 {
			i = j + 1
			continue
		}

		if 0xd800 <= cp && cp <= 0xdbff {
			// A high surrogate is fine if a low follows IMMEDIATELY.
			k := j + 5
			if k+1 < len(cell) && '\\' == cell[k] && 'u' == cell[k+1] {
				if lo := hex4(k + 2); 0xdc00 <= lo && lo <= 0xdfff {
					i = k + 6
					continue
				}
			}
			return utf8.RuneCountInString(cell[:start])
		}
		if 0xdc00 <= cp && cp <= 0xdfff {
			// A paired low was consumed above, so reaching one here
			// means it has no high before it. The prefix always ends on
			// a backslash, so counting runes over it never splits one.
			return utf8.RuneCountInString(cell[:start])
		}

		i = j + 5
	}

	return -1
}

// LoneSurrogateMessage is the message the runner uses when a shared cell
// holds one. Exported so both runtimes say the same thing, and so a
// caller building its own runner can reuse it rather than inventing a
// vaguer one.
func LoneSurrogateMessage(cell string, at int) string {
	return fmt.Sprintf(
		"expected cell holds an unpaired surrogate escape at code point %d: %q\n"+
			"  A shared expected column CANNOT express this: JSON.parse preserves a lone surrogate\n"+
			"  (a JavaScript string is UTF-16) and Go's encoding/json replaces it with U+FFFD\n"+
			"  (a Go string is UTF-8). The two runtimes would be asked different questions and both\n"+
			"  would pass. This is a recorded, permanent divergence - see DIVERGENCE.md.\n"+
			"  Put the case in a per-runtime register column, where each decoding is written out,\n"+
			"  or in each port's own suite with opposite assertions. A surrogate PAIR is fine here.",
		at, cell)
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
// key-order independent, an integer equal to the float of the same
// magnitude (Go grammars produce both, encoding/json produces only
// float64), NaN equal to itself — which == is not, and which a fixture
// cannot express in JSON but an in-language case can — and -0 NOT equal
// to 0.
//
// Those last two are the two halves of ADR-15, and they go opposite ways
// on purpose.
//
// Map key order is OUT of the parsed-value contract. TypeScript cannot
// preserve integer-like key order in a plain object — that is ECMAScript's
// own property-ordering rule, not a porting choice — so making order
// contractual would force that port to return an order-preserving
// container, a breaking change for every consumer, to pin a property no
// format in the fleet defines as significant.
//
// Signed zero is IN it. -0 is representable and distinguishable in both
// runtimes, and a parser that reports 0 for the input -0 has lost
// information the source carried.
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
		// By IEEE bits, not ==, because 0 == -0 is true and signed zero is
		// part of the value contract (ADR-15): a parser that reports 0 for
		// the input -0 has lost information the source carried. Every other
		// finite double has a unique bit pattern, so for those this is
		// exactly ==; NaN is handled above, where payloads must not matter.
		return math.Float64bits(an) == math.Float64bits(bn)
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

	// A defined numeric type — `type Number float64` in a grammar's own
	// package — is none of the cases above, because a type switch matches
	// exact types. Without this it fell through to reflect.DeepEqual, where
	// Number(1) did not even equal 1.0, so EVERY numeric row failed for such
	// a grammar; and Number(-0) compared equal to Number(0), so ADR-15's
	// signed-zero contract was not enforced for it at all.
	//
	// Kinds, not types, is the same choice the map comparison below already
	// makes for a defined string key type.
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64:
		return float64(rv.Uint()), true
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
	// json.Marshal already renders -0 as "-0" (unlike JavaScript's
	// JSON.stringify, which renders it "0"), so nothing extra is needed
	// here for ADR-15's signed zero. ts/src/expect.ts has to do the work.
	if b, err := json.Marshal(val); nil == err {
		return string(b)
	}
	return fmt.Sprintf("%v", val)
}
