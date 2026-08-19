/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* expect.test.js — reading the expected column, and comparing against it. */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Path = require('node:path')

const {
  findSpecDir, loadSpec,
  isErrorExpect, errorCode, errorExpect, parseExpect, equalValue,
  formatValue,
} = require('../dist/support.js')


const SPEC = findSpecDir(__dirname)


describe('expect-error', () => {

  it('spec: util/expect-error.tsv', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'expect-error.tsv'))
    assert.ok(0 < spec.rows.length, 'no cases')

    for (const row of spec.rows) {
      const cell = row.named('expected')
      const wantIsError = parseExpect(row.named('iserror'))

      assert.equal(isErrorExpect(cell), wantIsError,
        `${row.where()}: isErrorExpect(${JSON.stringify(cell)})`)

      // A cell that IS an error expectation but a malformed one: reading
      // it must throw rather than yield a position nobody meant.
      if (parseExpect(row.named('bad'))) {
        assert.throws(() => errorExpect(cell), /is 1-based/,
          `${row.where()}: errorExpect(${JSON.stringify(cell)})`)
        assert.throws(() => errorCode(cell), /is 1-based/,
          `${row.where()}: errorCode(${JSON.stringify(cell)})`)
        continue
      }

      if (wantIsError) {
        assert.equal(errorCode(cell), parseExpect(row.named('code')),
          `${row.where()}: errorCode(${JSON.stringify(cell)})`)

        // The position channel. `row`/`col` are empty for a cell that
        // pins no position, and parseExpect reads an empty cell as
        // undefined -- which is exactly what errorExpect returns then, so
        // the same two assertions cover both kinds of row and neither
        // kind can go unchecked.
        const at = errorExpect(cell)
        assert.equal(at.row, parseExpect(row.named('row')),
          `${row.where()}: errorExpect(${JSON.stringify(cell)}).row`)
        assert.equal(at.col, parseExpect(row.named('col')),
          `${row.where()}: errorExpect(${JSON.stringify(cell)}).col`)
      }
      else {
        assert.throws(() => errorCode(cell), /not an error expectation/,
          `${row.where()}: errorCode(${JSON.stringify(cell)})`)
        assert.throws(() => errorExpect(cell), /not an error expectation/,
          `${row.where()}: errorExpect(${JSON.stringify(cell)})`)
      }
    }
  })
})


describe('expect-parse', () => {

  it('reads an empty cell as no value', () => {
    assert.equal(parseExpect(''), undefined)
  })

  it('reads a cell as JSON, not as escaped text', () => {
    // The two characters `\n` inside a JSON string are a newline by JSON's
    // own rules. Escape-decoding the cell first would produce a real
    // newline inside the quotes, which is not valid JSON at all.
    assert.equal(parseExpect('"a\\nb"'), 'a\nb')
    assert.deepEqual(parseExpect('{"a":[1,2]}'), { a: [1, 2] })
    assert.equal(parseExpect('null'), null)
    assert.equal(parseExpect('0'), 0)
    assert.equal(parseExpect('false'), false)
  })

  it('names the offending cell when the JSON is bad', () => {
    assert.throws(() => parseExpect('{oops'), /invalid expected JSON/)
    assert.throws(() => parseExpect('1 2'), /invalid expected JSON/)
  })

  it('reads a number beyond float range as Infinity', () => {
    // JSON.parse's own answer. Go's encoding/json rejects the literal
    // outright, so `go/expect.go` goes out of its way to match this —
    // a JSON parser's own fixtures reach here.
    assert.equal(parseExpect('1e400'), Infinity)
    assert.equal(parseExpect('-1e400'), -Infinity)
    assert.deepEqual(parseExpect('[1e400]'), [Infinity])
    assert.deepEqual(parseExpect('{"a":1e400}'), { a: Infinity })
  })

  it('cannot tell integers beyond 2^53 apart, and says so', () => {
    // A canonical-runtime limit, shared rather than papered over: Go
    // rounds the same way, so a fixture must not pin such an integer.
    assert.equal(parseExpect('9007199254740993'), 9007199254740992)
  })
})


describe('expect-equal', () => {

  it('spec: util/value-equal.tsv', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'value-equal.tsv'))
    assert.ok(0 < spec.rows.length, 'no cases')

    for (const row of spec.rows) {
      const a = parseExpect(row.named('a'))
      const b = parseExpect(row.named('b'))
      const want = parseExpect(row.named('equal'))

      assert.equal(equalValue(a, b), want,
        `${row.where()}: equalValue(${row.named('a')}, ${row.named('b')})`)

      // Equality is symmetric; a comparison that is not would make a
      // fixture's meaning depend on which column it was written in.
      assert.equal(equalValue(b, a), want, `${row.where()}: reversed`)
    }
  })

  it('treats NaN as equal to itself', () => {
    // Not expressible in a fixture — JSON has no NaN — but a grammar can
    // produce one, and `===` would report it unequal to itself.
    assert.equal(equalValue(NaN, NaN), true)
    assert.equal(equalValue(NaN, 0), false)
    assert.equal(equalValue([NaN], [NaN]), true)
  })

  it('separates an absent key from an explicit undefined', () => {
    assert.equal(equalValue({ a: undefined }, {}), false)
    assert.equal(equalValue({ a: undefined }, { a: undefined }), true)
  })

  it('compares own keys only, not inherited ones', () => {
    // `k in obj` walks the prototype chain, so a result keyed
    // `constructor` or `valueOf` would match ANY object of the same size.
    // Relaxed grammars parse those keys perfectly happily — that is what
    // the `funky-keys` fixtures are for — so this is reachable, and a
    // false PASS is the worst kind.
    assert.equal(equalValue({ constructor: Object }, { x: 1 }), false)
    assert.equal(equalValue({ valueOf: 1 }, { x: 1 }), false)
    assert.equal(equalValue({ toString: 1 }, { hasOwnProperty: 1 }), false)

    // The keys themselves still compare normally.
    assert.equal(equalValue({ constructor: 1 }, { constructor: 1 }), true)
    assert.equal(equalValue({ constructor: 1 }, { constructor: 2 }), false)

    // And an object with no prototype at all behaves the same.
    const bare = Object.create(null)
    bare.a = 1
    assert.equal(equalValue(bare, { a: 1 }), true)
  })

  it('applies a normalize hook to both sides, at every depth', () => {
    // The hook is how a runtime-specific container — an insertion-ordered
    // map, a reference wrapper — is unwrapped into the plain value the
    // fixture describes.
    class Ordered {
      constructor(vals) { this.vals = vals }
    }
    const normalize = (v) => (v instanceof Ordered ? v.vals : v)

    const got = new Ordered({ a: new Ordered({ b: 1 }) })
    assert.equal(equalValue(got, { a: { b: 1 } }, { normalize }), true)
    assert.equal(equalValue(got, { a: { b: 2 } }, { normalize }), false)
    assert.equal(equalValue(got, { a: { b: 1 } }), false)
  })

  it('formats a value the way the fixture would write it', () => {
    assert.equal(formatValue({ a: 1 }), '{"a":1}')
    assert.equal(formatValue([1, 'x']), '[1,"x"]')
    assert.equal(formatValue(undefined), 'undefined')
    assert.equal(formatValue(null), 'null')

    // A value JSON cannot render still has to produce something readable:
    // this runs inside a failure message, where throwing again would
    // replace the real failure with a mystery.
    const cyclic = {}
    cyclic.self = cyclic
    assert.ok(0 < formatValue(cyclic).length)
    assert.equal(formatValue(() => 1), '() => 1')
  })
})
