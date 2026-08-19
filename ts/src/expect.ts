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
// A trailing `@<row>:<col>` is a position expectation and is NOT part of
// the code — see `errorExpect`.
// Throws when handed a cell that is not an error expectation at all.
export function errorCode(expected: string): string {
  return errorExpect(expected).code
}


// An error expectation, split into the parts a runner checks separately.
export type ErrorExpect = {
  // The code, or '' for "any code" (a bare `ERROR`, or a cell that pins
  // only a position).
  code: string

  // 1-based source position the error must report, when the cell pins one.
  // Both undefined otherwise.
  row?: number
  col?: number
}


// Trailing `@<row>:<col>`, anchored at the end and digits-only.
//
// Zero is matched here and REJECTED below rather than excluded by the
// pattern, so `@0:0` fails as a malformed fixture instead of quietly
// falling through and being read as part of the code.
//
// `@` rather than another colon because a code is not always a bare
// identifier: the fleet's fixtures already carry `ERROR:a:b` and whole
// diagnostic sentences with embedded colons, so `ERROR:x:1:8` could not be
// split without guessing. Anchoring at the end and requiring digits keeps
// a message that merely CONTAINS an `@` intact.
const POSITION_SUFFIX = /@(\d+):(\d+)$/


// Read an error expectation.
//
//   ERROR                     any error
//   ERROR:unexpected          that code, position unchecked
//   ERROR:unexpected@1:8      that code, reported at row 1 col 8
//   ERROR:@1:8                any code, reported at row 1 col 8
//
// The position channel exists because a code alone does not pin a
// diagnostic. Two runtimes can agree on `unexpected` and disagree on where
// they say it happened — which is exactly what the fleet audit found, in
// several repos at once, with every code row green. A fixture that pins
// the position makes that disagreement a failing row instead of a
// difference nobody is looking at.
//
// Throws when handed a cell that is not an error expectation at all.
export function errorExpect(expected: string): ErrorExpect {
  if (!isErrorExpect(expected)) {
    throw new Error('not an error expectation: ' + JSON.stringify(expected))
  }

  let code = ERROR_PREFIX === expected
    ? '' : expected.slice(ERROR_PREFIX.length + 1)

  const pos = code.match(POSITION_SUFFIX)
  if (!pos) {
    return { code }
  }

  const row = parseInt(pos[1], 10)
  const col = parseInt(pos[2], 10)

  // Positions are 1-based, so zero is not a position. It matters more than
  // it looks: an error type that leaves `row`/`col` at their zero value
  // when it has no position would MATCH `@0:0`, and the row would pass
  // while pinning no source location at all — the exact silent gap this
  // channel exists to close, reintroduced through its own syntax.
  if (row < 1 || col < 1) {
    throw new Error(
      'position in an error expectation is 1-based, so 0 is not a ' +
      'position: ' + JSON.stringify(expected))
  }

  return { code: code.slice(0, pos.index), row, col }
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
// independent, `-0` equal to `0`, and `NaN` equal to itself (which `===`
// is not, and which a fixture cannot express in JSON but an in-language
// case can).
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

  if (a === b) {
    return true // Covers primitives, and 0 === -0.
  }

  if ('number' === typeof a && 'number' === typeof b) {
    return Number.isNaN(a) && Number.isNaN(b)
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
  try {
    const out = JSON.stringify(val)
    return undefined === out ? String(val) : out
  }
  catch {
    return String(val)
  }
}
