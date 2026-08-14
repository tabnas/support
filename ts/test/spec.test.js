/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* spec.test.js — the fixture loader.
 *
 * `go/spec_test.go` asserts the same row count and the same line numbers
 * against the same file. Those two numbers are the whole agreement: if
 * the runtimes disagree about what a row IS, nothing a row says can be
 * trusted.
 */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Os = require('node:os')
const Path = require('node:path')

const {
  findSpecDir, loadSpec, loadSpecDir, parseSpec,
} = require('../dist/support.js')


const SPEC = findSpecDir(__dirname)


describe('spec-load', () => {

  it('skips the header, blank lines and tab-free comments', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))

    assert.deepEqual(spec.header, ['input', 'expected'])
    assert.equal(spec.file, 'loader-rows.tsv')

    // The file's own layout, asserted by line number: header on 1,
    // comments on 2-4, then a row, a blank, and the rest.
    assert.deepEqual(spec.rows.map((r) => r.line), [5, 7, 8, 9, 10])
    assert.deepEqual(spec.rows.map((r) => r.index), [0, 1, 2, 3, 4])
  })

  it('treats a #-leading line WITH a tab as data', () => {
    // Otherwise a fixture whose input is a C preprocessor directive, or a
    // comment in the parsed language, could not be written at all.
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    const row = spec.rows.find((r) => 7 === r.line)
    assert.equal(row.col(0), '#hash')
    assert.equal(row.col(1), '2')
  })

  it('keeps an empty leading column', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    const row = spec.rows.find((r) => 8 === r.line)
    assert.equal(row.col(0), '')
    assert.equal(row.col(1), '3')
  })

  it('returns columns raw, and decodes only on request', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    const row = spec.rows.find((r) => 9 === r.line)

    // Raw: the two characters backslash and t.
    assert.equal(row.col(0), 'b\\tc')
    // Decoded: a real tab.
    assert.equal(row.unesc(0), 'b\tc')
  })

  it('allows a row with more columns than the header names', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    const row = spec.rows.find((r) => 10 === r.line)
    assert.equal(row.cols.length, 3)
    assert.equal(row.col(2), '6')
  })

  it('reads a column out of range as empty, not as a crash', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    const row = spec.rows[0]
    assert.equal(row.col(9), '')
    assert.equal(row.col(-1), '')
    assert.equal(row.unesc(9), '')
    assert.equal(row.named('nosuch'), '')
    assert.equal(row.index_of('nosuch'), -1)
  })

  it('reads a column by header name', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    const row = spec.rows[0]
    assert.equal(row.named('input'), 'a')
    assert.equal(row.named('expected'), '1')
    assert.equal(row.unescNamed('input'), 'a')
    assert.equal(row.index_of('expected'), 1)
    assert.equal(row.resolve('expected'), 1)
    assert.equal(row.resolve(0), 0)
  })

  it('throws on an unknown column name rather than reading column -1', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    assert.throws(() => spec.rows[0].resolve('nosuch'), /no column named/)
  })

  it('reports file and line for a failure message', () => {
    const spec = loadSpec(Path.join(SPEC, 'util', 'loader-rows.tsv'))
    assert.equal(spec.rows[0].where(), 'loader-rows.tsv:5')
  })

  it('throws a locatable error for a missing file', () => {
    assert.throws(
      () => loadSpec(Path.join(SPEC, 'util', 'does-not-exist.tsv')),
      /spec file not found/)
  })
})


describe('spec-parse', () => {

  it('parses text without touching the filesystem', () => {
    const spec = parseSpec('inline.tsv', 'a\tb\n1\t2\n')
    assert.deepEqual(spec.header, ['a', 'b'])
    assert.equal(spec.rows.length, 1)
    assert.deepEqual(spec.rows[0].cols, ['1', '2'])
    assert.equal(spec.path, '')
  })

  it('splits CRLF the same as LF', () => {
    // A fixture checked out on Windows must mean what it means anywhere
    // else — the CR is a line ending, not part of the last column.
    const lf = parseSpec('x.tsv', 'a\tb\n1\t2\n3\t4\n')
    const crlf = parseSpec('x.tsv', 'a\tb\r\n1\t2\r\n3\t4\r\n')
    assert.deepEqual(
      crlf.rows.map((r) => r.cols), lf.rows.map((r) => r.cols))
    assert.deepEqual(crlf.header, lf.header)
  })

  it('trims a CR at end of file, final newline missing', () => {
    // CRLF endings with the last line unterminated: the final segment of
    // the split keeps its CR, which the Go loader's per-line trim
    // removes — so without stripping it here the two runtimes would
    // disagree about the last column of the last row.
    // `go/spec_test.go` pins the same bytes.
    const spec = parseSpec('x.tsv', 'a\tb\r\n1\tERROR:foo\r')
    assert.equal(spec.rows.length, 1)
    assert.deepEqual(spec.rows[0].cols, ['1', 'ERROR:foo'])
  })

  it('strips a leading BOM so the first column name still matches', () => {
    const spec = parseSpec('x.tsv', '\uFEFFinput\texpected\na\t1\n')
    assert.deepEqual(spec.header, ['input', 'expected'])
    assert.equal(spec.rows[0].named('input'), 'a')
  })

  it('handles a file with no trailing newline', () => {
    const spec = parseSpec('x.tsv', 'a\tb\n1\t2')
    assert.equal(spec.rows.length, 1)
  })

  it('handles a header-only file as zero rows, not as an error', () => {
    const spec = parseSpec('x.tsv', 'a\tb\n')
    assert.equal(spec.rows.length, 0)
    assert.deepEqual(spec.header, ['a', 'b'])
  })

  it('can be told the file has no header', () => {
    const spec = parseSpec('x.tsv', '1\t2\n3\t4\n', { header: false })
    assert.deepEqual(spec.header, [])
    assert.equal(spec.rows.length, 2)
    assert.deepEqual(spec.rows[0].cols, ['1', '2'])
    assert.equal(spec.rows[0].line, 1)
  })

  it('can be told to keep comment lines', () => {
    const spec = parseSpec('x.tsv', 'a\n# kept\n', { comment: false })
    assert.equal(spec.rows.length, 1)
    assert.equal(spec.rows[0].col(0), '# kept')
  })

  it('rejects a row with too few columns when a minimum is set', () => {
    assert.throws(
      () => parseSpec('x.tsv', 'a\tb\nonly-one\n', { minCols: 2 }),
      /x.tsv:2: expected at least 2/)
  })

  it('accepts a one-column row by default', () => {
    const spec = parseSpec('x.tsv', 'a\nsolo\n')
    assert.deepEqual(spec.rows[0].cols, ['solo'])
  })
})


describe('spec-dir', () => {

  it('loads every .tsv in a directory, sorted', () => {
    const specs = loadSpecDir(Path.join(SPEC, 'adder'))
    assert.deepEqual(specs.map((s) => s.file), ['basic.tsv', 'errors.tsv'])
    assert.ok(specs.every((s) => 0 < s.rows.length))
    assert.ok(specs.every((s) => '' !== s.path))
  })

  it('ignores non-.tsv files', () => {
    const dir = Fs.mkdtempSync(Path.join(Os.tmpdir(), 'tabnas-support-'))
    try {
      Fs.writeFileSync(Path.join(dir, 'b.tsv'), 'a\n1\n')
      Fs.writeFileSync(Path.join(dir, 'a.tsv'), 'a\n1\n')
      Fs.writeFileSync(Path.join(dir, 'notes.md'), 'not a fixture')
      Fs.writeFileSync(Path.join(dir, 'x.tsv.bak'), 'a\n1\n')

      const specs = loadSpecDir(dir)
      assert.deepEqual(specs.map((s) => s.file), ['a.tsv', 'b.tsv'])
    }
    finally {
      Fs.rmSync(dir, { recursive: true, force: true })
    }
  })

  it('ignores a DIRECTORY whose name ends in .tsv', () => {
    // Handing a directory to readFileSync aborts the run — and the Go
    // loader skips it, so the same fixture tree would work there and not
    // here.
    const dir = Fs.mkdtempSync(Path.join(Os.tmpdir(), 'tabnas-support-'))
    try {
      Fs.mkdirSync(Path.join(dir, 'nested.tsv'))
      Fs.writeFileSync(Path.join(dir, 'real.tsv'), 'a\n1\n')

      const specs = loadSpecDir(dir)
      assert.deepEqual(specs.map((s) => s.file), ['real.tsv'])
    }
    finally {
      Fs.rmSync(dir, { recursive: true, force: true })
    }
  })

  it('throws for a directory with no fixtures in it', () => {
    // The silent-pass failure mode one level up: a runner over an empty
    // directory reports green having run nothing.
    const dir = Fs.mkdtempSync(Path.join(Os.tmpdir(), 'tabnas-support-'))
    try {
      Fs.writeFileSync(Path.join(dir, 'notes.md'), 'not a fixture')
      assert.throws(() => loadSpecDir(dir), /no \.tsv fixtures/)
    }
    finally {
      Fs.rmSync(dir, { recursive: true, force: true })
    }
  })

  it('throws for a missing directory', () => {
    assert.throws(
      () => loadSpecDir(Path.join(SPEC, 'nosuchdir')),
      /spec directory not found/)
  })
})


describe('spec-dir-find', () => {

  it('walks up from a starting directory to test/spec', () => {
    assert.equal(findSpecDir(__dirname), SPEC)
    assert.equal(findSpecDir(Path.join(__dirname, '..')), SPEC)
    assert.equal(findSpecDir(Path.join(SPEC, 'adder')), SPEC)
  })

  it('defaults to the working directory', () => {
    // `npm test` runs from ts/, which is what every suite here relies on.
    assert.equal(findSpecDir(), SPEC)
  })

  it('throws when there is no test/spec above the start', () => {
    assert.throws(() => findSpecDir(Os.tmpdir()), /no test.spec directory/)
  })
})
