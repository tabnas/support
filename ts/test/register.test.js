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
