# Agents Guide — shared spec fixtures

`spec/**/*.tsv` holds this repo's cross-runtime fixtures. Both runtimes
run the same files, so a change here affects TypeScript and Go together —
edit with that in mind.

Unlike the other tabnas repos, these fixtures test the **fixture
machinery** rather than a parser. The format they use is the format this
package defines, so this directory is also the worked example: what is
written here is what `@tabnas/support` and `github.com/tabnas/support/go`
promise to read everywhere else.

## Layout

`spec/` is split by what a family drives, because the families have
genuinely different column shapes and no single runner could sweep them
all:

| Path | What it drives |
|---|---|
| `spec/adder/` | The adder grammar, through the full runner. `input` + `expected`, the ordinary two-column shape every other tabnas repo uses. |
| `spec/util/` | The utilities themselves — the escape codec, expectation parsing, value comparison, and the loader's own row-shape rules. |
| `spec/census/` | The census helpers — which expectation cells `codesInSpecDir` / `CodesInSpecDir` counts as codes, and how the expectation column is selected. |

## Format

Tab-separated, one case per line, with a header row naming the columns.

- Blank lines are skipped.
- A line starting with `#` that contains **no tab** is a comment and is
  skipped. A data row always has at least one tab, so a `#`-leading source
  such as a C preprocessor directive is still data.
- Columns are read **raw**; escape decoding is explicit and per column.
- `\n`, `\r`, `\t` and `\\` are the escape set. Every other backslash
  sequence passes through unchanged.

The full rules are in [`doc/reference.md`](../doc/reference.md).

## Escaping is not automatic — mind which column

The `input` column is escape-decoded; the `expected` column is **not**. It
is JSON, and JSON has its own escape rules: the two characters `\n` inside
a JSON string are a newline *by JSON's rules*, and decoding the cell first
would put a real newline inside the quotes, which is not valid JSON.

This was previously a genuine hazard rather than a convention — in
`@tabnas/parser` and `@tabnas/jsonic` the TypeScript loader decoded every
column while the Go loader decoded only the first, so an escape outside
`input` meant two different things in the two runtimes, and
`jsonic/test/AGENTS.md` told authors to keep escapes in `input` to work
around it. Both loaders here now decode nothing until asked. To carry a
decoded value in a non-input column, write it as JSON (see
`util/codec.tsv`, whose `value` and `escaped` columns do exactly that).

## Expected values

Either a JSON value the parse must produce, or `ERROR:<code>` for input
that must be rejected with that code. A bare `ERROR` accepts any code.

The code is part of the contract, not just "it threw": two runtimes that
reject the same source for different reasons have not agreed on anything.

Write `null` rather than leaving a cell empty when the value really is
null. An empty cell means "no value", and TypeScript reads that as
`undefined` while Go — which has no `undefined` — reads it as `nil`.

Two things to know about numbers in an expected cell: one beyond float64
range (`1e400`) reads as `Infinity` in both runtimes, and an integer
beyond 2^53 is **inexact** in both — `9007199254740993` reads as
`...992`, so do not pin one and expect either side to tell it from its
neighbour. See [`doc/reference.md`](../doc/reference.md).

## Who runs what

| Fixture | TypeScript | Go |
|---|---|---|
| `spec/adder/*.tsv` | `ts/test/adder.test.js` | `go/adder/adder_test.go` |
| `spec/util/codec.tsv` | `ts/test/codec.test.js` | `go/escape_test.go` |
| `spec/util/expect-error.tsv` | `ts/test/expect.test.js` | `go/expect_test.go` |
| `spec/util/value-equal.tsv` | `ts/test/expect.test.js` | `go/expect_test.go` |
| `spec/util/loader-rows.tsv` | `ts/test/spec.test.js` | `go/spec_test.go` |
| `spec/census/codes.tsv` | `ts/test/census.test.js` | `go/census_test.go` |
| `spec/census/named-col.tsv` | `ts/test/census.test.js` | `go/census_test.go` |

`spec/adder/` is discovered by directory listing in both runtimes: adding a
`.tsv` there runs it in both without touching either runner. The `util/`
and `census/` fixtures are named explicitly, because each has its own
column shape.

**A `util/` or `census/` fixture must be wired into both runtimes, and
the census tests check it** (`go/census_test.go`, `ts/test/census.test.js`):
each asserts that every `*.tsv` in those directories is named somewhere in
its own test sources, so adding a fixture and wiring up one side turns the
other side red. A row only one runtime runs is agreed by nobody, which is
the one thing this directory exists to prevent.

`spec/util/loader-rows.tsv` is a **layout** fixture: both runtimes assert
its rows sit on specific physical line numbers. Do not reflow it, and do
not add or remove lines above the data without updating both assertions.

## Rules

- Prefer adding a fixture here over a one-off in-language assertion when a
  case is expressible as input → output. That is what keeps the two
  runtimes honest against each other.
- TypeScript is canonical. If the two runtimes disagree, the TS behaviour
  is the expected value — unless Go has exposed a genuine TS defect, in
  which case fix TS first and pin the corrected behaviour here.
- A new fixture must pass in BOTH runtimes: run `make test` (or `go test
  ./...` from `go/` and `go/adder/`, and `npm test` from `ts/`) before
  considering it done.
- Some behaviour cannot be written down here — `NaN`, an explicit
  `undefined` key, Go's numeric types. Those stay as in-language cases
  next to the fixture-driven ones, and both runtimes carry their own.
