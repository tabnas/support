/* Copyright (c) 2026 tabnas, MIT License */

/* support.ts
 * @tabnas/support — shared test-support utilities for the tabnas parser
 * system.
 *
 * Every tabnas package (parser, json, jsonic, abnf, csv, ...) proves the
 * two runtimes agree by running one set of TSV fixtures from `test/spec/`
 * in both. This package is the machinery that reads those fixtures, so
 * there is one loader with one set of rules instead of a copy per repo
 * quietly drifting from its Go twin.
 *
 * The Go half is `github.com/tabnas/support/go`, and the two are written
 * to behave identically — same escape codec, same comment and blank-line
 * handling, same `ERROR:<code>` contract, same value comparison.
 *
 *   const { findSpecDir, makeRunner } = require('@tabnas/support')
 *   const { Tabnas } = require('@tabnas/parser')
 *
 *   const tn = new Tabnas({ plugins: [myGrammar] })
 *   makeRunner({ parse: (src) => tn.parse(src) })
 *     .file(Path.join(findSpecDir(__dirname), 'happy.tsv'))
 *
 * THIS BARREL IS FOR TEST CODE. It re-exports `runner`, which imports
 * `node:test` — so importing '@tabnas/support' loads Node's whole test
 * runner (harness, reporters, mock loader) into the process. That is
 * free in a test file and wrong everywhere else: production code that
 * only wants the fixture format pays for a test runner it never uses,
 * and in a runtime without one (workerd, Deno Deploy, a browser) the
 * import cannot resolve at all.
 *
 * Non-test consumers import the piece they need instead:
 *
 *   '@tabnas/support/spec'    parseSpec, loadSpec, loadSpecDir, findSpecDir
 *   '@tabnas/support/expect'  parseExpect, equalValue, formatValue,
 *                             loneSurrogateAt, ...
 *   '@tabnas/support/escape'  escape, unescape
 *
 * Those three are runtime-safe: `escape` and `expect` are pure, and
 * `spec` reaches for `node:fs` only inside the file-loading functions.
 * Keep it that way — a `node:test` import must never become reachable
 * from any of them.
 */

export const VERSION = '0.3.2'

export { unescape, escape } from './escape'

export {
  SpecRow, SpecFile,
  parseSpec, loadSpec, loadSpecDir, findSpecDir,
} from './spec'
export type { SpecOptions } from './spec'

export {
  ERROR_PREFIX,
  isErrorExpect, errorCode, parseExpect, equalValue, formatValue,
  loneSurrogateAt, loneSurrogateMessage,
} from './expect'
export type { EqualOptions } from './expect'

// The test-runner half of the barrel: `node:test` enters HERE, and this
// re-export is the only reason '@tabnas/support' cannot be imported by a
// non-Node runtime. See the header note before adding another.
export { SpecRunner, makeRunner } from './runner'
export type { RunnerOptions } from './runner'

export { codesInSpecDir, compareCatalogues, coverage } from './census'
export type {
  CensusOptions, CatalogueDiff, CoverageReport,
} from './census'
