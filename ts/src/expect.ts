/* Copyright (c) 2026 tabnas, MIT License */

/* expect.ts
 * Reading the `expected` column, and comparing a parse result against it.
 *
 * A fixture's expected cell is one of two things: a JSON value the parse
 * must produce, or an error the parse must raise, written `ERROR:<code>`.
 * The code is part of the contract — "it threw" is not enough, since two
 * runtimes that reject the same input for different reasons have not
 * actually agreed on anything.
 *
 * `go/expect.go` mirrors all of this.
 */


// The prefix marking an expected-failure cell.
export const ERROR_PREFIX = 'ERROR'


// Is this expected cell an error expectation?
//
// Exactly `ERROR`, or `ERROR:` followed by a code. Note the colon: a bare
// prefix test would read a legitimate `ERRORS` or `ERROR_LIST` expected
// value as a failure expectation and then never check the parse result at
// all — a fixture row that silently tests nothing.
export function isErrorExpect(expected: string): boolean {
  return ERROR_PREFIX === expected || expected.startsWith(ERROR_PREFIX + ':')
}


// The code from an error expectation: `ERROR:unexpected` gives
// `unexpected`, and a bare `ERROR` gives '' (meaning "any code").
// Throws when handed a cell that is not an error expectation at all.
export function errorCode(expected: string): string {
  if (!isErrorExpect(expected)) {
    throw new Error('not an error expectation: ' + JSON.stringify(expected))
  }
  return ERROR_PREFIX === expected
    ? '' : expected.slice(ERROR_PREFIX.length + 1)
}


// Parse an expected cell as JSON. An empty cell is `undefined` — the
// fixture convention for "no value", as in a utility whose result is
// nothing at all.
//
// The cell is NOT escape-decoded first: it is JSON, and JSON has its own
// escape rules. Decoding it here would turn the two characters `\n` inside
// a JSON string into a real newline, which is not valid JSON.
export function parseExpect(expected: string): unknown {
  if ('' === expected) {
    return undefined
  }
  try {
    return JSON.parse(expected)
  }
  catch (err: any) {
    throw new Error(
      `invalid expected JSON: ${JSON.stringify(expected)}: ${err.message}`)
  }
}


// How to compare a parse result against an expected value.
export type EqualOptions = {
  // Rewrite a value before it is compared. Applied to every node on both
  // sides, outermost first. This is where a runtime-specific container —
  // an insertion-ordered map, a reference wrapper — is unwrapped into the
  // plain value the fixture's JSON describes.
  normalize?: (val: unknown) => unknown
}


// Compare two values with JSON semantics: structural, key-order
// independent, `NaN` equal to itself (which `===` is not, and which a
// fixture cannot express in JSON but an in-language case can), and `-0`
// NOT equal to `0`.
//
// Those last two are the two halves of ADR-15, and they go opposite ways
// on purpose.
//
// Map key order is OUT of the parsed-value contract. TypeScript cannot
// preserve integer-like key order in a plain object — that is ECMAScript's
// own property-ordering rule, not a porting choice — so making order
// contractual would force this port to return an order-preserving
// container, a breaking change for every consumer, to pin a property no
// format in the fleet defines as significant.
//
// Signed zero is IN it. `-0` is representable and distinguishable in both
// runtimes, and a parser that reports `0` for the input `-0` has lost
// information the source carried.
export function equalValue(
  got: unknown, expected: unknown, options?: EqualOptions,
): boolean {
  const norm = options?.normalize
  return deepEqual(got, expected, norm)
}


function deepEqual(
  a: unknown, b: unknown, norm?: (val: unknown) => unknown,
): boolean {
  if (norm) {
    a = norm(a)
    b = norm(b)
  }

  // Numbers first, BEFORE the `===` shortcut, because `0 === -0` is true
  // and signed zero is part of the value contract (ADR-15): a parser that
  // reports `0` for the input `-0` has lost information the source carried.
  // `Object.is` separates them, and treats NaN as equal to itself, which is
  // the other place `===` gives the wrong answer for a fixture.
  if ('number' === typeof a || 'number' === typeof b) {
    return 'number' === typeof a && 'number' === typeof b && Object.is(a, b)
  }

  if (a === b) {
    return true // Covers the remaining primitives.
  }

  if (null == a || null == b) {
    return false // One side is null/undefined and they were not ===.
  }

  if (Array.isArray(a) || Array.isArray(b)) {
    if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) {
      return false
    }
    for (let i = 0; i < a.length; i++) {
      if (!deepEqual(a[i], b[i], norm)) {
        return false
      }
    }
    return true
  }

  if ('object' === typeof a && 'object' === typeof b) {
    const am = a as Record<string, unknown>
    const bm = b as Record<string, unknown>
    const ak = Object.keys(am)
    const bk = Object.keys(bm)
    if (ak.length !== bk.length) {
      return false
    }
    for (const k of ak) {
      // `hasOwnProperty`, not `in`: `in` walks the prototype chain, so a
      // result keyed `constructor` or `valueOf` would match ANY object —
      // `{constructor: Object}` compared equal to `{x: 1}`. Relaxed
      // grammars parse those keys perfectly happily (that is what the
      // `funky-keys` fixtures are for), so this is reachable, and a false
      // PASS is the worst kind.
      //
      // Own, not merely present: an explicit `undefined` value is still
      // not the same as an absent key.
      if (!Object.prototype.hasOwnProperty.call(bm, k) ||
        !deepEqual(am[k], bm[k], norm)) {
        return false
      }
    }
    return true
  }

  return false
}


// Render a value for a failure message. JSON where possible, so the text
// lines up with how the fixture wrote it; a readable fallback otherwise
// (a cyclic structure, a BigInt, a function).
export function formatValue(val: unknown): string {
  if (undefined === val) {
    return 'undefined'
  }
  // JSON.stringify renders -0 as "0", so a signed-zero mismatch would
  // report "got 0, expected 0" — a failure message that reads as a bug in
  // the runner. Since ADR-15 made the two distinguishable, the formatter
  // has to be able to spell the difference.
  if (Object.is(val, -0)) {
    return '-0'
  }
  try {
    const out = JSON.stringify(val)
    return undefined === out ? String(val) : out
  }
  catch {
    return String(val)
  }
}
