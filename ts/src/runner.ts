/* Copyright (c) 2026 tabnas, MIT License */

/* runner.ts
 * The table-driven runner: fixture rows in, `node:test` cases out.
 *
 * Loading a fixture is only half of what every repo was duplicating. The
 * other half is the loop — read the input column, parse it, branch on
 * whether the expected column names a value or an error, compare, and
 * report with enough context to find the row. That loop is the same
 * everywhere, and `go/runner.go` runs the identical one against
 * `*testing.T`.
 */

import { describe, it } from 'node:test'

import {
  SpecFile, SpecRow, SpecOptions,
  loadSpec, loadSpecDir,
} from './spec'

import {
  isErrorExpect, errorCode, parseExpect, equalValue, formatValue,
  loneSurrogateAt, loneSurrogateMessage,
} from './expect'


export type RunnerOptions = {
  // Parse one input. Throws (or rejects the value some other way) for
  // input the grammar must not accept.
  parse: (input: string, row: SpecRow) => unknown

  // The error code carried by a thrown error. Only needed for fixtures
  // with `ERROR:<code>` rows; the default reads `err.code`, which is what
  // `TabnasError` exposes.
  errorCode?: (err: unknown, row: SpecRow) => string | undefined

  // Decide whether a thrown error satisfies an `ERROR:<want>` row, when
  // comparing a code cannot. It replaces the code comparison entirely, so
  // `errorCode` is not consulted for a runner that sets this.
  //
  // A code is the contract this package prefers — two runtimes that reject
  // the same input for different reasons have not agreed on anything — but
  // some grammars have no stable code to pin: a parser whose failures are
  // distinguished only by their message, or a fixture that names a
  // position (`ERROR:1:8`) rather than a kind. Those fixtures would
  // otherwise have to weaken to a bare `ERROR`, which asserts nothing more
  // than "it failed".
  //
  // A bare `ERROR` cell still means "any error", and does not reach here.
  matchError?: (err: unknown, want: string, row: SpecRow) => boolean

  // Read the expected cell, when the fixture's vocabulary is wider than
  // JSON. It replaces `parseExpect`, and is reached only for a value row
  // — an `ERROR` cell is an error expectation before it is anything else.
  //
  // JSON is what an expected cell should be wherever it can be, because
  // it is the one notation both runtimes already agree on. But some
  // grammars produce values JSON cannot spell: JSON5's `NaN` and
  // `Infinity`, and the `UNDEFINED` several repos use for "the parse
  // yielded no value at all", which is a different result from `null`.
  // Those fixtures would otherwise have to stop pinning the distinction
  // they exist to pin.
  //
  // Call `parseExpect` for the cells the hook does not claim, so the
  // ordinary rows keep the ordinary rules.
  parseExpected?: (expected: string, row: SpecRow) => unknown

  // Rewrite values before comparison — see `EqualOptions.normalize`.
  normalize?: (val: unknown) => unknown

  // Column holding the parser input, by position or header name.
  // Default: 0.
  input?: number | string

  // Column holding the expected result. Default: 1.
  expected?: number | string

  // Fixture-file loading options, passed through to the loader.
  spec?: SpecOptions

  // Name a test case. The default is `row <line>: <input>`, which keeps
  // the file's own line numbers in the test output.
  caseName?: (row: SpecRow, input: string) => string
}


// A runner bound to one parser. Reuse it across fixture files.
export class SpecRunner {
  private options: RunnerOptions

  constructor(options: RunnerOptions) {
    // Said plainly at construction, rather than as a "not a function"
    // thrown once per row, where the cause is much less obvious. The
    // types cover a TypeScript caller; the suites that use this are `.js`.
    if ('function' !== typeof options?.parse) {
      throw new Error('SpecRunner: options.parse is required')
    }
    this.options = options
  }

  // Throw if a fixture cannot be run. `spec` calls this before it
  // registers anything, so an unusable fixture fails loudly at once
  // rather than as one red case among the rest.
  //
  // A fixture that loads but holds no rows is the case that matters: it
  // is a silent pass, and a silent pass is indistinguishable from
  // coverage that was never there.
  checkSpec(spec: SpecFile): void {
    if (0 === spec.rows.length) {
      throw new Error(spec.file + ': no cases')
    }

    const probe = spec.rows[0]
    probe.resolve(this.options.input ?? 0)
    probe.resolve(this.options.expected ?? 1)
  }

  // Run every row of an already-loaded fixture, inside a `describe` block
  // named for the file.
  spec(spec: SpecFile): void {
    const opts = this.options

    this.checkSpec(spec)

    describe('spec: ' + spec.file, () => {
      for (const row of spec.rows) {
        const input = row.unesc(row.resolve(opts.input ?? 0))
        const expected = row.col(row.resolve(opts.expected ?? 1))
        const name = opts.caseName
          ? opts.caseName(row, input)
          : `row ${row.line}: ${JSON.stringify(input)}`

        it(name, () => this.row(row, input, expected))
      }
    })
  }

  // Run one row. Exposed so a suite with its own reporting can drive the
  // same comparison without `describe`/`it`.
  row(row: SpecRow, input: string, expected: string): void {
    const opts = this.options

    if (isErrorExpect(expected)) {
      const want = errorCode(expected)
      let threw: unknown = null
      let got: unknown = undefined
      let ok = false

      try {
        got = opts.parse(input, row)
      }
      catch (err) {
        threw = err
        ok = true
      }

      if (!ok) {
        throw new Error(
          `${row.where()}: parse(${JSON.stringify(input)}) should fail ` +
          `with ${expected}, but returned ${formatValue(got)}`)
      }

      if ('' !== want) {
        if (opts.matchError) {
          if (!opts.matchError(threw, want, row)) {
            throw new Error(
              `${row.where()}: parse(${JSON.stringify(input)}) failed, but ` +
              `the error does not match ${JSON.stringify(want)}` +
              `\n  error: ${threw}`)
          }
        }
        else {
          const code = opts.errorCode
            ? opts.errorCode(threw, row)
            : (threw as any)?.code
          if (code !== want) {
            throw new Error(
              `${row.where()}: parse(${JSON.stringify(input)}) failed with ` +
              `code ${formatValue(code)}, expected ${JSON.stringify(want)}` +
              `\n  error: ${threw}`)
          }
        }
      }

      return
    }

    // Refuse a shared cell that the two runtimes would decode
    // differently. `loneSurrogateAt` says why; the short version is that
    // this is the one thing a shared expected column cannot express, and
    // that it fails SILENTLY — both runtimes pass, having been asked
    // different questions. Audit item S2.
    //
    // Checked before `parseExpected` as well as before `parseExpect`: a
    // custom hook decodes the same cell text with the same JSON rules
    // and inherits the same asymmetry.
    const loneAt = loneSurrogateAt(expected)
    if (0 <= loneAt) {
      throw new Error(
        `${row.where()}: ${loneSurrogateMessage(expected, loneAt)}`)
    }

    const want = opts.parseExpected
      ? opts.parseExpected(expected, row)
      : parseExpect(expected)

    let got: unknown
    try {
      got = opts.parse(input, row)
    }
    catch (err) {
      // A value row that threw is a failure like any other, and it needs
      // the same `<file>:<line>` prefix — an unadorned parser error says
      // nothing about which fixture row provoked it.
      throw new Error(
        `${row.where()}: parse(${JSON.stringify(input)}) failed: ${err}`,
        { cause: err })
    }

    if (!equalValue(got, want, { normalize: opts.normalize })) {
      throw new Error(
        `${row.where()}: parse(${JSON.stringify(input)})` +
        `\n  got:      ${formatValue(got)}` +
        `\n  expected: ${formatValue(want)}`)
    }
  }

  // Load one fixture file by path and run it.
  file(path: string): void {
    this.spec(loadSpec(path, this.options.spec))
  }

  // Load and run every `*.tsv` in a directory. `loadSpecDir` rejects a
  // directory with no fixtures in it, so a run that would have been
  // green having done nothing throws here instead.
  dir(dir: string): void {
    for (const spec of loadSpecDir(dir, this.options.spec)) {
      this.spec(spec)
    }
  }
}


// Build a runner. `new SpecRunner(...)` does the same; this reads better
// at a call site that immediately runs one file.
export function makeRunner(options: RunnerOptions): SpecRunner {
  return new SpecRunner(options)
}
