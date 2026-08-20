# Reference

The fixture format, and the full API in both runtimes side by side. The
two are written to behave identically; where a difference is unavoidable
it is marked **⚠ differs** and explained. There are six, and adding a
seventh without documenting it silently breaks the guarantee the package
exists to provide.

- TypeScript: `@tabnas/support`, source in [`../ts/src/`](../ts/src/).
- Go: `github.com/tabnas/support/go`, source in [`../go/`](../go/).

Neither belongs in a release artifact: `@tabnas/support` is a
`devDependency` imported only from `test/`, and its Go half is imported
only from `_test.go` files. See
[keeping it out of your build](../go/README.md#keeping-it-out-of-your-build).

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

### Numbers in the expected column

Two limits, both inherited from the canonical runtime rather than
invented here:

- **Beyond float64 range reads as ±Infinity.** `1e400` is `Infinity`,
  which is what `JSON.parse` answers. Go's `encoding/json` rejects the
  literal outright, so `ParseExpect` re-reads such a cell keeping the
  number as text and widens it with `strconv` — otherwise a fixture row
  would run in TypeScript and fail to *load* in Go. `Infinity` compares
  equal to itself, so an overflow row can be pinned.
- **Integers beyond 2^53 are not exact, in either runtime.**
  `JSON.parse('9007199254740993')` is `9007199254740992`, and Go reads it
  the same way. Do not pin such an integer in a fixture and expect either
  side to tell it from its neighbour. Making Go exact here would make it
  *reject* rows TypeScript accepts, which is the divergence this package
  exists to prevent.

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

`loadSpecDir` takes every `*.tsv` **file** in a directory, sorted by name
so both runtimes and successive runs visit them in the same order.
Discovery by listing is deliberate: adding a fixture then runs it without
editing a runner. A directory whose own name ends in `.tsv` is skipped,
not read as a file.

A directory holding no `.tsv` files is an **error**, not an empty list.
That is the silent-pass failure mode one level up: a runner over an empty
directory reports green having run nothing.

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
| `loneSurrogateAt(cell): number` | `LoneSurrogateAt(cell string) int` |
| `loneSurrogateMessage(cell, at): string` | `LoneSurrogateMessage(cell string, at int) string` |

### A shared expected cell cannot hold a lone surrogate

The runner **refuses** a value cell containing an unpaired `\uXXXX`
surrogate escape, naming the row — **on the default path only**. A suite
supplying `parseExpected` / `ParseExpected` has a wider vocabulary than
JSON, which need not read `\uXXXX` as an escape at all, so the check
stands aside; call `loneSurrogateAt` from the hook if the syntax does use
JSON escapes.

The position is reported in **code points of the raw cell text** — the
one unit the two ports agree on without conversion, since the natural
index is a UTF-16 offset in TypeScript and a byte offset in Go. Counted
over the raw text, so a written-out surrogate pair spells twelve
characters.

The two runtimes decode one differently, and neither is wrong:
`JSON.parse` preserves it, because a JavaScript string is UTF-16 and may
hold one; Go's `encoding/json` replaces it with U+FFFD, because a Go
string is UTF-8 and cannot. Measured on the same cells:

| cell | TypeScript | Go |
|---|---|---|
| `"\ud800"` | 1 unit, `d800` | 3 bytes, `ef bf bd` |
| `"a\ud800b"` | `61 d800 62` | `61 ef bf bd 62` |
| `"\ud83d\ude00"` | `d83d de00` | `f0 9f 98 80` — **agree** |

So a shared cell holding one asks the two runtimes different questions
and **both pass**. It is the one thing a shared expected column cannot
express, and it fails silently, which is why this is an error rather than
something to notice later.

A surrogate **pair** is fine — both runtimes decode it to the same
character. An `ERROR:` cell is unaffected: it carries no JSON.

Where the case *does* belong: a **per-runtime register column**, where
each decoding is written out and each runtime reads its own; or each
port's own suite, asserting opposite results. `loneSurrogateAt` is
exported so a suite with its own runner can apply the same rule.

Only the escape form is detected, because it is the only one that can
occur: a fixture file is UTF-8, and a lone surrogate has no UTF-8
encoding, so it cannot appear literally in one.

`equalValue` compares with **JSON semantics**, not the host language's:

- Structural, and key-order independent. **Map key order is not part of
  the parsed-value contract** (ADR-15). TypeScript cannot preserve
  integer-like key order in a plain object — that is ECMAScript's own
  property-ordering rule, not a porting choice — so a fixture must not
  depend on it, and a difference in it between two ports is a recorded
  difference rather than a defect.
- `-0` does **not** equal `0` (ADR-15). Signed zero is representable and
  distinguishable in both runtimes, and a parser that reports `0` for the
  input `-0` has lost information the source carried, so `-0` is pinnable
  in a fixture. Numbers compare by IEEE bits; every other finite double
  has a unique bit pattern, so for those this is exactly `===` / `==`.
  `formatValue` spells `-0` as `-0` **at every depth**, because
  `JSON.stringify` renders it `0` and a nested mismatch would otherwise
  report "got [0], expected [0]". Go's `json.Marshal` already writes `-0`,
  so only the TypeScript formatter needed the work — and without it the two
  runtimes would disagree about their own diagnostics.
- **Go only:** a *defined* numeric type (`type Number float64` in a
  grammar's own package) is compared as the number it is, by kind rather
  than by exact type. Without that it fell through to `reflect.DeepEqual`,
  where `Number(1)` did not equal `1.0` — so every numeric row failed for
  such a grammar — and `Number(-0)` equalled `Number(0)`, so the signed-zero
  contract was not enforced for it. Same choice the map comparison makes for
  a defined string key type.
- `NaN` equals itself, which `===` and `==` do not. A fixture cannot
  express `NaN` — JSON has none — but a grammar can produce one.
- **Go only:** an integer equals the float of the same magnitude, and any
  slice or string-keyed map compares by contents. The expected side always
  arrives from `encoding/json` as `float64`, while a grammar's result can
  be any numeric type; without this, every integer row would fail for the
  Go runtime alone. A map keyed by a *defined* string type
  (`map[TokenName]any`) compares against a `map[string]any` expectation;
  a `map[int]any` does not, however happily Go would convert the key.
- A number is never a string, and a container of one kind is never a
  container of another — empty or not.
- **Own keys only.** An object's inherited properties take no part: a
  result keyed `constructor` or `valueOf` is compared as the ordinary key
  it is, not matched against every object that inherits one. Relaxed
  grammars parse those keys perfectly happily — that is what the
  `funky-keys` fixtures are for.

The `normalize` hook rewrites every node on both sides, outermost first.
This is where a runtime-specific container — an insertion-ordered map, a
reference wrapper — is unwrapped into the plain value the fixture's JSON
describes.


## The divergence register

Where the two ports of one grammar are known to **disagree**, and the
difference has been argued rather than repaired (ADR-14).

```js
makeRegister({ parse, runtime: 'ts', runtimes: ['ts', 'go'] })
  .file(Path.join(specDir, 'divergent.tsv'))
```

```go
support.Register{
    Runner:   support.Runner{Parse: parse},
    Runtime:  "go",
    Runtimes: []string{"ts", "go"},
}.File(t, filepath.Join(specDir, "divergent.tsv"))
```

The fixture has an `input` column and **one column per runtime**, each
written in the ordinary expected vocabulary — a JSON value, or
`ERROR:<code>`:

```
input	ts	go
"\uD800"	"\ud800"	"\ufffd"
```

Both suites run the same file and read different columns of it.

### Why this is not just a fixture

A fixture fails when behaviour **regresses**. A register also fails when the
divergence is **fixed**.

When a port is repaired to agree with the other, the register still claims
they differ, so the suite goes red and names the row to delete:

```
divergent.tsv:12: this divergence is CLOSED. go now produces what the ts
column records ("A"), not its own ("a").
  This is the register working: a fixed divergence fails as loudly as a
  regressed one, so the row cannot outlive it.
  DELETE this row. Do not edit it to match — that would record a divergence
  that no longer exists, which is what this mechanism exists to prevent.
```

That distinction is the whole point. A regression and a repair produce the
same red build from a plain fixture, and the message sends the reader to
the opposite conclusion in one of the two cases.

The 2026-08 fleet audit found **29 recorded divergence claims contradicted
by execution**, and one file that had been wrong in *both* directions at
once. Prose does not hold, because nothing runs it. Neither does a
divergence recorded only as a passing test of current behaviour, because a
fix leaves it passing while it describes something that no longer happens.

### Rules the register enforces

- **A row must record a disagreement.** If every runtime column says the
  same thing, the row asserts nothing and would pass forever — the shape of
  the claims this replaces. It fails.
- **Runtimes are named, not inferred** from the header, so a `note` or
  `issue` column is not silently read as a runtime that "agrees" with a
  sentence. Every named column must exist.
- **A regression still reports as a mismatch**, not as a closed divergence.
- **Comparison is the runner's**, unchanged. A register must not develop
  its own idea of what "equal" means.

`noDivergences(where)` / `NoDivergences(t, where)` declares that a repo has
none, so the claim appears in the test output rather than being inferred
from a file nobody notices is missing.


## Runner API

TypeScript builds a runner with `makeRunner(options)` (or
`new SpecRunner(options)`); Go uses a `Runner` struct literal.

| TypeScript | Go | Meaning |
|---|---|---|
| `parse` | `Parse` | Parse one input. **Required.** TS throws on rejection; Go returns an `error`. |
| `parse` (2nd arg) | `ParseRow` | Parse one input, given the row as well. |
| `errorCode` | `ErrorCode` | Extract the code from a failure. Optional. |
| `matchError` | `MatchError` | Decide whether a failure satisfies `ERROR:<want>`, when a code cannot. Optional. |
| `parseExpected` | `ParseExpected` | Read the expected cell, when the fixture's vocabulary is wider than JSON. Optional. |
| `normalize` | `Normalize` | Rewrite values before comparison. Optional. |
| `input` | `Input` / `InputName` | Input column. Default 0. |
| `expected` | `Expected` / `ExpectedName` | Expected column. Default 1. |
| `spec` | `Load` | Loader options. |
| `caseName` | `CaseName` | Name a test case. Default `row <line>: <input>`. |

Go's `Input` and `Expected` are `*int` because 0 is a real column: a plain
`int` could not tell "column 0" from "not set", and the one it would have
to guess wrong is the default. `Int(0)` builds one.

The row is handed to the parse hook because a fixture's other columns can
take part in the parse — an `opts` column of plugin options is the common
one, and a runner that could not see it would leave every such repo
writing its own loop again.

> **⚠ differs.** TypeScript's `parse` simply takes the row as a second
> argument, which a caller who does not want it leaves off. Go has no
> optional parameter, so the row-taking form is the separate field
> `ParseRow`; folding the row into `Parse` would make every simple suite —
> the majority — write an ignored `_ *Row` and give up passing a parser's
> own method as the hook. Set one or the other: setting both is an error,
> not a precedence rule, because the two say different things about the
> same row and running one quietly would hide that the other never ran.

The default `errorCode` reads `err.code` in TypeScript, and in Go reads a
`Code() string` method or a `Code` string field — which is what
`*tabnas.TabnasError` carries, read by shape so this module needs no
dependency on the parser.

`matchError` is the escape hatch for a grammar with no stable code to pin
— one whose failures are distinguished only by their message, or a fixture
that names a position (`ERROR:1:8`) rather than a kind. It is handed the
error, the wanted text and the row, and it **replaces** the code
comparison, so `errorCode` is not consulted by a runner that sets it. A
bare `ERROR` still means "any error" and does not reach the hook.

Prefer a code where there is one. Matching a message pins less: two
runtimes whose messages happen to share a substring have agreed on less
than two that answer the same code, and a message is the thing most likely
to be reworded. The hook exists so such a fixture can keep asserting
*something* specific rather than weakening to a bare `ERROR`, which
asserts only that it failed.

`parseExpected` widens the expected cell's vocabulary. JSON is what the
cell should be wherever it can be — it is the one notation both runtimes
already agree on — but some grammars produce values JSON cannot spell:
JSON5's `NaN` and `Infinity`, and the `UNDEFINED` several repos use for
"the parse yielded no value at all", which is a *different result* from
`null`. Without the hook those fixtures could not pin the distinction they
exist to pin. It replaces `parseExpect`, so call `parseExpect` for the
cells the hook does not claim, and it is reached only for a value row —
an `ERROR` cell is an error expectation before it is anything else.

| TypeScript | Go | Runs |
|---|---|---|
| `runner.dir(dir)` | `Runner.Dir(t, dir)` | Every `*.tsv` in a directory. |
| `runner.file(path)` | `Runner.File(t, path)` | One fixture file. |
| `runner.spec(spec)` | `Runner.Spec(t, spec)` | An already-loaded fixture. |
| `runner.row(row, input, expected)` | `Runner.Row(t, row, input, expected)` | One row. |
| `runner.checkSpec(spec)` | `Runner.CheckSpec(spec) error` | Nothing — reports whether a fixture *can* be run. |
| — | `Runner.CheckRow(row, input, expected) error` | One row, returning the failure instead of reporting it. |

An empty fixture and an empty directory both **fail**. A fixture that
loads but holds nothing is a silent pass, and a silent pass is
indistinguishable from coverage that was never there.

`checkSpec` / `CheckSpec` is that guard, split out so it can be asserted:
reporting through `node:test` or `*testing.T` cannot be caught by a test,
so a guard that only ever failed a test could not itself be pinned. It
also rejects a misspelt column name at registration time, rather than as
one red case per row. `spec` / `Spec` calls it first.

> **⚠ differs.** TypeScript's checks throw; Go's return an `error`. Same
> split as `row` / `CheckRow`, and each is its language's convention.

## Census API

Coverage and parity tripwires over error codes, for a consuming repo to
run against its own data. Every input arrives as an argument — this
package fetches no catalogue and imports no engine, because it depends
on nothing and never will.

| TypeScript | Go |
|---|---|
| `codesInSpecDir(dir, options?): string[]` | `CodesInSpecDir(dir string, opts CensusOpts) ([]string, error)` |
| `compareCatalogues(a, b): CatalogueDiff` | `CompareCatalogues(a, b map[string]string) CatalogueDiff` |
| `coverage(declared, exercised): CoverageReport` | `Coverage(declared, exercised []string) CoverageReport` |

`codesInSpecDir` walks a fixture directory with the shared loader
(`loadSpecDir`, so a missing or empty directory is an **error**, not "no
codes") and returns the codes its expectation cells exercise: sorted,
unique. Only a **code-style** cell counts — `ERROR:` followed by a bare
`[a-z][a-z0-9_]*` token. A message-style expectation (`ERROR:bad token`,
`ERROR:1:8`) and a bare `ERROR` assert a rejection without naming a
code, and returning them would count coverage that is not there.

The expectation column defaults to each **row's last column**; select it
with `col` / `Col` (position) or `name` / `Name` (header name, which
wins when set). An unknown name is an error — the same caller defect
`resolve` and the runner report. Go's `Col` is a `*int` for the same
reason `Runner.Input` is: 0 is a real column, and a plain `int` could
not tell it from "not set". `Int(0)` builds one.

`compareCatalogues` diffs two `{code: template}` maps — message
catalogues, hint catalogues, or one runtime's against the other's:

| TypeScript | Go | Meaning |
|---|---|---|
| `missing` | `Missing` | Keys of `a` absent in `b`. |
| `extra` | `Extra` | Keys of `b` absent in `a`. |
| `templateMismatch` | `TemplateMismatch` | Shared keys whose templates differ **byte for byte**. |

Byte-for-byte is deliberate: two templates that merely "mean the same"
have still drifted, and the byte diff is what a maintainer has to
reconcile.

`coverage` compares the codes a package declares against the codes its
fixtures exercise (typically `codesInSpecDir`'s answer — whether
inherited base codes count as declared is the caller's choice):

| TypeScript | Go | Meaning |
|---|---|---|
| `uncovered` | `Uncovered` | Declared, but exercised by no fixture. |
| `orphan` | `Orphan` | Exercised, but declared by nobody. |

Every list in every census answer is sorted **by code point**, and empty
rather than absent (`[]` in TypeScript, a non-nil empty slice in Go)
when there is nothing to report, so the two runtimes render the same
answer the same way. Code point is deliberate: Go's `sort.Strings`
compares UTF-8 bytes, which for valid UTF-8 *is* code-point order, while
TypeScript's default sort compares UTF-16 code units and would order a
non-BMP catalogue key differently — so the TypeScript census supplies
its own comparator instead.

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
