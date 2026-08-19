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
		if err := g.Runner.CheckSpec(spec); err != nil {
			t.Fatalf("%v", err)
		}

		// Every named runtime column must exist. Row.Named returns "" for
		// a column the file does not have, so a typo in Runtimes would
		// otherwise make every row look like agreement and be reported as
		// "this row records no divergence" — a message pointing at the
		// fixture when the fault is in the caller.
		for _, name := range g.Runtimes {
			if 0 > spec.Rows[0].IndexOf(name) {
				t.Fatalf("%s: no column named %q (Register.Runtimes)",
					spec.Name, name)
			}
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
		if cell != mine {
			agree = false
		}
	}

	// 1. Does this row record a divergence at all?
	if agree {
		return fmt.Errorf(
			"%s: every runtime column says %q, so this row records no "+
				"divergence and can never fail meaningfully. Delete it, or "+
				"correct the cells to what the runtimes actually do.",
			row.Where(), mine)
	}

	// 2. Does this runtime still do what the register says?
	mismatch := g.Runner.CheckRow(row, input, mine)
	if nil == mismatch {
		return nil
	}

	// 3. It does not. Does it now do what one of the OTHERS says? Then the
	//    divergence is closed, and reporting a regression would point the
	//    reader at exactly the wrong conclusion.
	//
	//    Reusing Runner.CheckRow for this keeps one comparison
	//    implementation: a register must not develop its own idea of what
	//    "equal" means.
	for _, o := range others {
		if nil != g.Runner.CheckRow(row, input, o.cell) {
			continue
		}
		return fmt.Errorf(
			"%s: this divergence is CLOSED. %s now produces what the %s "+
				"column records (%q), not its own (%q).\n"+
				"  This is the register working: a fixed divergence fails as "+
				"loudly as a regressed one, so the row cannot outlive it.\n"+
				"  DELETE this row. Do not edit it to match — that would "+
				"record a divergence that no longer exists, which is what "+
				"this mechanism exists to prevent.",
			row.Where(), g.Runtime, o.name, o.cell, mine)
	}

	// 4. Neither. An ordinary regression; the runner's own message says
	//    what was produced and what was expected.
	return mismatch
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
