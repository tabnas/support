/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* census.test.js — every shared fixture runs in BOTH runtimes.
 *
 * `test/spec/adder/` needs no check: both runtimes discover it by
 * directory listing, so a fixture added there runs in both without
 * anyone touching a runner.
 *
 * `test/spec/util/` cannot work that way — each file has its own column
 * shape and its own assertion, so each suite names the files it runs.
 * That leaves a gap the rest of the suite cannot see: a fixture added to
 * disk and wired into ONE runtime passes everywhere, and the row it
 * pins is then agreed by nobody.
 *
 * This is a static tripwire rather than proof. It checks that each
 * fixture's name appears somewhere in this runtime's test sources; a
 * name sitting in a comment would satisfy it. What it does catch is the
 * realistic mistake — adding a fixture and wiring up one side — which
 * nothing else here would.
 *
 * `go/census_test.go` asserts the same thing over the Go sources.
 */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const { findSpecDir } = require('../dist/support.js')


const SPEC = findSpecDir(__dirname)


describe('census', () => {

  it('names every util fixture in this runtime', () => {
    const utilDir = Path.join(SPEC, 'util')
    const fixtures = Fs.readdirSync(utilDir)
      .filter((name) => name.endsWith('.tsv'))
      .sort()

    // A corpus that vanished must not pass quietly either.
    assert.ok(0 < fixtures.length, 'no fixtures in ' + utilDir)

    const sources = Fs.readdirSync(__dirname)
      .filter((name) => name.endsWith('.js'))
      .map((name) => Fs.readFileSync(Path.join(__dirname, name), 'utf8'))
      .join('\n')

    const missing = fixtures.filter((name) => !sources.includes(name))

    assert.deepEqual(missing, [],
      `fixture(s) not named by any test in ts/test/: ${missing.join(', ')}` +
      ' — wire them in here AND in go/, or the row is agreed by nobody')
  })

  it('discovers the adder fixtures by listing, in both runtimes', () => {
    // Recorded so the asymmetry above is deliberate rather than
    // forgotten: this directory needs no census because adding a file
    // to it runs it in both runtimes automatically.
    const adderDir = Path.join(SPEC, 'adder')
    const fixtures = Fs.readdirSync(adderDir).filter((n) => n.endsWith('.tsv'))

    assert.ok(0 < fixtures.length, 'no fixtures in ' + adderDir)

    const adderTest = Fs.readFileSync(
      Path.join(__dirname, 'adder.test.js'), 'utf8')
    assert.match(adderTest, /\.dir\(/,
      'adder.test.js must run the directory, not named files')
  })
})
