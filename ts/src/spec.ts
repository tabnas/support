/* Copyright (c) 2026 tabnas, MIT License */

/* spec.ts
 * The TSV spec-fixture loader.
 *
 * Every tabnas package keeps its cross-runtime conformance fixtures in
 * `test/spec/*.tsv` at the repo root, above both runtimes, so `ts/` and
 * `go/` run the same files. Before this package each repo carried its own
 * loader, and they had quietly drifted: one decoded `\t` and another did
 * not, one skipped `#` comment lines and another crashed on them, one
 * decoded escapes in every column and another only in the first. A row
 * that means two different things in two runtimes cannot pin agreement on
 * anything else, so there is one loader now, and `go/spec.go` mirrors it.
 *
 * The rules, in full:
 *
 * - Line 1 is a header naming the columns. Names are how a row is read by
 *   `row.named('input')` rather than by position.
 * - A blank line is skipped.
 * - A line that starts with `#` and holds no tab is a comment and is
 *   skipped. A `#`-leading line WITH a tab is data, so a fixture whose
 *   input is a C preprocessor directive still works.
 * - Columns are returned RAW. Escape decoding is per-column and explicit
 *   (`row.unesc(i)`), because the `expected` column is normally JSON,
 *   which carries its own escapes and must not be decoded twice.
 * - Line numbers are the physical 1-based line in the file, so a failure
 *   message points an editor at the offending row.
 */

import { readFileSync, readdirSync, existsSync, statSync } from 'node:fs'
import { join, basename, dirname, resolve, sep } from 'node:path'

import { unescape } from './escape'


// How to interpret a fixture file. The defaults are the tabnas convention;
// override them only for a fixture that genuinely differs.
export type SpecOptions = {
  header?: boolean   // First line names the columns. Default true.
  comment?: boolean  // Skip `#`-leading lines that hold no tab. Default true.
  minCols?: number   // Reject a data row with fewer columns. Default 1.
}


const DEFAULTS: Required<SpecOptions> = {
  header: true,
  comment: true,
  minCols: 1,
}


// One data row of a fixture file.
export class SpecRow {
  // Base name of the file this row came from (`happy.tsv`).
  file: string

  // Physical 1-based line number within that file.
  line: number

  // 0-based position among the file's DATA rows (header and skipped
  // lines do not advance it).
  index: number

  // The row's columns, exactly as they appear in the file.
  cols: string[]

  // The file's header names, shared with every row of the file.
  header: string[]

  constructor(
    file: string, line: number, index: number,
    cols: string[], header: string[],
  ) {
    this.file = file
    this.line = line
    this.index = index
    this.cols = cols
    this.header = header
  }

  // Raw column by position. Out of range is '' — a fixture with a trailing
  // optional column should not need a length check at every use.
  col(i: number): string {
    return 0 <= i && i < this.cols.length ? this.cols[i] : ''
  }

  // Escape-decoded column by position. This is what a parser input column
  // goes through.
  unesc(i: number): string {
    return unescape(this.col(i))
  }

  // Position of a header name, or -1 when the file has no such column.
  index_of(name: string): number {
    return this.header.indexOf(name)
  }

  // Raw column by header name; '' when there is no such column.
  named(name: string): string {
    return this.col(this.index_of(name))
  }

  // Escape-decoded column by header name.
  unescNamed(name: string): string {
    return unescape(this.named(name))
  }

  // Resolve a column selector — a position or a header name — to a
  // position. An unknown name is a defect in the caller, not a missing
  // value, so it throws rather than silently reading column -1.
  resolve(sel: number | string): number {
    if ('number' === typeof sel) {
      return sel
    }
    const i = this.index_of(sel)
    if (-1 === i) {
      throw new Error(
        `${this.file}: no column named '${sel}' (header: ${this.header.join(', ')})`)
    }
    return i
  }

  // `<file>:<line>` — the prefix every failure message should carry.
  where(): string {
    return this.file + ':' + this.line
  }
}


// A loaded fixture file.
export class SpecFile {
  file: string       // Base name (`happy.tsv`).
  path: string       // Path it was read from ('' when parsed from text).
  header: string[]   // Column names, [] when `header: false`.
  rows: SpecRow[]    // Data rows, in file order.

  constructor(file: string, path: string, header: string[], rows: SpecRow[]) {
    this.file = file
    this.path = path
    this.header = header
    this.rows = rows
  }
}


// Parse fixture text that is already in memory. `loadSpec` is this plus a
// file read; tests of the loader itself use this form.
export function parseSpec(
  file: string, text: string, options?: SpecOptions,
): SpecFile {
  const opts = { ...DEFAULTS, ...options }

  // A BOM ahead of the header would become part of the first column name,
  // and the lookup by that name would then fail in a way nothing about the
  // fixture explains.
  if (text.startsWith('\uFEFF')) {
    text = text.slice(1)
  }

  const lines = text.split(/\r?\n/)
  const name = basename(file)

  let header: string[] = []
  const rows: SpecRow[] = []

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const lineNo = i + 1

    if (opts.header && 0 === i) {
      header = line.split('\t')
      continue
    }

    if ('' === line) {
      continue
    }

    // A comment needs no tab; a data row always has at least one, so a
    // `#`-leading source stays usable as input.
    if (opts.comment && line.startsWith('#') && !line.includes('\t')) {
      continue
    }

    const cols = line.split('\t')
    if (cols.length < opts.minCols) {
      throw new Error(
        `${name}:${lineNo}: expected at least ${opts.minCols} ` +
        `tab-separated column(s), found ${cols.length}`)
    }

    rows.push(new SpecRow(name, lineNo, rows.length, cols, header))
  }

  return new SpecFile(name, '', header, rows)
}


// Load one fixture file by path.
export function loadSpec(path: string, options?: SpecOptions): SpecFile {
  if (!existsSync(path)) {
    throw new Error('spec file not found: ' + path)
  }
  const spec = parseSpec(basename(path), readFileSync(path, 'utf8'), options)
  spec.path = path
  return spec
}


// Load every `*.tsv` in a directory, sorted by name so both runtimes and
// successive runs visit them in the same order. Discovery by listing is
// deliberate: adding a fixture then runs it without editing a runner.
export function loadSpecDir(dir: string, options?: SpecOptions): SpecFile[] {
  if (!existsSync(dir) || !statSync(dir).isDirectory()) {
    throw new Error('spec directory not found: ' + dir)
  }
  return readdirSync(dir)
    .filter((name) => name.endsWith('.tsv'))
    .sort()
    .map((name) => loadSpec(join(dir, name), options))
}


// Walk up from `from` looking for a `test/spec` directory, and return it.
//
// This replaces the `join(__dirname, '..', '..', 'test', 'spec')` that
// every repo hard-codes — a relative hop that has to be recounted whenever
// a test moves a directory, and that is spelt differently in `go/` anyway.
export function findSpecDir(from?: string): string {
  let dir = resolve(from ?? process.cwd())

  for (; ;) {
    const candidate = join(dir, 'test', 'spec')
    if (existsSync(candidate) && statSync(candidate).isDirectory()) {
      return candidate
    }

    const parent = dirname(dir)
    if (parent === dir) {
      break // Filesystem root: there is nowhere left to look.
    }
    dir = parent
  }

  throw new Error(
    `no test${sep}spec directory found at or above: ` +
    resolve(from ?? process.cwd()))
}
