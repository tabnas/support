/* Copyright (c) 2026 tabnas, MIT License */

/* census.ts
 * Coverage and parity tripwires over error codes.
 *
 * Every tabnas package declares a set of error codes, renders each
 * through a `{code: template}` message catalogue, and pins its
 * rejections in fixtures as `ERROR:<code>` rows. Three lists, three ways
 * to drift: a declared code no fixture ever exercises, two catalogues
 * whose keys or templates have come apart, a fixture pinning a code
 * nobody declares. These helpers compute those gaps from real loaded
 * data.
 *
 * Every input arrives as an argument. Nothing here fetches a catalogue
 * or imports the engine — this package is a dependency of every tabnas
 * repo, so anything it required would land in all of them — and that is
 * exactly why these tripwires can live in the one module the repos
 * already share.
 *
 * `go/census.go` mirrors all of this.
 */

import { SpecRow, loadSpecDir } from './spec'

import { isErrorExpect, errorCode } from './expect'


// What a code-style expectation names: a bare lowercase token, which is
// how every tabnas error code is spelt. `ERROR:bad token` and
// `ERROR:1:8` are message- and position-style expectations — real
// rejections, but not codes, and a census that returned them would
// count coverage that is not there.
const CODE_TOKEN = /^[a-z][a-z0-9_]*$/


// Which column holds the expectation. The default is each row's last
// column — the ordinary two-column `input`/`expected` fixture needs no
// selection at all — but a wider fixture can keep the expectation
// anywhere, which is why the selection exists.
export type CensusOptions = {
  col?: number   // Expectation column by position.
  name?: string  // Expectation column by header name; wins when set.
}


// Collect every code named by a code-style expectation cell under a
// fixture directory: sorted, unique. Message-style expectations and
// bare `ERROR` cells assert a rejection without naming a code, so they
// are not collected; a value row is not an expectation at all.
//
// The directory is read with the shared loader, so an empty directory
// fails here the way it fails everywhere else — a census over nothing
// must not report "no codes" as if it had looked.
export function codesInSpecDir(dir: string, options?: CensusOptions): string[] {
  const opts = options ?? {}
  const codes = new Set<string>()

  for (const spec of loadSpecDir(dir)) {
    for (const row of spec.rows) {
      const cell = row.col(expectationCol(row, opts))

      if (!isErrorExpect(cell)) {
        continue
      }

      const code = errorCode(cell)
      if (CODE_TOKEN.test(code)) {
        codes.add(code)
      }
    }
  }

  return [...codes].sort(codePointCompare)
}


// Resolve the expectation column for one row: the named column when
// `name` is set, else the given position, else the row's own last
// column. An unknown name throws (via `resolve`) — that is a defect in
// the caller, and a census silently reading the wrong column would
// report coverage nobody has.
function expectationCol(row: SpecRow, opts: CensusOptions): number {
  if (undefined !== opts.name) {
    return row.resolve(opts.name)
  }
  if (undefined !== opts.col) {
    return opts.col
  }
  return row.cols.length - 1
}


// What compareCatalogues found. All three lists are sorted.
export type CatalogueDiff = {
  missing: string[]           // Keys of `a` absent in `b`.
  extra: string[]             // Keys of `b` absent in `a`.
  templateMismatch: string[]  // Shared keys whose templates differ.
}


// Diff two `{code: template}` catalogues — message catalogues, hint
// catalogues, or one runtime's against the other's. Templates compare
// byte for byte: two templates that merely "mean the same" have still
// drifted, and the byte diff is what a maintainer has to reconcile.
export function compareCatalogues(
  a: Record<string, string>, b: Record<string, string>,
): CatalogueDiff {
  const missing: string[] = []
  const extra: string[] = []
  const templateMismatch: string[] = []

  for (const code of Object.keys(a)) {
    // `hasOwnProperty`, not `in` or indexing: a catalogue legitimately
    // keyed `constructor` or `toString` would otherwise appear present
    // in every catalogue — the same trap `equalValue` guards against.
    if (!Object.prototype.hasOwnProperty.call(b, code)) {
      missing.push(code)
    }
    else if (a[code] !== b[code]) {
      templateMismatch.push(code)
    }
  }

  for (const code of Object.keys(b)) {
    if (!Object.prototype.hasOwnProperty.call(a, code)) {
      extra.push(code)
    }
  }

  return {
    missing: missing.sort(codePointCompare),
    extra: extra.sort(codePointCompare),
    templateMismatch: templateMismatch.sort(codePointCompare),
  }
}


// What coverage found. Both lists are sorted.
export type CoverageReport = {
  uncovered: string[]  // Declared, but exercised by no fixture.
  orphan: string[]     // Exercised, but declared by nobody.
}


// Compare the codes a package declares against the codes its fixtures
// exercise (typically `codesInSpecDir`'s answer). Whether inherited
// base codes count as declared is the caller's choice — pass them in or
// leave them out.
//
// An uncovered code is a rejection nobody has pinned; an orphan is a
// fixture pinning a code the package does not declare — a misspelt
// code, or one that has since been removed. Both lists empty is what
// "the fixtures and the declarations agree" means.
export function coverage(
  declared: string[], exercised: string[],
): CoverageReport {
  const dset = new Set(declared)
  const eset = new Set(exercised)

  const uncovered =
    [...dset].filter((code) => !eset.has(code)).sort(codePointCompare)
  const orphan =
    [...eset].filter((code) => !dset.has(code)).sort(codePointCompare)

  return { uncovered, orphan }
}


// Order two strings by Unicode CODE POINT. The default Array sort
// compares UTF-16 code units, which puts a non-BMP string — a surrogate
// pair, both units below 0xE000 — ahead of a BMP character above it,
// while Go's sort.Strings compares UTF-8 bytes, which for valid UTF-8
// IS code-point order. A tabnas error code can never contain either,
// but a catalogue key is whatever the caller's map holds, and a census
// whose two runtimes order the same answer differently would fail the
// very parity it exists to report on.
function codePointCompare(a: string, b: string): number {
  const acp = Array.from(a)
  const bcp = Array.from(b)
  const len = Math.min(acp.length, bcp.length)

  for (let i = 0; i < len; i++) {
    const d = (acp[i].codePointAt(0) ?? 0) - (bcp[i].codePointAt(0) ?? 0)
    if (0 !== d) {
      return d
    }
  }

  return acp.length - bcp.length
}
