// Copyright (c) 2026 tabnas, MIT License

// Package adder holds the adder grammar, as a plugin.
//
// This is the integer-addition grammar from the tabnas/parser README
// (1+2+3 => 6), packaged so both runtimes can run it against the shared
// test/spec/adder/*.tsv fixtures. It is the smallest grammar that is still
// a real one — two rules, one custom token, a push and a repeat — which
// makes it the natural end-to-end check that github.com/tabnas/support/go
// and @tabnas/support do the same thing.
//
// ts/src/adder.ts is the same grammar, declared the same way.
//
// The grammar reads:
//
//	val = add            -- `val` holds the running total
//	add = NR [ PL add ]  -- each number adds to it; `+` repeats
//
// `val` opens by pushing `add` with the total at 0. `add` opens on a
// number and adds it to its parent's node, then closes on `+` by REPLACING
// itself with another `add` — a repeat at the same stack depth, so
// 1+2+... of any length runs in one frame and every iteration's parent is
// still `val`, where the total lands.
//
// This is a SEPARATE module from github.com/tabnas/support/go, and stays
// that way. The support module is a dependency of every tabnas repo, so it
// carries no dependencies of its own; the grammar that exercises it needs
// the parser, and keeping the two apart is what lets both facts hold.
package adder

import (
	"fmt"

	tabnas "github.com/tabnas/parser/go"
)

// Adder applies the adder grammar to an instance. It is a tabnas plugin:
//
//	tn := tabnas.Make()
//	tn.Use(adder.Adder)
//	tn.Parse("1+2+3")   // => 6
func Adder(tn *tabnas.Tabnas, _ map[string]any) error {
	return tn.Grammar(&tabnas.GrammarSpec{

		// Actions are named here and referenced as "@" strings below,
		// which is what keeps the spec itself a plain data structure.
		Ref: map[tabnas.FuncRef]any{

			// Start the running total at 0.
			"@init": tabnas.AltAction(func(r *tabnas.Rule, _ *tabnas.Context) {
				r.Node = float64(0)
			}),

			// Add this number to the total. r.Parent is the `val` rule;
			// r.O[0] is the number token matched at the first open position.
			"@add": tabnas.AltAction(func(r *tabnas.Rule, _ *tabnas.Context) {
				r.Parent.Node = number(r.Parent.Node) + number(r.O[0].Val)
			}),
		},

		OptionsMap: map[string]any{

			// A new fixed token named #PL: the "+" character.
			"fixed": map[string]any{"token": map[string]any{"#PL": "+"}},

			// Start parsing at the 'val' rule.
			"rule": map[string]any{"start": "val"},
		},

		// A Go map has no order of its own; this is the order the rules
		// were written in, which is what (*Tabnas).RuleNames then reports.
		RuleOrder: []string{"val", "add"},

		Rule: map[string]*tabnas.GrammarRuleSpec{

			// The 'val' rule holds the running total in its node.
			"val": {
				Open: []*tabnas.GrammarAltSpec{
					// Push down into 'add', with the total initialised to 0.
					{P: "add", A: "@init"},
				},
				Close: []*tabnas.GrammarAltSpec{
					{}, // Ending alternate — does nothing.
				},
			},

			// The 'add' rule performs the addition. #NR is the engine's
			// built-in number token.
			"add": {
				Open: []*tabnas.GrammarAltSpec{
					{S: []string{"#NR"}, A: "@add"},
				},
				Close: []*tabnas.GrammarAltSpec{
					// A following "+" repeats the rule; otherwise it ends.
					{S: []string{"#PL"}, R: "add"},
					{},
				},
			},
		},
	})
}

// Make returns a new instance with the adder grammar installed.
func Make() (*tabnas.Tabnas, error) {
	tn := tabnas.Make()
	if err := tn.Use(Adder); err != nil {
		return nil, err
	}
	return tn, nil
}

// number widens whatever numeric type the lexer produced for a #NR token
// to float64, so the total is one type throughout. The TypeScript runtime
// has one number type and needs no such step; this is the Go tax on the
// same grammar, and keeping it in one place is what stops it leaking into
// the grammar's shape.
func number(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case uint64:
		return float64(n)
	case nil:
		return 0
	}
	// Unreachable for a #NR token, and loud if the engine ever changes
	// what one carries — a silent 0 here would turn a broken lexer into a
	// wrong-but-plausible total.
	panic(fmt.Sprintf("adder: #NR token value is not numeric: %T (%v)", v, v))
}
