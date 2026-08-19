/* Copyright (c) 2026 tabnas, MIT License */

/* register.ts
 * A divergence register: the places two ports of one grammar DISAGREE,
 * recorded in a fixture both ports execute.
 *
 * The audit this exists for found 29 recorded divergence claims
 * contradicted by execution, and one file that had been wrong in BOTH
 * directions at once. Prose does not hold. What did hold was the one
 * mechanism that ran: a register whose rows are executed, so a row that
 * stops being true fails the build.
 *
 * The property that makes it work is not that a regression fails. It is
 * that a FIX fails too. When a port is repaired to agree with the other,
 * the register still claims they differ, so the suite goes red and names
 * the row to delete. A register cannot quietly outlive the divergence it
 * records — which is precisely how the 29 prose claims survived.
 *
 * `go/register.go` mirrors all of this.
 */

import { describe, it } from 'node:test'

import type { SpecFile, SpecRow } from './spec'
import { loadSpec } from './spec'

import { SpecRunner, RunnerOptions } from './runner'


export type RegisterOptions = RunnerOptions & {
  // Which column holds THIS runtime's answer — 'ts' in the TypeScript
  // suite, 'go' in the Go one. The two suites run the same file and read
  // different columns of it.
  runtime: string

  // Every runtime column in the file, this one included. Named rather
  // than inferred from the header, because inferring would silently treat
  // a `note` or `issue` column as a runtime and then report that the
  // ports "agree" with a sentence.
  runtimes: string[]
}


// A fixture of recorded divergences, run by both ports.
//
// Each row gives an input and one cell per runtime, written in the same
// vocabulary as an ordinary fixture's `expected` — a JSON value, or
// `ERROR:<code>`. Every row is checked three ways:
//
//   1. The row must actually record a divergence. If every runtime cell
//      says the same thing, the row asserts nothing and would pass
//      forever; that is the shape of the claims this mechanism replaces.
//   2. This runtime must still produce what the register says it does.
//   3. When it does not, and it now produces what ANOTHER runtime's cell
//      says, the failure says the divergence is CLOSED and names the row
//      to delete — rather than reporting it as a regression, which is the
//      opposite conclusion.
export class DivergenceRegister extends SpecRunner {
  readonly register: RegisterOptions

  constructor(options: RegisterOptions) {
    if (!options.runtime) {
      throw new Error('DivergenceRegister: options.runtime is required')
    }
    if (!options.runtimes?.includes(options.runtime)) {
      throw new Error(
        'DivergenceRegister: options.runtimes must include ' +
        JSON.stringify(options.runtime) + ', got ' +
        JSON.stringify(options.runtimes ?? []))
    }
    if (options.runtimes.length < 2) {
      throw new Error(
        'DivergenceRegister: a divergence needs at least two runtimes, ' +
        'got ' + JSON.stringify(options.runtimes))
    }

    // The expectation column IS this runtime's column, so every
    // comparison rule the ordinary runner applies applies here unchanged.
    super({ ...options, expected: options.runtime })
    this.register = options
  }

  // As SpecRunner.checkSpec, plus: every named runtime column must exist.
  // A typo in `runtimes` would otherwise surface as a missing-column error
  // on some later row, or not at all.
  checkSpec(spec: SpecFile): void {
    super.checkSpec(spec)
    const probe = spec.rows[0]
    for (const name of this.register.runtimes) {
      probe.resolve(name)
    }
  }

  row(row: SpecRow, input: string, _expected: string): void {
    const mine = row.col(row.resolve(this.register.runtime))
    const others = this.register.runtimes
      .filter((name) => name !== this.register.runtime)
      .map((name) => ({ name, cell: row.col(row.resolve(name)) }))

    // 1. Does this row record a divergence at all?
    if (others.every((o) => o.cell === mine)) {
      throw new Error(
        `${row.where()}: every runtime column says ${JSON.stringify(mine)}, ` +
        'so this row records no divergence and can never fail meaningfully. ' +
        'Delete it, or correct the cells to what the runtimes actually do.')
    }

    // 2. Does this runtime still do what the register says?
    let mismatch: unknown
    try {
      super.row(row, input, mine)
      return
    }
    catch (err) {
      mismatch = err
    }

    // 3. It does not. Does it now do what one of the OTHERS says? Then the
    //    divergence is closed, and reporting a regression would point the
    //    reader at exactly the wrong conclusion.
    //
    //    Reusing super.row for this keeps one comparison implementation:
    //    a register must not develop its own idea of what "equal" means.
    for (const other of others) {
      try {
        super.row(row, input, other.cell)
      }
      catch {
        continue
      }
      throw new Error(
        `${row.where()}: this divergence is CLOSED. ` +
        `${this.register.runtime} now produces what the ${other.name} ` +
        `column records (${JSON.stringify(other.cell)}), not its own ` +
        `(${JSON.stringify(mine)}).\n` +
        '  This is the register working: a fixed divergence fails as ' +
        'loudly as a regressed one, so the row cannot outlive it.\n' +
        '  DELETE this row. Do not edit it to match — that would record a ' +
        'divergence that no longer exists, which is what this mechanism ' +
        'exists to prevent.')
    }

    // 4. Neither. An ordinary regression; the runner's own message says
    //    what was produced and what was expected.
    throw mismatch
  }

  // Load one register file by path and run it.
  file(path: string): void {
    this.spec(loadSpec(path, this.register.spec))
  }
}


// An EMPTY register is legitimate — a repo with no recorded divergences —
// but an empty FILE is not, because `checkSpec` cannot tell "no rows" from
// "the loader read nothing". Use this to declare that a repo has no
// divergences, so the claim is executed rather than assumed from a file
// nobody notices is missing.
export function noDivergences(where: string): void {
  describe('divergence register: ' + where, () => {
    it('records no divergences', () => {
      // Deliberately nothing to assert. The value is that the suite names
      // the claim out loud, so "this repo has no divergences" appears in
      // the test output rather than being inferred from silence.
    })
  })
}


// Build a register. `new DivergenceRegister(...)` does the same; this
// reads better at a call site that immediately runs one file.
export function makeRegister(options: RegisterOptions): DivergenceRegister {
  return new DivergenceRegister(options)
}
