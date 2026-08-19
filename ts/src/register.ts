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

import { isErrorExpect, errorCode, parseExpect, equalValue } from './expect'

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

  // Do two expectation CELLS mean the same thing?
  //
  // Compared by meaning, not by bytes. `1` and `1.0`, or two objects
  // written with their keys in a different order, are the same expectation
  // to the runner — so a row whose cells differ only that way records no
  // divergence, and comparing the raw strings would let it sit there
  // passing in both ports forever while describing a disagreement that
  // does not exist. That is the exact failure this whole mechanism exists
  // to prevent, one level up.
  private sameExpectation(a: string, b: string): boolean {
    if (a === b) {
      return true
    }

    if (isErrorExpect(a) || isErrorExpect(b)) {
      if (!isErrorExpect(a) || !isErrorExpect(b)) {
        return false
      }
      return errorCode(a) === errorCode(b)
    }

    const read = (cell: string, at: SpecRow) => this.register.parseExpected
      ? this.register.parseExpected(cell, at)
      : parseExpect(cell)

    try {
      return equalValue(
        read(a, this.probeRow!), read(b, this.probeRow!),
        { normalize: this.register.normalize })
    }
    catch {
      // A cell the fixture's own reader cannot parse is a defect the
      // runner will report with a better message when it runs the row.
      // Saying "these differ" here defers to that.
      return false
    }
  }

  // The row currently being checked, for `sameExpectation`'s reader hook,
  // which takes a row so a suite can key its vocabulary on one.
  private probeRow?: SpecRow

  // A runner whose `parse` answers from ONE evaluation, however many
  // times it is asked. Every comparison a row needs then goes through the
  // ordinary runner — same equality, same ERROR contract, same hooks —
  // while the grammar under test is still run exactly once.
  //
  // The outcome is replayed rather than cached-and-returned: a parse that
  // threw must throw the same error again, or an error row would compare
  // as a success on the second look.
  private rowRunner(): SpecRunner {
    let done = false
    let value: unknown
    let thrown: unknown

    return new SpecRunner({
      ...this.register,
      expected: this.register.runtime,
      parse: (src, at) => {
        if (!done) {
          try {
            value = this.register.parse(src, at)
          }
          catch (err) {
            thrown = err
          }
          done = true
        }
        if (undefined !== thrown) {
          throw thrown
        }
        return value
      },
    })
  }

  row(row: SpecRow, input: string, _expected: string): void {
    this.probeRow = row

    const cells = this.register.runtimes.map((name) => ({
      name, cell: row.col(row.resolve(name)),
    }))
    const mine = cells.find((c) => c.name === this.register.runtime)!.cell
    const others = cells.filter((c) => c.name !== this.register.runtime)

    // 1. Does this row record a divergence at all?
    if (others.every((o) => this.sameExpectation(o.cell, mine))) {
      throw new Error(
        `${row.where()}: every runtime column means ` +
        `${JSON.stringify(mine)}, so this row records no divergence and ` +
        'can never fail meaningfully. Delete it, or correct the cells to ' +
        'what the runtimes actually do.')
    }

    // ONE parse per row, whatever it ends up being compared against.
    //
    // Every comparison below goes through the runner, so the register
    // never develops its own idea of what "equal" means — but the runner
    // parses as part of comparing, and calling it three times would parse
    // three times. A parse hook that carries state, or a parser whose
    // state changes after an error, could then answer differently on the
    // second call and turn a genuine regression into a reported "closed
    // divergence" — the opposite conclusion.
    const runner = this.rowRunner()

    // 2. Does this runtime still do what the register says?
    let mismatch: unknown
    try {
      runner.row(row, input, mine)
      return
    }
    catch (err) {
      mismatch = err
    }

    // 3. It does not. Which of the OTHERS does it now agree with?
    const converged = others.filter((other) => {
      try {
        runner.row(row, input, other.cell)
        return true
      }
      catch {
        return false
      }
    })

    if (0 === converged.length) {
      // 4. None. An ordinary regression; the runner's own message says
      //    what was produced and what was expected.
      throw mismatch
    }

    const names = converged.map((c) => c.name).join(', ')

    // Converged with EVERY other runtime: the divergence is gone.
    if (converged.length === others.length) {
      throw new Error(
        `${row.where()}: this divergence is CLOSED. ` +
        `${this.register.runtime} now produces what the ${names} ` +
        `column${1 < converged.length ? 's' : ''} record` +
        `${1 < converged.length ? '' : 's'} ` +
        `(${JSON.stringify(converged[0].cell)}), not its own ` +
        `(${JSON.stringify(mine)}).\n` +
        '  This is the register working: a fixed divergence fails as ' +
        'loudly as a regressed one, so the row cannot outlive it.\n' +
        '  DELETE this row. Do not edit it to match — that would record a ' +
        'divergence that no longer exists, which is what this mechanism ' +
        'exists to prevent.')
    }

    // Converged with SOME. The row is still recording a live disagreement
    // between the runtimes that have not converged, so deleting it would
    // drop that coverage. Only this runtime's own column is stale.
    throw new Error(
      `${row.where()}: this divergence is PARTIALLY closed. ` +
      `${this.register.runtime} now agrees with ${names}, but not with ` +
      `${others.filter((o) => !converged.includes(o))
        .map((o) => o.name).join(', ')}.\n` +
      '  Do NOT delete this row: it still records a live disagreement ' +
      'between the runtimes that have not converged.\n' +
      `  UPDATE the ${this.register.runtime} column to what it now ` +
      `produces, instead of ${JSON.stringify(mine)}.`)
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
