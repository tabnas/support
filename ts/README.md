# @tabnas/support

Shared test-support utilities for the [tabnas](https://github.com/tabnas)
parser system: the TSV spec-fixture loader, the escape codec, the
expectation helpers and the cross-runtime test runner.

This is the TypeScript half. The Go half is
[`github.com/tabnas/support/go`](../go/), and the two are written to behave
identically — same escape codec, same comment and blank-line handling, same
`ERROR:<code>` contract, same value comparison. That is the point: every
tabnas package proves its two runtimes agree by running **one** set of TSV
fixtures in **both**, and a loader that disagreed with its twin would make
those fixtures prove nothing.

## Install

```bash
npm install --save-dev @tabnas/support
```

`@tabnas/parser` is an optional peer dependency, needed only for the
[adder grammar](#the-adder-grammar).

## Use

```js
const Path = require('node:path')
const { Tabnas } = require('@tabnas/parser')
const { findSpecDir, makeRunner } = require('@tabnas/support')

const tn = new Tabnas({ plugins: [myGrammar] })

makeRunner({ parse: (src) => tn.parse(src) })
  .dir(Path.join(findSpecDir(__dirname), 'happy'))
```

`findSpecDir` walks up until it finds a `test/spec` directory, so a suite
does not hard-code how many levels up the fixtures are. The runner loads
every `.tsv` in the named directory and emits one `node:test` case per row,
reporting with the fixture's own file name and line number.

One fixture file, rather than a directory:

```js
makeRunner({ parse: (src) => tn.parse(src) })
  .file(Path.join(findSpecDir(__dirname), 'happy.tsv'))
```

Reading fixtures without the runner:

```js
const { loadSpec, parseExpect, equalValue } = require('@tabnas/support')

const spec = loadSpec(Path.join(specDir, 'utility-str.tsv'))
for (const row of spec.rows) {
  const got = str(row.unesc(0), Number(row.named('maxlen')))
  assert.ok(equalValue(got, parseExpect(row.named('expected'))), row.where())
}
```

Columns come back **raw**; `unesc` decodes the escape set on the columns
that need it. The `expected` column normally does not: it is JSON, which
carries its own escapes.

## The adder grammar

```js
const { Tabnas } = require('@tabnas/parser')
const { adder } = require('@tabnas/support/adder')

const tn = new Tabnas({ plugins: [adder] })
tn.parse('1+2+3')   // => 6
tn.parse('10+20')   // => 30
```

The integer-addition grammar from the `@tabnas/parser` README, packaged as
a plugin. It is the smallest grammar that is still a real one — two rules,
one custom token, a push and a repeat — which makes it this package's
end-to-end check: `test/adder.test.js` and `go/adder/adder_test.go` run it
against the same `test/spec/adder/*.tsv` rows in both runtimes.

## API

Full signatures, and the Go equivalents, are in the
[reference](../doc/reference.md).

| | |
|---|---|
| `unescape` `escape` | The fixture escape codec: `\n` `\r` `\t` `\\`. |
| `loadSpec` `loadSpecDir` `parseSpec` `findSpecDir` | Loading fixtures. |
| `SpecRow` `SpecFile` `SpecOptions` | What loading gives back. |
| `isErrorExpect` `errorCode` `parseExpect` | Reading the `expected` column. |
| `equalValue` `formatValue` | Comparing and reporting. |
| `makeRunner` `SpecRunner` | Rows in, `node:test` cases out. |
| `VERSION` | Kept in step with the Go module. |

## Development

```bash
npm run build
npm test
```

## License

MIT. Copyright (c) Richard Rodger.
