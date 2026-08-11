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
 */

export const VERSION = '0.3.0'

export { unescape, escape } from './escape'

export {
  SpecRow, SpecFile,
  parseSpec, loadSpec, loadSpecDir, findSpecDir,
} from './spec'
export type { SpecOptions } from './spec'

export {
  ERROR_PREFIX,
  isErrorExpect, errorCode, parseExpect, equalValue, formatValue,
} from './expect'
export type { EqualOptions } from './expect'

export { SpecRunner, makeRunner } from './runner'
export type { RunnerOptions } from './runner'
