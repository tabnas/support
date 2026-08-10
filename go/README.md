# support (Go)

Version: 0.1.0

Shared test-support utilities for the [tabnas](https://github.com/tabnas)
parser system: the TSV spec-fixture loader, the escape codec, the
expectation helpers and the cross-runtime test runner.

This is the Go half. The canonical TypeScript half is
[`@tabnas/support`](../ts/), and the two are written to behave identically
— same escape codec, same comment and blank-line handling, same
`ERROR:<code>` contract, same value comparison. That is the point: every
tabnas package proves its two runtimes agree by running **one** set of TSV
fixtures in **both**, and a loader that disagreed with its twin would make
those fixtures prove nothing.

**This module has no dependencies, and never will.** Every tabnas repo
depends on it, so anything it required would land in all of them —
including the parser it is used to test. The adder grammar that exercises
it end to end therefore lives in the separate [`adder`](adder/) module.

## Install

```bash
go get github.com/tabnas/support/go@latest
```

## Use

```go
package mygrammar

import (
	"path/filepath"
	"testing"

	support "github.com/tabnas/support/go"
)

func TestSpec(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}

	tn := makeGrammar(t)
	support.Runner{Parse: tn.Parse}.Dir(t, filepath.Join(dir, "happy"))
}
```

`FindSpecDir("")` walks up from the working directory — the package
directory under `go test` — until it finds a `test/spec` directory, so a
suite does not hard-code how many levels up the fixtures are. `Dir` loads
every `.tsv` there and runs one subtest per row, reporting with the
fixture's own file name and line number.

One fixture file, rather than a directory:

```go
support.Runner{Parse: tn.Parse}.File(t, filepath.Join(dir, "happy.tsv"))
```

Reading fixtures without the runner:

```go
spec, err := support.LoadSpec(filepath.Join(dir, "utility-str.tsv"), nil)
if err != nil {
	t.Fatal(err)
}

for _, row := range spec.Rows {
	want, err := support.ParseExpect(row.Named("expected"))
	if err != nil {
		t.Fatalf("%s: %v", row.Where(), err)
	}
	if got := Str(row.Unesc(0), maxlen(row)); !support.EqualValue(got, want) {
		t.Errorf("%s: got %s, want %s",
			row.Where(), support.FormatValue(got), support.FormatValue(want))
	}
}
```

Columns come back **raw**; `Unesc` decodes the escape set on the columns
that need it. The `Expected` column normally does not: it is JSON, which
carries its own escapes.

`EqualValue` compares with JSON semantics rather than Go's, which matters
most for numbers: the expected side always arrives from `encoding/json` as
`float64`, while a grammar's result can be any numeric type.
`reflect.DeepEqual` would fail every integer row for the Go runtime alone.

## The adder grammar

```go
import "github.com/tabnas/support/go/adder"

tn, _ := adder.Make()
tn.Parse("1+2+3")   // => 6
```

The integer-addition grammar from the `tabnas/parser` README, packaged as
a plugin in the separate [`adder`](adder/) module. It is this module's
end-to-end check: `adder/adder_test.go` and `ts/test/adder.test.js` run it
against the same `test/spec/adder/*.tsv` rows in both runtimes.

## API

Full signatures, and the TypeScript equivalents, are in the
[reference](../doc/reference.md).

| | |
|---|---|
| `Unescape` `Escape` | The fixture escape codec: `\n` `\r` `\t` `\\`. |
| `LoadSpec` `LoadSpecDir` `ParseSpec` `FindSpecDir` | Loading fixtures. |
| `Row` `File` `Options` | What loading gives back. |
| `IsErrorExpect` `ErrorCode` `ParseExpect` | Reading the expected column. |
| `EqualValue` `EqualValueWith` `FormatValue` | Comparing and reporting. |
| `Runner` | Rows in, `*testing.T` subtests out. |
| `Bool` `Int` | Pointer helpers for the option fields. |
| `VERSION` | Kept in step with `ts/package.json`. |

## Development

```bash
go test ./...          # this module
cd adder && go test ./...   # the adder module
```

## License

MIT. Copyright (c) Richard Rodger.
