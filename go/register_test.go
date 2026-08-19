// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// register_test.go — the register's own failure behaviour.
//
// What needs its own test is not that a regression fails; the runner
// underneath already does that. It is the property a plain fixture does
// NOT have: that a FIXED divergence fails too, and says so in words that
// send the reader to delete the row rather than to restore the old
// behaviour.
//
// ts/test/register.test.js mirrors every case here.

type regErr struct{ Code string }

func (e *regErr) Error() string { return e.Code }

func registerFixture(t *testing.T) *File {
	t.Helper()
	return mustParse(t, "d.tsv", strings.Join([]string{
		"input\tts\tgo",
		`a	"A"	"a"`,
		"b\tERROR:bad_b\t\"b\"",
	}, "\n"), nil)
}

// goPort is the Go column's recorded behaviour: uppercase nothing, and
// never fail.
func goPort(s string) (any, error) { return s, nil }

func goRegister(parse func(string) (any, error)) Register {
	return Register{
		Runner:   Runner{Parse: parse},
		Runtime:  "go",
		Runtimes: []string{"ts", "go"},
	}
}

func checkReg(t *testing.T, g Register, spec *File, i int) error {
	t.Helper()
	row := spec.Rows[i]
	return g.CheckRow(row, row.Unesc(row.IndexOf("input")))
}

func TestRegisterPassesWhenTheDivergenceHolds(t *testing.T) {
	g := goRegister(goPort)
	for i := range registerFixture(t).Rows {
		if err := checkReg(t, g, registerFixture(t), i); err != nil {
			t.Errorf("row %d: %v", i, err)
		}
	}
}

// The case this whole mechanism exists for.
func TestRegisterFailsWhenTheDivergenceIsClosed(t *testing.T) {
	// Go repaired to agree with TypeScript: it now uppercases, which is
	// what the ts column records.
	g := goRegister(func(s string) (any, error) {
		return strings.ToUpper(s), nil
	})

	err := checkReg(t, g, registerFixture(t), 0)
	if nil == err {
		t.Fatal("expected a failure: the ports now agree")
	}
	if !strings.Contains(err.Error(), "CLOSED") ||
		!strings.Contains(err.Error(), "DELETE this row") {
		t.Errorf("failure must say the divergence is closed and send the "+
			"reader to delete the row, got: %v", err)
	}
	// And it must NOT read as a regression, which is the opposite
	// conclusion and the one a plain fixture would report.
	if strings.Contains(err.Error(), "expected") {
		t.Errorf("failure reads as a regression: %v", err)
	}
}

// Closed the other way round: a row whose recorded Go answer is an error.
func TestRegisterFailsWhenAnErrorRowCloses(t *testing.T) {
	g := goRegister(func(s string) (any, error) {
		return nil, &regErr{Code: "bad_b"}
	})

	err := checkReg(t, g, registerFixture(t), 1)
	if nil == err {
		t.Fatal("expected a failure: go now fails the way ts does")
	}
	if !strings.Contains(err.Error(), "CLOSED") {
		t.Errorf("got: %v", err)
	}
}

func TestRegisterFailsAnOrdinaryRegression(t *testing.T) {
	// Neither the recorded go answer nor the recorded ts one: a real
	// regression, which must still report as a mismatch rather than as a
	// closed divergence.
	g := goRegister(func(s string) (any, error) { return "something else", nil })

	err := checkReg(t, g, registerFixture(t), 0)
	if nil == err {
		t.Fatal("expected a failure")
	}
	if strings.Contains(err.Error(), "CLOSED") {
		t.Errorf("a regression must not be reported as a closed "+
			"divergence, got: %v", err)
	}
}

func TestRegisterRejectsARowThatRecordsNoDivergence(t *testing.T) {
	// Both columns say the same thing, so the row asserts nothing and
	// would pass forever. That is the shape of the prose claims this
	// mechanism replaces, so it is a failure, not a pass.
	spec := mustParse(t, "d.tsv", strings.Join([]string{
		"input\tts\tgo",
		`a	"a"	"a"`,
	}, "\n"), nil)

	err := checkReg(t, goRegister(goPort), spec, 0)
	if nil == err {
		t.Fatal("expected a failure: the row records no divergence")
	}
	if !strings.Contains(err.Error(), "records no divergence") {
		t.Errorf("got: %v", err)
	}
}

func TestRegisterRejectsBadConfiguration(t *testing.T) {
	spec := registerFixture(t)

	cases := []struct {
		name string
		reg  Register
		want string
	}{
		{"no runtime",
			Register{Runner: Runner{Parse: goPort}, Runtimes: []string{"ts", "go"}},
			"Runtime is required"},
		{"runtime not in runtimes",
			Register{Runner: Runner{Parse: goPort}, Runtime: "rust",
				Runtimes: []string{"ts", "go"}},
			"must include"},
		{"one runtime is not a divergence",
			Register{Runner: Runner{Parse: goPort}, Runtime: "go",
				Runtimes: []string{"go"}},
			"at least two runtimes"},
	}

	for _, c := range cases {
		err := checkReg(t, c.reg, spec, 0)
		if nil == err {
			t.Errorf("%s: expected a failure", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want a message containing %q",
				c.name, err, c.want)
		}
	}
}

// The shared fixture, run the way a real repo would run it.
func TestRegisterSpecFile(t *testing.T) {
	g := Register{
		Runner: Runner{Parse: func(s string) (any, error) {
			switch s {
			case "a":
				return "a", nil
			case "b":
				return "b", nil
			case "c":
				return nil, &regErr{Code: "bad_c"}
			}
			return nil, errors.New("unexpected input " + s)
		}},
		Runtime:  "go",
		Runtimes: []string{"ts", "go"},
	}
	g.File(t, filepath.Join(specDir(t), "register", "divergent.tsv"))
}
