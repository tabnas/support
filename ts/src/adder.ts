/* Copyright (c) 2026 tabnas, MIT License */

/* adder.ts
 * The adder grammar, as a plugin.
 *
 * This is the integer-addition grammar from the @tabnas/parser README
 * (`1+2+3` => 6), packaged so both runtimes can run it against the shared
 * `test/spec/adder-*.tsv` fixtures. It is the smallest grammar that is
 * still a real one — two rules, one custom token, a push and a repeat —
 * which makes it the natural end-to-end check that this package's loader,
 * escape codec, expectation parsing and runner do the same thing in
 * TypeScript and in Go.
 *
 * `go/adder/adder.go` is the same grammar, declared the same way.
 *
 * The grammar reads:
 *
 *   val = add            -- `val` holds the running total
 *   add = NR [ PL add ]  -- each number adds to it; `+` repeats
 *
 * `val` opens by pushing `add` with the total at 0. `add` opens on a
 * number and adds it to its parent's node, then closes on `+` by
 * REPLACING itself with another `add` — a repeat at the same stack depth,
 * so `1+2+...` of any length runs in one frame and every iteration's
 * parent is still `val`, where the total lands.
 */

import type { Tabnas, Rule } from '@tabnas/parser'


// Apply the adder grammar to an instance. Use it as a plugin:
//
//   const tn = new Tabnas({ plugins: [adder] })
//   tn.parse('1+2+3')   // => 6
//
export function adder(tn: Tabnas): void {
  tn.grammar({

    // Actions are named here and referenced as `@`-strings below. The
    // README writes them inline, which the engine also accepts; naming
    // them keeps the spec itself free of functions — the point of the
    // declarative form — and makes this grammar the same shape as its Go
    // twin, which has no other option.
    ref: {

      // Start the running total at 0.
      '@init': (r: Rule) => { r.node = 0 },

      // Add this number to the total. r.parent is the 'val' rule; r.o[0]
      // is the number token matched at the first open position.
      '@add': (r: Rule) => { r.parent.node += r.o[0].val },
    },

    options: {

      // A new fixed token named #PL: the "+" character.
      fixed: { token: { '#PL': '+' } },

      // Start parsing at the 'val' rule.
      rule: { start: 'val' },
    },

    rule: {

      // The 'val' rule holds the running total in its node.
      val: {
        open: [
          // Push down into 'add', with the total initialised to 0.
          { p: 'add', a: '@init' },
        ],
        close: [
          {}, // Ending alternate — does nothing.
        ],
      },

      // The 'add' rule performs the addition. #NR is the engine's
      // built-in number token.
      add: {
        open: [
          { s: '#NR', a: '@add' },
        ],
        close: [
          // A following "+" repeats the rule; otherwise the rule ends.
          { s: '#PL', r: 'add' },
          {},
        ],
      },
    },
  })
}
