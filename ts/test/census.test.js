/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* census.test.js — every shared fixture runs in BOTH runtimes.
 *
 * `test/spec/adder/` needs no check: both runtimes discover it by
 * directory listing, so a fixture added there runs in both without
 * anyone touching a runner.
 *
 * `test/spec/util/` and `test/spec/census/` cannot work that way — each
 * file has its own column shape and its own assertion, so each suite
 * names the files it runs. That leaves a gap the rest of the suite
 * cannot see: a fixture added to disk and wired into ONE runtime passes
 * everywhere, and the row it pins is then agreed by nobody.
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

const {
  findSpecDir, codesInSpecDir, compareCatalogues, coverage,
} = require('../dist/support.js')


const SPEC = findSpecDir(__dirname)


describe('census', () => {

  // `util/`, `census/` and `register/` are all named-file families: each
  // fixture has its own column shape, so each suite names the files it
  // runs, and the tripwire below is what notices one wired into a single
  // runtime.
  //
  // `register` was added to this list in the same change that created the
  // directory. A new family that is not listed here is invisible to the
  // tripwire, so the safeguard would pass while a fixture ran in one port
  // only -- which is the safeguard's whole subject.
  for (const family of ['census', 'register', 'util']) {
    it(`names every ${family} fixture in this runtime`, () => {
      const familyDir = Path.join(SPEC, family)
      const fixtures = Fs.readdirSync(familyDir)
        .filter((name) => name.endsWith('.tsv'))
        .sort()

      // A corpus that vanished must not pass quietly either.
      assert.ok(0 < fixtures.length, 'no fixtures in ' + familyDir)

      const sources = Fs.readdirSync(__dirname)
        .filter((name) => name.endsWith('.js'))
        .map((name) => Fs.readFileSync(Path.join(__dirname, name), 'utf8'))
        .join('\n')

      const missing = fixtures.filter((name) => !sources.includes(name))

      assert.deepEqual(missing, [],
        `fixture(s) not named by any test in ts/test/: ${missing.join(', ')}` +
        ' — wire them in here AND in go/, or the row is agreed by nobody')
    })
  }

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


/* The census helpers, over the test/spec/census/ fixtures:
 * codes.tsv is the ordinary two-column shape with the expectation last;
 * named-col.tsv is three-column with the expectation in the middle, so
 * its trailing `note` column can bait the default column selection.
 * `go/census_test.go` asserts the same lists.
 */

describe('census-codes', () => {

  it('collects code-style expectations only: sorted, unique', () => {
    // With the expectation column named, both files are read right:
    // codes.tsv contributes unexpected, unterminated_string and
    // positioned (its message-style, bare-ERROR and value rows contribute
    // nothing), and named-col.tsv contributes named_only plus a repeat of
    // unexpected. `positioned` is written `ERROR:positioned@2:5`: a code
    // with a position suffix is still a code.
    assert.deepEqual(codesInSpecDir(Path.join(SPEC, 'census'), {
      name: 'expected',
    }), ['named_only', 'positioned', 'unexpected', 'unterminated_string'])
  })

  it('selects the expectation column by position', () => {
    // Column 1 is `expected` in both files, so the answer is the same.
    assert.deepEqual(codesInSpecDir(Path.join(SPEC, 'census'), {
      col: 1,
    }), ['named_only', 'positioned', 'unexpected', 'unterminated_string'])
  })

  it('defaults to each row\'s last column', () => {
    // codes.tsv's last column IS the expectation; named-col.tsv's is
    // `note`, whose code-shaped bait is then collected and whose real
    // codes are missed — the mistake the column selection exists to
    // prevent, pinned so it stays visible.
    assert.deepEqual(codesInSpecDir(Path.join(SPEC, 'census')),
      ['another_trap', 'positioned', 'trap_note', 'unexpected',
        'unterminated_string'])
  })

  it('rejects an unknown column name', () => {
    assert.throws(
      () => codesInSpecDir(Path.join(SPEC, 'census'), { name: 'nope' }),
      /no column named/)
  })

  it('rejects a missing fixture directory', () => {
    // The shared loader's guard: a census over nothing must not report
    // "no codes" as if it had looked.
    assert.throws(
      () => codesInSpecDir(Path.join(SPEC, 'no-such-dir')),
      /spec directory not found/)
  })
})


describe('census-catalogues', () => {

  it('reports identical catalogues as identical', () => {
    const cat = { unexpected: 'unexpected {token}', unprintable: 'oops' }
    assert.deepEqual(compareCatalogues(cat, { ...cat }),
      { missing: [], extra: [], templateMismatch: [] })
    assert.deepEqual(compareCatalogues({}, {}),
      { missing: [], extra: [], templateMismatch: [] })
  })

  it('finds a missing key, an extra key and a changed template', () => {
    const a = { alpha: 'A', beta: 'B', gamma: 'C' }
    const b = { beta: 'B!', gamma: 'C', delta: 'D' }
    assert.deepEqual(compareCatalogues(a, b), {
      missing: ['alpha'],
      extra: ['delta'],
      templateMismatch: ['beta'],
    })
  })

  it('sorts all three lists', () => {
    const some = { zz: '1', mm: '1', aa: '1' }
    const drifted = { zz: '2', mm: '2', aa: '2' }
    assert.deepEqual(compareCatalogues(some, {}).missing,
      ['aa', 'mm', 'zz'])
    assert.deepEqual(compareCatalogues({}, some).extra,
      ['aa', 'mm', 'zz'])
    assert.deepEqual(compareCatalogues(some, drifted).templateMismatch,
      ['aa', 'mm', 'zz'])
  })

  it('compares templates byte for byte', () => {
    // One space of drift is still drift: "means the same" is not the
    // contract, identical bytes is.
    assert.deepEqual(
      compareCatalogues({ near: 'near {x}' }, { near: 'near  {x}' }),
      { missing: [], extra: [], templateMismatch: ['near'] })
  })

  it('sorts by code point, matching Go\'s byte order', () => {
    // '\u{10000}' is a surrogate pair in UTF-16, and the default Array
    // sort compares code units — which would put it BEFORE '�'.
    // Go's sort.Strings compares UTF-8 bytes, which for valid UTF-8 is
    // code-point order, and puts it after. The census sorts by code
    // point so the two runtimes order the same answer the same way.
    // `go/census_test.go` pins the same keys.
    assert.deepEqual(
      compareCatalogues({ '\u{10000}': 'x', '�': 'y' }, {}).missing,
      ['�', '\u{10000}'])
    assert.deepEqual(
      coverage(['\u{10000}', '�'], []).uncovered,
      ['�', '\u{10000}'])
  })
})


describe('census-coverage', () => {

  it('reports a clean package as clean', () => {
    assert.deepEqual(coverage(['a', 'b'], ['b', 'a']),
      { uncovered: [], orphan: [] })
  })

  it('finds uncovered and orphan codes, sorted', () => {
    assert.deepEqual(
      coverage(
        ['unterminated_string', 'unexpected', 'unprintable'],
        ['unexpected', 'mystery', 'another']),
      {
        uncovered: ['unprintable', 'unterminated_string'],
        orphan: ['another', 'mystery'],
      })
  })

  it('ties out against the code census', () => {
    const exercised = codesInSpecDir(Path.join(SPEC, 'census'), {
      name: 'expected',
    })
    assert.deepEqual(
      coverage(
        ['named_only', 'positioned', 'unexpected', 'unreached',
          'unterminated_string'],
        exercised),
      { uncovered: ['unreached'], orphan: [] })
  })
})
