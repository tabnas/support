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


// The position of the first UNPAIRED `\uXXXX` surrogate escape in an
// expected cell, counted in CODE POINTS, or -1.
//
// Code points because this number crosses the two runtimes. The natural
// index here is a UTF-16 offset and in `go/expect.go` it is a BYTE
// offset, and those disagree the moment anything non-ASCII precedes the
// escape: for `"é\ud800"` they are 2 and 3. A helper whose whole purpose
// is to keep the two ports saying the same thing cannot report a number
// that depends on which port asked. A code-point count is the same in
// both by definition, and it is also what someone counting characters in
// a TSV cell would arrive at.
//
// WHY THIS IS NOT A CURIOSITY. The two runtimes decode such an escape
// differently, and neither is wrong: `JSON.parse` preserves it, because a
// JavaScript string is UTF-16 and may hold one; Go's `encoding/json`
// replaces it with U+FFFD, because a Go string is UTF-8 and cannot. That
// is `parser/DIVERGENCE.md`'s first entry, deliberate and permanent.
//
// Measured on the same cells:
//
//     cell             TypeScript            Go
//     "\ud800"         1 unit, d800          3 bytes, ef bf bd
//     "a\ud800b"       61 d800 62            61 ef bf bd 62
//     "\ud83d\ude00"  d83d de00             f0 9f 98 80    (a PAIR - agree)
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
export function loneSurrogateAt(cell: string): number {
  const hex4 = (at: number): number =>
    at + 4 <= cell.length && /^[0-9a-fA-F]{4}$/.test(cell.slice(at, at + 4))
      ? parseInt(cell.slice(at, at + 4), 16)
      : -1

  let i = 0
  while (i < cell.length) {
    if ('\\' !== cell[i]) {
      i++
      continue
    }

    // A run of backslashes escapes itself in pairs; only an ODD run
    // leaves a live escape, whose introducer is the character after the
    // whole run. `\\ud800` is a literal backslash then `ud800`.
    let j = i
    while (j < cell.length && '\\' === cell[j]) j++
    if (0 === (j - i) % 2) {
      i = j
      continue
    }

    const start = j - 1
    if ('u' !== cell[j]) {
      i = j + 1
      continue
    }

    const cp = hex4(j + 1)
    if (cp < 0) {
      i = j + 1
      continue
    }

    if (0xd800 <= cp && cp <= 0xdbff) {
      // A high surrogate is fine if a low follows IMMEDIATELY.
      const k = j + 5
      if ('\\' === cell[k] && 'u' === cell[k + 1]) {
        const lo = hex4(k + 2)
        if (0xdc00 <= lo && lo <= 0xdfff) {
          i = k + 6
          continue
        }
      }
      return codePointsBefore(cell, start)
    }
    if (0xdc00 <= cp && cp <= 0xdfff) {
      // A paired low was consumed above, so reaching one here means it
      // has no high before it.
      return codePointsBefore(cell, start)
    }

    i = j + 5
  }

  return -1
}


// Code points in `cell` before the UTF-16 index `at`. The prefix always
// ends on a backslash, so it never splits a surrogate pair.
function codePointsBefore(cell: string, at: number): number {
  let n = 0
  for (const _ of cell.slice(0, at)) n++
  return n
}


// The message the runner uses when a shared cell holds one. Exported so
// both runtimes say the same thing, and so a caller building its own
// runner can reuse it rather than inventing a vaguer one.
export function loneSurrogateMessage(cell: string, at: number): string {
  return (
    `expected cell holds an unpaired surrogate escape at code point ${at}: ` +
    `${JSON.stringify(cell)}\n` +
    '  A shared expected column CANNOT express this: JSON.parse preserves ' +
    'a lone surrogate\n' +
    '  (a JavaScript string is UTF-16) and Go\'s encoding/json replaces it ' +
    'with U+FFFD\n' +
    '  (a Go string is UTF-8). The two runtimes would be asked different ' +
    'questions and both\n' +
    '  would pass. This is a recorded, permanent divergence — see ' +
    'DIVERGENCE.md.\n' +
    '  Put the case in a per-runtime register column, where each decoding ' +
    'is written out,\n' +
    '  or in each port\'s own suite with opposite assertions. A surrogate ' +
    'PAIR is fine here.'
  )
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
