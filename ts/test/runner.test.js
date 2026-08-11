/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* runner.test.js — the runner's own failure behaviour.
 *
 * The runner reports through `node:test`, so its passing path is exercised
 * by every other suite here simply by being green. What needs its own test
 * is the other path: that a wrong answer, a missing failure, a wrong error
 * code and an empty fixture each FAIL. A runner that quietly passes is the
 * one bug that hides every other one.
 */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Os = require('node:os')
const Path = require('node:path')

const {
  SpecRunner, makeRunner, parseSpec, parseExpect,
} = require('../dist/support.js')


// Run one row directly, bypassing describe/it, and return the error it
// raised (or null when it passed).
function check(runner, spec, rowIndex, inputCol = 0, expectedCol = 1) {
  const row = spec.rows[rowIndex]
  try {
    runner.row(row, row.unesc(inputCol), row.col(expectedCol))
    return null
  }
  catch (err) {
    return err
  }
}


describe('runner', () => {

  const spec = parseSpec('t.tsv', [
    'input\texpected',
    'a\t"A"',
    'b\tERROR:bad_b',
    'c\tERROR',
  ].join('\n'))

  it('passes a row whose value matches', () => {
    const runner = makeRunner({ parse: (s) => s.toUpperCase() })
    assert.equal(check(runner, spec, 0), null)
  })

  it('fails a row whose value does not match, quoting both sides', () => {
    const runner = makeRunner({ parse: () => 'WRONG' })
    const err = check(runner, spec, 0)
    assert.match(err.message, /t\.tsv:2/)
    assert.match(err.message, /got: *"WRONG"/)
    assert.match(err.message, /expected: *"A"/)
  })

  it('fails a value row that threw, rather than reporting a pass', () => {
    const runner = makeRunner({
      parse: () => { throw new Error('boom') },
    })
    const err = check(runner, spec, 0)
    // Located like every other failure: an unadorned parser error says
    // nothing about which fixture row provoked it.
    assert.match(err.message, /t\.tsv:2/)
    assert.match(err.message, /boom/)
    assert.equal(err.cause.message, 'boom')
  })

  it('passes an ERROR row that failed with the named code', () => {
    const runner = makeRunner({
      parse: () => { throw Object.assign(new Error('x'), { code: 'bad_b' }) },
    })
    assert.equal(check(runner, spec, 1), null)
  })

  it('fails an ERROR row that did not fail at all', () => {
    const runner = makeRunner({ parse: () => 'fine' })
    const err = check(runner, spec, 1)
    assert.match(err.message, /should fail with ERROR:bad_b/)
    assert.match(err.message, /returned "fine"/)
  })

  it('fails an ERROR row that failed with a different code', () => {
    // The point of the code being in the fixture: rejecting the input for
    // the wrong reason is not agreement.
    const runner = makeRunner({
      parse: () => { throw Object.assign(new Error('x'), { code: 'other' }) },
    })
    const err = check(runner, spec, 1)
    assert.match(err.message, /code "other", expected "bad_b"/)
  })

  it('accepts any code for a bare ERROR row', () => {
    const runner = makeRunner({
      parse: () => { throw Object.assign(new Error('x'), { code: 'any' }) },
    })
    assert.equal(check(runner, spec, 2), null)
  })

  it('reads the code through a custom errorCode hook', () => {
    const runner = makeRunner({
      parse: () => { throw new Error('bad_b') },
      errorCode: (err) => err.message,
    })
    assert.equal(check(runner, spec, 1), null)
  })

  it('matches an error through a matchError hook', () => {
    // For a grammar with no stable code to pin: the hook replaces the code
    // comparison, so `ERROR:bad_b` can be matched against the message.
    // Without it such a fixture would have to weaken to a bare `ERROR`,
    // which asserts nothing beyond "it failed".
    let seen = ''
    const runner = makeRunner({
      parse: () => { throw new Error('something bad_b happened') },
      // Deliberately a wrong answer: matchError replaces the code
      // comparison entirely, so this must not be read.
      errorCode: () => 'WRONG',
      matchError: (err, want, row) => {
        seen = row.where()
        return err.message.includes(want)
      },
    })

    assert.equal(check(runner, spec, 1), null)
    assert.equal(seen, 't.tsv:3')

    // A bare ERROR means "any error" and never reaches the hook.
    seen = ''
    assert.equal(check(runner, spec, 2), null)
    assert.equal(seen, '')
  })

  it('fails a row its matchError hook rejects', () => {
    // A hook that could only pass would turn every error fixture into a
    // silent one.
    const runner = makeRunner({
      parse: () => { throw new Error('other') },
      matchError: () => false,
    })

    const err = check(runner, spec, 1)
    assert.match(err.message, /t\.tsv:3/)
    assert.match(err.message, /does not match "bad_b"/)
    assert.match(err.message, /other/)
  })

  it('reads the expected cell through a parseExpected hook', () => {
    // For a fixture vocabulary wider than JSON. `UNDEFINED` is the
    // spelling several repos use for "the parse yielded no value at all",
    // which is a different result from `null` and which JSON cannot say.
    const wider = parseSpec('w.tsv', [
      'input\texpected',
      'a\tUNDEFINED',
      'b\t"B"',
    ].join('\n'))

    const runner = makeRunner({
      parse: (s) => 'a' === s ? undefined : s.toUpperCase(),
      parseExpected: (cell) =>
        'UNDEFINED' === cell ? undefined : parseExpect(cell),
    })

    assert.equal(check(runner, wider, 0), null)
    // The cells the hook does not claim keep the ordinary rules.
    assert.equal(check(runner, wider, 1), null)
  })

  it('fails when a parseExpected row does not match', () => {
    const wider = parseSpec('w.tsv', ['input\texpected', 'a\tUNDEFINED'].join('\n'))
    const runner = makeRunner({
      parse: () => null,   // null is NOT undefined
      parseExpected: (cell) =>
        'UNDEFINED' === cell ? undefined : parseExpect(cell),
    })

    const err = check(runner, wider, 0)
    assert.match(err.message, /w\.tsv:2/)
  })

  it('does not reach parseExpected for an ERROR row', () => {
    // An ERROR cell is an error expectation before it is anything else.
    let seen = false
    const runner = makeRunner({
      parse: () => { const e = new Error('nope'); e.code = 'bad_b'; throw e },
      parseExpected: (cell) => { seen = true; return parseExpect(cell) },
    })

    assert.equal(check(runner, spec, 1), null)
    assert.equal(seen, false)
  })

  it('hands the row to the parse hook', () => {
    // A fixture whose other columns take part in the parse — an `opts`
    // column of plugin options is the common one — needs more than the
    // input string.
    const opted = parseSpec('o.tsv', [
      'input\texpected\topts',
      'a\t"A"\tupper',
      'b\t"b"\t',
    ].join('\n'))
    const runner = makeRunner({
      parse: (s, row) => 'upper' === row.named('opts') ? s.toUpperCase() : s,
    })

    assert.equal(check(runner, opted, 0), null)
    assert.equal(check(runner, opted, 1), null)
  })

  it('compares through a normalize hook', () => {
    const runner = makeRunner({
      parse: (s) => ({ boxed: s.toUpperCase() }),
      normalize: (v) => (v && v.boxed !== undefined ? v.boxed : v),
    })
    assert.equal(check(runner, spec, 0), null)
  })

  it('selects columns by header name', () => {
    const named = parseSpec('n.tsv', [
      'expected\tinput',
      '"A"\ta',
    ].join('\n'))
    const runner = makeRunner({
      parse: (s) => s.toUpperCase(),
      input: 'input',
      expected: 'expected',
    })
    const row = named.rows[0]
    assert.doesNotThrow(
      () => runner.row(row, row.unesc(row.resolve('input')),
        row.col(row.resolve('expected'))))
  })

  it('decodes escapes in the input column', () => {
    const esc = parseSpec('e.tsv', ['input\texpected', 'a\\tb\t2'].join('\n'))
    const runner = makeRunner({ parse: (s) => s.split('\t').length })
    assert.equal(check(runner, esc, 0), null)
  })

  it('is constructible directly as well as through makeRunner', () => {
    const runner = new SpecRunner({ parse: (s) => s.toUpperCase() })
    assert.equal(check(runner, spec, 0), null)
  })

  it('names a missing parse function at construction', () => {
    assert.throws(() => makeRunner({}), /options.parse is required/)
    assert.throws(() => makeRunner(), /options.parse is required/)
  })

  it('refuses a fixture with no cases', () => {
    // The silent-pass path. A fixture that loads but holds nothing runs
    // no assertions, and a runner that reported green over it would hide
    // every other failure this suite exists to catch.
    const runner = makeRunner({ parse: (s) => s })
    const empty = parseSpec('empty.tsv', 'input\texpected\n')

    assert.equal(empty.rows.length, 0)
    assert.throws(() => runner.checkSpec(empty), /empty\.tsv: no cases/)
    assert.throws(() => runner.spec(empty), /empty\.tsv: no cases/)
  })

  it('refuses a fixture whose named column is not there', () => {
    // Registration-time, not one red case per row: a misspelt column
    // name is a defect in the suite, not in the fixture.
    const runner = makeRunner({ parse: (s) => s, input: 'nosuch' })
    assert.throws(
      () => runner.spec(parseSpec('n.tsv', 'input\texpected\na\t"A"')),
      /no column named 'nosuch'/)
  })

  it('refuses a directory with no fixtures in it', () => {
    const dir = Fs.mkdtempSync(Path.join(Os.tmpdir(), 'tabnas-support-'))
    try {
      Fs.writeFileSync(Path.join(dir, 'notes.md'), 'not a fixture')
      const runner = makeRunner({ parse: (s) => s })
      assert.throws(() => runner.dir(dir), /no \.tsv fixtures/)
    }
    finally {
      Fs.rmSync(dir, { recursive: true, force: true })
    }
  })

  it('names the row for a bad expected cell', () => {
    const bad = parseSpec('bad.tsv', 'input\texpected\na\t{oops')
    const runner = makeRunner({ parse: (s) => s })
    const err = check(runner, bad, 0)
    assert.match(err.message, /invalid expected JSON/)
  })
})
