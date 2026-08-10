/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* adder.test.js
 * The end-to-end check on this package: a real grammar, driven entirely
 * by the shared fixtures, through the public API.
 *
 * `go/adder/adder_test.go` runs the SAME rows against the SAME grammar in
 * Go, so a divergence anywhere in the chain — the escape codec, the
 * comment rule, the `ERROR:<code>` contract, the value comparison — turns
 * one of the two runtimes red.
 */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Path = require('node:path')

const { Tabnas, TabnasError } = require('@tabnas/parser')

const { findSpecDir, makeRunner, loadSpec } = require('../dist/support.js')
const { adder } = require('../dist/adder.js')


const SPEC = findSpecDir(__dirname)

const tn = new Tabnas({ plugins: [adder] })

makeRunner({
  parse: (src) => tn.parse(src),

  // Assert the error TYPE while pulling out the code: an ERROR row that
  // passed because some unrelated TypeError escaped the parser would be a
  // green tick over a broken grammar.
  errorCode: (err) => {
    assert.ok(err instanceof TabnasError, 'expected TabnasError, got: ' + err)
    return err.code
  },
})
  .dir(Path.join(SPEC, 'adder'))


describe('adder', () => {

  it('parses the README examples', () => {
    assert.equal(tn.parse('1+2+3'), 6)
    assert.equal(tn.parse('10+20'), 30)
    assert.equal(tn.parse('12+3+45'), 60)
  })

  it('is a plugin, so it applies to any bare instance', () => {
    const other = new Tabnas()
    other.use(adder)
    assert.equal(other.parse('1+2'), 3)
  })

  it('holds no state between parses', () => {
    assert.equal(tn.parse('1+2'), 3)
    assert.equal(tn.parse('1+2'), 3)
    assert.equal(tn.parse('4'), 4)
  })

  it('repeats in one stack frame, so length is not a limit', () => {
    const n = 500
    const src = Array.from({ length: n }, (_, i) => i + 1).join('+')
    assert.equal(tn.parse(src), (n * (n + 1)) / 2)
  })

  it('runs every fixture row — no fixture is silently skipped', () => {
    // The runner reports per row, but only a count checked here catches a
    // fixture that stopped being loaded at all.
    const basic = loadSpec(Path.join(SPEC, 'adder', 'basic.tsv'))
    const errors = loadSpec(Path.join(SPEC, 'adder', 'errors.tsv'))
    assert.ok(15 < basic.rows.length, 'basic.tsv rows: ' + basic.rows.length)
    assert.ok(5 < errors.rows.length, 'errors.tsv rows: ' + errors.rows.length)
  })
})
