// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"fmt"
	"strings"
	"testing"
)

// register.go — a divergence register: the places two ports of one grammar
// DISAGREE, recorded in a fixture both ports execute.
//
// The audit this exists for found 29 recorded divergence claims
// contradicted by execution, and one file that had been wrong in BOTH
// directions at once. Prose does not hold. What did hold was the one
// mechanism that ran: a register whose rows are executed, so a row that
// stops being true fails the build.
//
// The property that makes it work is not that a regression fails. It is
// that a FIX fails too. When a port is repaired to agree with the other,
// the register still claims they differ, so the suite goes red and names
// the row to delete. A register cannot quietly outlive the divergence it
// records — which is precisely how the 29 prose claims survived.
//
// ts/src/register.ts mirrors all of this.

// Register is a fixture of recorded divergences, run by both ports.
//
// Each row gives an input and one cell per runtime, written in the same
// vocabulary as an ordinary fixture's expected column — a JSON value, or
// ERROR:<code>. Every row is checked three ways:
//
//  1. The row must actually record a divergence. If every runtime cell
//     says the same thing, the row asserts nothing and would pass
//     forever; that is the shape of the claims this mechanism replaces.
//  2. This runtime must still produce what the register says it does.
//  3. When it does not, and it now produces what ANOTHER runtime's cell
//     says, the failure says the divergence is CLOSED and names the row to
//     delete — rather than reporting it as a regression, which is the
//     opposite conclusion.
type Register struct {
	// Runner supplies the parse hook and every comparison rule. The
	// register adds which column to read and what a mismatch means; it
	// does not get its own idea of what "equal" is.
	Runner Runner

	// Runtime is the column holding THIS runtime's answer — "go" in the
	// Go suite, "ts" in the TypeScript one. The two suites run the same
	// file and read different columns of it.
	Runtime string

	// Runtimes is every runtime column in the file, this one included.
	// Named rather than inferred from the header, because inferring would
	// silently treat a `note` or `issue` column as a runtime and then
	// report that the ports "agree" with a sentence.
	Runtimes []string
}

// check reports why a register cannot be run, or nil when it can.
func (g Register) check() error {
	if "" == g.Runtime {
		return fmt.Errorf("Register: Runtime is required")
	}
	if 2 > len(g.Runtimes) {
		return fmt.Errorf(
			"Register: a divergence needs at least two runtimes, got %v",
			g.Runtimes)
	}
	for _, name := range g.Runtimes {
		if name == g.Runtime {
			return nil
		}
	}
	return fmt.Errorf("Register: Runtimes must include %q, got %v",
		g.Runtime, g.Runtimes)
}

// checkColumns reports a runtime column the file does not have.
//
// Row.Named returns "" for an absent column, and in Go an empty expectation
// means nil — so a parser returning (nil, nil) would make such a row PASS,
// and a typo in Runtimes would otherwise make every row look like agreement
// and be blamed on the fixture. TypeScript's Row.resolve throws instead, so
// without this the two ports disagree about a malformed register.
func (g Register) checkColumns(row *Row) error {
	for _, name := range g.Runtimes {
		if 0 > row.IndexOf(name) {
			return fmt.Errorf("no column named %q (Register.Runtimes)", name)
		}
	}
	return nil
}

// rowRunner returns a Runner reading THIS runtime's column, and — when
// given a row's outcome holder — answering from ONE evaluation however many
// times it is asked.
//
// Every comparison a row needs goes through the ordinary Runner, so the
// register never develops its own idea of what "equal" means. But the
// Runner parses as part of comparing, and calling it three times would
// parse three times: a parse hook that carries state, or a parser whose
// state changes after an error, could then answer differently on the second
// call and turn a genuine regression into a reported "closed divergence" —
// the opposite conclusion.
func (g Register) rowRunner(once *parseOnce) Runner {
	r := g.Runner
	r.Expected = nil
	r.ExpectedName = g.Runtime

	if nil != once {
		inner, _ := g.Runner.parser()
		r.Parse = nil
		r.ParseRow = func(input string, row *Row) (any, error) {
			if !once.done {
				once.value, once.err = inner(input, row)
				once.done = true
			}
			return once.value, once.err
		}
	}

	return r
}

// parseOnce holds one row's parse outcome, replayed for every comparison.
type parseOnce struct {
	done  bool
	value any
	err   error
}

// File loads one register file by path and runs it.
func (g Register) File(t *testing.T, path string) {
	t.Helper()

	if err := g.check(); err != nil {
		t.Fatalf("%v", err)
	}

	spec, err := LoadSpec(path, g.Runner.Load)
	if err != nil {
		t.Fatalf("%v", err)
	}
	g.Spec(t, spec)
}

// Spec runs every row of an already-loaded register.
func (g Register) Spec(t *testing.T, spec *File) {
	t.Helper()

	t.Run("divergence register: "+spec.Name, func(t *testing.T) {
		// Checked through a runner whose expectation column IS this
		// runtime's, because that is the column a register reads. Handing
		// CheckSpec the caller's ordinary Expected/ExpectedName would
		// reject a perfectly good register whose header is `input ts go`
		// merely because the Runner it was built from names an "expected"
		// column that a register never consults.
		if err := g.rowRunner(nil).CheckSpec(spec); err != nil {
			t.Fatalf("%v", err)
		}

		if err := g.checkColumns(spec.Rows[0]); err != nil {
			t.Fatalf("%s: %v", spec.Name, err)
		}

		inCol, _ := g.Runner.column(spec, g.Runner.Input, g.Runner.InputName, 0)

		for _, row := range spec.Rows {
			input := row.Unesc(inCol)
			name := fmt.Sprintf("row %d: %q", row.Line, input)
			if nil != g.Runner.CaseName {
				name = g.Runner.CaseName(row, input)
			}

			t.Run(name, func(t *testing.T) {
				if err := g.CheckRow(row, input); err != nil {
					t.Error(err)
				}
			})
		}
	})
}

// CheckRow runs one row and returns the failure rather than reporting it,
// for a suite that does its own reporting.
func (g Register) CheckRow(row *Row, input string) error {
	if err := g.check(); err != nil {
		return err
	}
	// Checked here too, not only in Spec: CheckRow is public, and a caller
	// driving it directly would otherwise read a missing column as an empty
	// expectation, which in Go means nil.
	if err := g.checkColumns(row); err != nil {
		return fmt.Errorf("%s: %v", row.Where(), err)
	}

	mine := row.Named(g.Runtime)

	type other struct{ name, cell string }
	var others []other
	agree := true
	for _, name := range g.Runtimes {
		if name == g.Runtime {
			continue
		}
		cell := row.Named(name)
		others = append(others, other{name, cell})
		if !g.sameExpectation(cell, mine) {
			agree = false
		}
	}

	// 1. Does this row record a divergence at all?
	if agree {
		return fmt.Errorf(
			"%s: every runtime column means %q, so this row records no "+
				"divergence and can never fail meaningfully. Delete it, or "+
				"correct the cells to what the runtimes actually do.",
			row.Where(), mine)
	}

	// ONE parse per row, whatever it ends up being compared against.
	runner := g.rowRunner(&parseOnce{})

	// 2. Does this runtime still do what the register says?
	mismatch := runner.CheckRow(row, input, mine)
	if nil == mismatch {
		return nil
	}

	// 3. It does not. Which of the OTHERS does it now agree with?
	var converged []other
	for _, o := range others {
		if nil == runner.CheckRow(row, input, o.cell) {
			converged = append(converged, o)
		}
	}

	// 4. None. An ordinary regression; the runner's own message says what
	//    was produced and what was expected.
	if 0 == len(converged) {
		return mismatch
	}

	names := make([]string, 0, len(converged))
	for _, c := range converged {
		names = append(names, c.name)
	}

	// Converged with EVERY other runtime: the divergence is gone.
	if len(converged) == len(others) {
		return fmt.Errorf(
			"%s: this divergence is CLOSED. %s now produces what the %s "+
				"column(s) record (%q), not its own (%q).\n"+
				"  This is the register working: a fixed divergence fails as "+
				"loudly as a regressed one, so the row cannot outlive it.\n"+
				"  DELETE this row. Do not edit it to match — that would "+
				"record a divergence that no longer exists, which is what "+
				"this mechanism exists to prevent.",
			row.Where(), g.Runtime, strings.Join(names, ", "),
			converged[0].cell, mine)
	}

	// Converged with SOME. The row still records a live disagreement
	// between the runtimes that have not converged, so deleting it would
	// drop that coverage. Only this runtime's own column is stale.
	var live []string
	for _, o := range others {
		found := false
		for _, c := range converged {
			if c.name == o.name {
				found = true
			}
		}
		if !found {
			live = append(live, o.name)
		}
	}

	return fmt.Errorf(
		"%s: this divergence is PARTIALLY closed. %s now agrees with %s, "+
			"but not with %s.\n"+
			"  Do NOT delete this row: it still records a live disagreement "+
			"between the runtimes that have not converged.\n"+
			"  UPDATE the %s column to what it now produces, instead of %q.",
		row.Where(), g.Runtime, strings.Join(names, ", "),
		strings.Join(live, ", "), g.Runtime, mine)
}

// sameExpectation reports whether two expectation CELLS mean the same
// thing.
//
// Compared by meaning, not by bytes. `1` and `1.0`, or two objects written
// with their keys in a different order, are the same expectation to the
// runner — so a row whose cells differ only that way records no divergence,
// and comparing raw strings would let it sit there passing in both ports
// forever while describing a disagreement that does not exist. That is the
// exact failure this whole mechanism exists to prevent, one level up.
func (g Register) sameExpectation(a, b string) bool {
	if a == b {
		return true
	}

	if IsErrorExpect(a) || IsErrorExpect(b) {
		if !IsErrorExpect(a) || !IsErrorExpect(b) {
			return false
		}
		ca, ea := ErrorCode(a)
		cb, eb := ErrorCode(b)
		return nil == ea && nil == eb && ca == cb
	}

	read := func(cell string) (any, error) {
		if nil != g.Runner.ParseExpected {
			return g.Runner.ParseExpected(cell, nil)
		}
		return ParseExpect(cell)
	}

	va, erra := read(a)
	vb, errb := read(b)
	if nil != erra || nil != errb {
		// A cell the fixture's own reader cannot parse is a defect the
		// runner will report with a better message when it runs the row.
		// Saying "these differ" here defers to that.
		return false
	}

	if nil != g.Runner.Normalize {
		return EqualValueWith(va, vb, g.Runner.Normalize)
	}
	return EqualValue(va, vb)
}

// NoDivergences declares that a repo records no divergences at all.
//
// An empty register is legitimate; an empty FILE is not, because CheckSpec
// cannot tell "no rows" from "the loader read nothing". Use this so the
// claim is executed rather than assumed from a file nobody notices is
// missing.
func NoDivergences(t *testing.T, where string) {
	t.Helper()

	t.Run("divergence register: "+strings.TrimSpace(where), func(t *testing.T) {
		// Deliberately nothing to assert. The value is that the suite names
		// the claim out loud, so "this repo has no divergences" appears in
		// the test output rather than being inferred from silence.
	})
}
