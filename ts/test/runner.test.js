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

const {
  SpecRunner, makeRunner, parseSpec,
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

  it('names the row for a bad expected cell', () => {
    const bad = parseSpec('bad.tsv', 'input\texpected\na\t{oops')
    const runner = makeRunner({ parse: (s) => s })
    const err = check(runner, bad, 0)
    assert.match(err.message, /invalid expected JSON/)
  })
})
