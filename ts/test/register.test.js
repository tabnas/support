/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* register.test.js — the register's own failure behaviour.
 *
 * What needs its own test is not that a regression fails; the runner
 * underneath already does that. It is the property a plain fixture does
 * NOT have: that a FIXED divergence fails too, and says so in words that
 * send the reader to delete the row rather than to restore the old
 * behaviour.
 *
 * `go/register_test.go` mirrors every case here.
 */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Path = require('node:path')

const {
  findSpecDir, parseSpec, makeRegister, DivergenceRegister,
} = require('../dist/support.js')


const SPEC = findSpecDir(__dirname)

const fixture = () => parseSpec('d.tsv', [
  'input\tts\tgo',
  'a\t"A"\t"a"',
  'b\tERROR:bad_b\t"b"',
].join('\n'))

// The TS column's recorded behaviour: uppercase.
const tsPort = (s) => s.toUpperCase()

const tsRegister = (parse) => makeRegister({
  parse,
  runtime: 'ts',
  runtimes: ['ts', 'go'],
})

// Run one row directly and return the error it raised (or null).
function checkReg(reg, spec, i) {
  const row = spec.rows[i]
  try {
    reg.row(row, row.unesc(row.resolve('input')), '')
    return null
  }
  catch (err) {
    return err
  }
}


describe('register', () => {

  it('passes while the divergence holds', () => {
    // Row 0's ts cell is a value, row 1's is ERROR:bad_b, so the port has
    // to answer each row the way the register records.
    const bad = () => {
      throw Object.assign(new Error('bad_b'), { code: 'bad_b' })
    }
    for (const [i, parse] of [[0, tsPort], [1, bad]]) {
      assert.equal(checkReg(tsRegister(parse), fixture(), i), null,
        `row ${i}`)
    }
  })

  // The case this whole mechanism exists for.
  it('fails when the divergence is CLOSED', () => {
    // TypeScript repaired to agree with Go: it now leaves the case alone,
    // which is what the go column records.
    const err = checkReg(tsRegister((s) => s), fixture(), 0)

    assert.ok(err, 'expected a failure: the ports now agree')
    assert.match(err.message, /CLOSED/)
    assert.match(err.message, /DELETE this row/)
    // And it must NOT read as a regression, which is the opposite
    // conclusion and the one a plain fixture would report.
    assert.doesNotMatch(err.message, /expected:/)
  })

  it('fails when an error row closes', () => {
    // ts row 1 records ERROR:bad_b and go records "b". If TypeScript stops
    // failing and returns "b", the two agree.
    const err = checkReg(tsRegister((s) => s), fixture(), 1)
    assert.ok(err, 'expected a failure: ts now returns what go returns')
    assert.match(err.message, /CLOSED/)
  })

  it('fails an ordinary regression as a mismatch, not as closed', () => {
    const err = checkReg(tsRegister(() => 'something else'), fixture(), 0)
    assert.ok(err, 'expected a failure')
    assert.doesNotMatch(err.message, /CLOSED/,
      'a regression must not be reported as a closed divergence')
  })

  it('rejects a row that records no divergence', () => {
    // Both columns say the same thing, so the row asserts nothing and
    // would pass forever. That is the shape of the prose claims this
    // mechanism replaces, so it is a failure, not a pass.
    const spec = parseSpec('d.tsv', [
      'input\tts\tgo',
      'a\t"A"\t"A"',
    ].join('\n'))

    const err = checkReg(tsRegister(tsPort), spec, 0)
    assert.ok(err, 'expected a failure: the row records no divergence')
    assert.match(err.message, /records no divergence/)
  })

  // --- Review findings on the first cut of this file ---

  it('compares cells by MEANING, not by bytes', () => {
    // `1` and `1.0` are the same expectation to the runner, so a row whose
    // columns differ only that way records no divergence. Comparing raw
    // strings let it sit there passing in both ports forever while
    // describing a disagreement that does not exist -- the exact failure
    // this mechanism exists to prevent, one level up.
    const cases = [
      ['same number, different notation', '1', '1.0', /records no divergence/],
      ['same object, different key order',
        '{"a":1,"b":2}', '{"b":2,"a":1}', /records no divergence/],
      ['same error code', 'ERROR:bad', 'ERROR:bad', /records no divergence/],
      ['genuinely different numbers', '1', '2', null],
      ['value against error', '1', 'ERROR:bad', null],
    ]

    for (const [name, tsCell, goCell, want] of cases) {
      const spec = parseSpec('d.tsv', [
        'input\tts\tgo',
        `a\t${tsCell}\t${goCell}`,
      ].join('\n'))

      // A parser answering whatever the TS column records, so only the
      // divergence check can fail the row.
      const reg = tsRegister(() => {
        if (tsCell.startsWith('ERROR')) {
          throw Object.assign(new Error('bad'), { code: 'bad' })
        }
        return JSON.parse(tsCell)
      })

      const err = checkReg(reg, spec, 0)
      if (null === want) {
        assert.equal(err, null, `${name}: expected a pass`)
      }
      else {
        assert.ok(err, `${name}: expected a failure`)
        assert.match(err.message, want, name)
      }
    }
  })

  it('keeps the row when only SOME runtimes converge', () => {
    // With three runtimes, agreeing with one does not close the
    // divergence -- the others still disagree, so deleting the row would
    // drop live coverage.
    const spec = parseSpec('d.tsv', [
      'input\tts\tgo\trust',
      'a\t"A"\t"a"\t"aa"',
    ].join('\n'))

    // TypeScript repaired to agree with go. rust still differs from both.
    const reg = makeRegister({
      parse: () => 'a',
      runtime: 'ts',
      runtimes: ['ts', 'go', 'rust'],
    })

    const err = checkReg(reg, spec, 0)
    assert.ok(err, 'expected a failure: ts no longer produces its own cell')
    assert.match(err.message, /PARTIALLY closed/)
    assert.match(err.message, /Do NOT delete this row/)
    assert.doesNotMatch(err.message, /DELETE this row/,
      'must not tell the reader to delete a row that still records a ' +
      'live disagreement')
  })

  it('closes only when ALL runtimes converge', () => {
    const spec = parseSpec('d.tsv', [
      'input\tts\tgo\trust',
      'a\t"A"\t"a"\t"a"',
    ].join('\n'))

    const reg = makeRegister({
      parse: () => 'a',
      runtime: 'ts',
      runtimes: ['ts', 'go', 'rust'],
    })

    const err = checkReg(reg, spec, 0)
    assert.ok(err, 'expected a failure')
    assert.match(err.message, /CLOSED/)
  })

  it('parses ONCE per row', () => {
    // A hook that answers differently on a later call would otherwise
    // turn a regression into a reported "closed divergence".
    let calls = 0
    const reg = tsRegister(() => {
      calls++
      return 1 < calls ? 'a' : 'something else'
    })

    const err = checkReg(reg, fixture(), 0)
    assert.ok(err, 'expected a failure')
    assert.equal(calls, 1, 'parse must run once per row')
    assert.doesNotMatch(err.message, /CLOSED/,
      'a regression was reported as closed because the parser was re-run')
  })

  it('rejects bad configuration', () => {
    assert.throws(
      () => new DivergenceRegister({ parse: tsPort, runtimes: ['ts', 'go'] }),
      /runtime is required/)
    assert.throws(
      () => new DivergenceRegister({
        parse: tsPort, runtime: 'rust', runtimes: ['ts', 'go'],
      }),
      /must include/)
    assert.throws(
      () => new DivergenceRegister({
        parse: tsPort, runtime: 'ts', runtimes: ['ts'],
      }),
      /at least two runtimes/)
  })
})


// The shared fixture, run the way a real repo would run it.
makeRegister({
  parse: (src) => {
    if ('a' === src) return 'A'
    if ('b' === src) {
      throw Object.assign(new Error('bad_b'), { code: 'bad_b' })
    }
    if ('c' === src) return 'c'
    throw new Error('unexpected input ' + src)
  },
  runtime: 'ts',
  runtimes: ['ts', 'go'],
}).file(Path.join(SPEC, 'register', 'divergent.tsv'))
