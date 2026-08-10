# Reference

The fixture format, and the full API in both runtimes side by side. The
two are written to behave identically; where a difference is unavoidable
it is marked **⚠ differs** and explained.

- TypeScript: `@tabnas/support`, source in [`../ts/src/`](../ts/src/).
- Go: `github.com/tabnas/support/go`, source in [`../go/`](../go/).

## The fixture format

A fixture is a tab-separated file, one case per line.

```
input	expected
{a:1}	{"a":1}
a,	["a"]
{	ERROR:unexpected
```

| Line | Meaning |
|---|---|
| Line 1 | **Header**, naming the columns. A row can then be read by name rather than by position. |
| Blank | Skipped. |
| `#` with no tab | **Comment**, skipped. |
| `#` **with** a tab | **Data.** A data row always has at least one tab, so a fixture whose input is a C preprocessor directive — or a comment in the parsed language — still works. |
| Anything else | Data. |

Columns are returned **raw**. Escape decoding is per column and explicit,
because the `expected` column is normally JSON, which carries its own
escape rules and must not be decoded twice: the two characters `\n` inside
a JSON string are a newline *by JSON's rules*, and decoding the cell first
would put a real newline inside the quotes, which is not valid JSON.

Line numbers reported are the **physical 1-based line** in the file, so a
failure message points an editor straight at the row.

A leading UTF-8 BOM is stripped — otherwise it becomes part of the first
column's name and every lookup by that name fails for no visible reason.
CRLF and LF line endings mean the same thing.

### The escape codec

A TSV cell cannot hold a raw tab (a column separator) or a raw newline (a
row separator). Fixtures write those as escapes:

| Escape | Decodes to |
|---|---|
| `\n` | newline |
| `\r` | carriage return |
| `\t` | tab |
| `\\` | backslash |
| anything else | **itself, unchanged** — `\q` stays `\q` |

Passing an unrecognised escape through is what lets a fixture carry its own
backslashes — a regex, a Windows path, a JSON string escape — without a
second layer of quoting. Decoding is left to right, so `\\n` is an escaped
backslash followed by the letter `n`, not a newline.

Encoding is the exact inverse: `unescape(escape(s)) === s` for every `s`.

### The expected column

Either a **JSON value** the parse must produce, or an **error** the parse
must raise:

```
ERROR                 -- must fail, with any code
ERROR:unexpected      -- must fail, with exactly this code
```

The cell counts as an error expectation only when it is exactly `ERROR` or
starts with `ERROR:`. A bare `startsWith('ERROR')` test would read a
legitimate `ERRORS` expected value as a failure expectation and then never
compare the parse result at all — a row that silently tests nothing.

The **code** is part of the contract. Two runtimes that reject the same
input for different reasons have not agreed on anything.

An empty `expected` cell means "no value".

> **⚠ differs.** TypeScript reads an empty cell as `undefined` and `null`
> as `null`; Go has no `undefined`, so both are `nil`. In a cross-runtime
> fixture, write `null` explicitly rather than leaving the cell empty.

## Escape codec API

| TypeScript | Go |
|---|---|
| `unescape(src: string): string` | `Unescape(src string) string` |
| `escape(src: string): string` | `Escape(src string) string` |

## Loading

### Types

| TypeScript | Go |
|---|---|
| `SpecRow` | `Row` |
| `SpecFile` | `File` |
| `SpecOptions` | `Options` |

`SpecFile` / `File` fields:

| TypeScript | Go | Meaning |
|---|---|---|
| `file` | `Name` | Base name (`happy.tsv`). |
| `path` | `Path` | Path it was read from; `''` when parsed from text. |
| `header` | `Header` | Column names. |
| `rows` | `Rows` | Data rows, in file order. |

`SpecRow` / `Row` fields:

| TypeScript | Go | Meaning |
|---|---|---|
| `file` | `File` | Base name of the file this row came from. |
| `line` | `Line` | Physical 1-based line number. |
| `index` | `Index` | 0-based position among **data** rows. |
| `cols` | `Cols` | The columns, raw. |
| `header` | `Header` | The file's column names. |

`SpecRow` / `Row` methods:

| TypeScript | Go | Meaning |
|---|---|---|
| `col(i)` | `Col(i)` | Raw column by position; `''` out of range. |
| `unesc(i)` | `Unesc(i)` | Escape-decoded column by position. |
| `named(name)` | `Named(name)` | Raw column by header name; `''` if absent. |
| `unescNamed(name)` | `UnescNamed(name)` | Escape-decoded column by header name. |
| `index_of(name)` | `IndexOf(name)` | Position of a header name, or `-1`. |
| `resolve(sel)` | — | Resolve a position **or** a name to a position; throws on an unknown name. |
| `where()` | `Where()` | `<file>:<line>`, for a failure message. |

> **⚠ differs.** TypeScript's `resolve` takes `number | string`, which Go
> has no equivalent for; a Go caller uses `Col` or `Named` directly, and
> `Runner` takes the position and the name as separate fields.

Reading a column out of range gives `''` rather than an error: a fixture
with a trailing optional column should not need a length check at every
use. Reading an **unknown header name** through `resolve` / `Runner`
*is* an error — that is a defect in the caller, not a missing value, and
silently reading column `-1` would compare the input against itself.

### Options

| TypeScript | Go | Default | Meaning |
|---|---|---|---|
| `header?: boolean` | `Header *bool` | true | Line 1 names the columns. |
| `comment?: boolean` | `Comment *bool` | true | Skip `#`-leading lines with no tab. |
| `minCols?: number` | `MinCols int` | 1 | Reject a data row with fewer columns. |

Go uses pointers so "false" is distinguishable from "not set" — the same
convention the parser's option structs use. `Bool(false)` builds one.

### Functions

| TypeScript | Go |
|---|---|
| `parseSpec(file, text, options?)` | `ParseSpec(name, text string, opts *Options) (*File, error)` |
| `loadSpec(path, options?)` | `LoadSpec(path string, opts *Options) (*File, error)` |
| `loadSpecDir(dir, options?)` | `LoadSpecDir(dir string, opts *Options) ([]*File, error)` |
| `findSpecDir(from?)` | `FindSpecDir(from string) (string, error)` |

`loadSpecDir` takes every `*.tsv` in a directory, sorted by name so both
runtimes and successive runs visit them in the same order. Discovery by
listing is deliberate: adding a fixture then runs it without editing a
runner.

`findSpecDir` walks up from `from` until it finds a `test/spec` directory.
This replaces the `join(__dirname, '..', '..', 'test', 'spec')` that every
repo hard-codes — a relative hop that has to be recounted whenever a test
moves a directory, and that is spelt differently in `go/` anyway. An empty
`from` (Go) or an omitted one (TypeScript) starts at the working
directory, which is the package directory under `go test` and the `ts/`
directory under `npm test`.

> **⚠ differs.** TypeScript throws; Go returns an `error`. Each is its
> language's convention, and every message carries the same text.

## Expectation API

| TypeScript | Go |
|---|---|
| `ERROR_PREFIX` | `ErrorPrefix` |
| `isErrorExpect(expected): boolean` | `IsErrorExpect(expected string) bool` |
| `errorCode(expected): string` | `ErrorCode(expected string) (string, error)` |
| `parseExpect(expected): unknown` | `ParseExpect(expected string) (any, error)` |
| `equalValue(got, expected, options?)` | `EqualValue(got, expected any) bool` |
| — | `EqualValueWith(got, expected any, normalize func(any) any) bool` |
| `formatValue(val): string` | `FormatValue(val any) string` |

`equalValue` compares with **JSON semantics**, not the host language's:

- Structural, and key-order independent.
- `-0` equals `0`. The `expected` column is written with JavaScript
  `JSON.stringify` semantics, which renders `-0` as `0`.
- `NaN` equals itself, which `===` and `==` do not. A fixture cannot
  express `NaN` — JSON has none — but a grammar can produce one.
- **Go only:** an integer equals the float of the same magnitude, and any
  slice or string-keyed map compares by contents. The expected side always
  arrives from `encoding/json` as `float64`, while a grammar's result can
  be any numeric type; without this, every integer row would fail for the
  Go runtime alone.
- A number is never a string, and a container of one kind is never a
  container of another — empty or not.

The `normalize` hook rewrites every node on both sides, outermost first.
This is where a runtime-specific container — an insertion-ordered map, a
reference wrapper — is unwrapped into the plain value the fixture's JSON
describes.

## Runner API

TypeScript builds a runner with `makeRunner(options)` (or
`new SpecRunner(options)`); Go uses a `Runner` struct literal.

| TypeScript | Go | Meaning |
|---|---|---|
| `parse` | `Parse` | Parse one input. **Required.** TS throws on rejection; Go returns an `error`. |
| `errorCode` | `ErrorCode` | Extract the code from a failure. Optional. |
| `normalize` | `Normalize` | Rewrite values before comparison. Optional. |
| `input` | `Input` / `InputName` | Input column. Default 0. |
| `expected` | `Expected` / `ExpectedName` | Expected column. Default 1. |
| `spec` | `Load` | Loader options. |
| `caseName` | `CaseName` | Name a test case. Default `row <line>: <input>`. |

Go's `Input` and `Expected` are `*int` because 0 is a real column: a plain
`int` could not tell "column 0" from "not set", and the one it would have
to guess wrong is the default. `Int(0)` builds one.

The default `errorCode` reads `err.code` in TypeScript, and in Go reads a
`Code() string` method or a `Code` string field — which is what
`*tabnas.TabnasError` carries, read by shape so this module needs no
dependency on the parser.

| TypeScript | Go | Runs |
|---|---|---|
| `runner.dir(dir)` | `Runner.Dir(t, dir)` | Every `*.tsv` in a directory. |
| `runner.file(path)` | `Runner.File(t, path)` | One fixture file. |
| `runner.spec(spec)` | `Runner.Spec(t, spec)` | An already-loaded fixture. |
| `runner.row(row, input, expected)` | `Runner.Row(t, row, input, expected)` | One row. |
| — | `Runner.CheckRow(row, input, expected) error` | One row, returning the failure instead of reporting it. |

An empty fixture and an empty directory both **fail**. A fixture that
loads but holds nothing is a silent pass, and a silent pass is
indistinguishable from coverage that was never there.

## The adder grammar

A plugin, in both runtimes, holding the integer-addition grammar from the
`@tabnas/parser` README:

```
val = add            -- `val` holds the running total
add = NR [ PL add ]  -- each number adds to it; `+` repeats
```

| TypeScript | Go |
|---|---|
| `require('@tabnas/support/adder').adder` | `github.com/tabnas/support/go/adder` — `adder.Adder`, `adder.Make()` |

```js
const { Tabnas } = require('@tabnas/parser')
const { adder } = require('@tabnas/support/adder')

const tn = new Tabnas({ plugins: [adder] })
tn.parse('1+2+3')   // => 6
```

```go
tn, _ := adder.Make()
tn.Parse("1+2+3")   // => 6
```

`val` opens by pushing `add` with the total at 0. `add` opens on a number
and adds it to its parent's node, then closes on `+` by **replacing**
itself with another `add` — a repeat at the same stack depth, so `1+2+...`
of any length runs in one frame and every iteration's parent is still
`val`, where the total lands.

The Go plugin is a separate module. The support module is a dependency of
every tabnas repo, so it carries none of its own; the grammar that
exercises it needs the parser, and splitting them is what lets both facts
hold.

> **⚠ differs.** TypeScript has one number type; Go's `#NR` token value is
> widened to `float64` by an unexported helper, so the total is one type
> throughout.
