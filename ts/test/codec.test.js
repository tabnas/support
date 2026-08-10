/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* codec.test.js — the escape codec, against the shared fixture. */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Path = require('node:path')

const {
  unescape, escape, findSpecDir, loadSpec, parseExpect,
} = require('../dist/support.js')


const SPEC = findSpecDir(__dirname)


describe('codec', () => {

  it('spec: util/codec.tsv', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'codec.tsv'))
    assert.ok(0 < spec.rows.length, 'no cases')

    for (const row of spec.rows) {
      const source = row.named('source')
      const value = parseExpect(row.named('value'))
      const escaped = parseExpect(row.named('escaped'))

      assert.equal(unescape(source), value,
        `${row.where()}: unescape(${JSON.stringify(source)})`)

      assert.equal(escape(value), escaped,
        `${row.where()}: escape(${JSON.stringify(value)})`)

      // Round trip. Encoding then decoding must be the identity — that is
      // what lets a generator write a fixture the loader can read back.
      assert.equal(unescape(escape(value)), value,
        `${row.where()}: round trip`)
    }
  })

  it('leaves a cell with no backslash exactly as it is', () => {
    for (const s of ['', 'plain', 'a b c', '{"a":1}', 'ünïcödé', '𝒜𝒷']) {
      assert.equal(unescape(s), s)
      assert.equal(escape(s), s)
    }
  })

  it('decodes left to right, so escapes cannot be chained by accident', () => {
    // `\\` consumes both backslashes, leaving `n` as an ordinary letter.
    assert.equal(unescape('\\\\n'), '\\n')
    // Whereas an unescaped pair really is a newline.
    assert.equal(unescape('\\n'), '\n')
  })

  it('carries a tab and a newline through a cell', () => {
    // The two characters a TSV cell physically cannot hold.
    assert.equal(unescape('a\\tb'), 'a\tb')
    assert.equal(unescape('a\\nb'), 'a\nb')
    assert.equal(escape('a\tb'), 'a\\tb')
    assert.equal(escape('a\nb'), 'a\\nb')
  })

  it('round-trips arbitrary text', () => {
    const samples = [
      '', 'a', '\n', '\r\n', '\t\t', '\\', '\\\\', '\\n', 'a\\nb',
      'line1\nline2\r\nline3', 'C:\\dir\\file', '/^\\d+$/', '"\t"',
      'ünïcödé\t𝒜𝒷\n',
    ]
    for (const s of samples) {
      assert.equal(unescape(escape(s)), s, JSON.stringify(s))
    }
  })
})
