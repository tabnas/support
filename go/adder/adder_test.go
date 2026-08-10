// Copyright (c) 2026 tabnas, MIT License

package adder

import (
	"path/filepath"
	"strings"
	"testing"

	tabnas "github.com/tabnas/parser/go"
	support "github.com/tabnas/support/go"
)

// adder_test.go — the end-to-end check on the support module: a real
// grammar, driven entirely by the shared fixtures, through the public API.
//
// ts/test/adder.test.js runs the SAME rows against the SAME grammar in
// TypeScript, so a divergence anywhere in the chain — the escape codec,
// the comment rule, the ERROR:<code> contract, the value comparison —
// turns one of the two runtimes red.

func specDir(t *testing.T) string {
	t.Helper()

	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatalf("%v", err)
	}
	return dir
}

func makeAdder(t *testing.T) *tabnas.Tabnas {
	t.Helper()

	tn, err := Make()
	if err != nil {
		t.Fatalf("%v", err)
	}
	return tn
}

func TestAdderSpec(t *testing.T) {
	tn := makeAdder(t)

	support.Runner{
		Parse: tn.Parse,

		// Assert the error TYPE while pulling out the code: an ERROR row
		// that passed because some unrelated failure escaped the parser
		// would be a green tick over a broken grammar.
		ErrorCode: func(err error) string {
			te, ok := err.(*tabnas.TabnasError)
			if !ok {
				return "not-a-TabnasError:" + err.Error()
			}
			return te.Code
		},
	}.Dir(t, filepath.Join(specDir(t), "adder"))
}

func TestAdderREADMEExamples(t *testing.T) {
	tn := makeAdder(t)

	for src, want := range map[string]float64{
		"1+2+3": 6, "10+20": 30, "12+3+45": 60,
	} {
		got, err := tn.Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %v", src, err)
		}
		if !support.EqualValue(got, want) {
			t.Errorf("Parse(%q) = %v, want %v", src, got, want)
		}
	}
}

func TestAdderIsAPlugin(t *testing.T) {
	// It applies to any bare instance, the same way any grammar does.
	tn := tabnas.Make()
	if err := tn.Use(Adder); err != nil {
		t.Fatalf("%v", err)
	}

	got, err := tn.Parse("1+2")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !support.EqualValue(got, 3) {
		t.Errorf("got %v", got)
	}
}

func TestAdderHoldsNoStateBetweenParses(t *testing.T) {
	tn := makeAdder(t)

	for _, c := range []struct {
		src  string
		want float64
	}{{"1+2", 3}, {"1+2", 3}, {"4", 4}} {
		got, err := tn.Parse(c.src)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.src, err)
		}
		if !support.EqualValue(got, c.want) {
			t.Errorf("Parse(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}

func TestAdderRepeatsInOneStackFrame(t *testing.T) {
	// `r: 'add'` replaces rather than pushes, so the loop does not grow
	// the stack and length is not a limit.
	const n = 500

	terms := make([]string, n)
	want := float64(0)
	for i := 1; i <= n; i++ {
		terms[i-1] = itoa(i)
		want += float64(i)
	}

	got, err := makeAdder(t).Parse(strings.Join(terms, "+"))
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !support.EqualValue(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAdderRunsEveryFixtureRow(t *testing.T) {
	// The runner reports per row, but only a count checked here catches a
	// fixture that stopped being loaded at all.
	dir := specDir(t)

	basic, err := support.LoadSpec(filepath.Join(dir, "adder", "basic.tsv"), nil)
	if err != nil {
		t.Fatalf("%v", err)
	}
	errs, err := support.LoadSpec(filepath.Join(dir, "adder", "errors.tsv"), nil)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if 15 >= len(basic.Rows) {
		t.Errorf("basic.tsv rows: %d", len(basic.Rows))
	}
	if 5 >= len(errs.Rows) {
		t.Errorf("errors.tsv rows: %d", len(errs.Rows))
	}
}

func itoa(i int) string {
	if 0 == i {
		return "0"
	}
	var b []byte
	for ; 0 < i; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}
