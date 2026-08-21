# support

Shared test-support utilities for the **tabnas** parser system.

Docs, guides, the error reference and the playground: **[tabnas.dev](https://tabnas.dev)**.

Every tabnas package ([parser](https://github.com/tabnas/parser),
[json](https://github.com/tabnas/json),
[jsonic](https://github.com/tabnas/jsonic),
[abnf](https://github.com/tabnas/abnf), ...) ships two implementations — a
canonical TypeScript one and a Go port — and proves they agree by running
**one** set of TSV fixtures from `test/spec/` in **both**. This repository
is the machinery that reads those fixtures.

| Runtime | Package |
|---|---|
| TypeScript | `@tabnas/support` — [`ts/`](ts/) |
| Go | `github.com/tabnas/support/go` — [`go/`](go/) |

## Why

The fixtures were already shared. The *loaders* were not — each repo
carried its own copy, and by the time there were three pairs of them they
had drifted:

| | parser | jsonic | json |
|---|---|---|---|
| decodes `\t` | TS ✅ Go ✅ | TS ❌ Go ✅ | ✅ |
| decodes `\\` | ❌ | ❌ | ✅ |
| skips `#` comment lines | ❌ | TS ✅ Go ✅ | ✅ |
| decodes escapes in | TS: every column<br>Go: `input` only | TS: every column<br>Go: `input` only | `input` only |

Two of those are more than untidiness. `jsonic`'s TypeScript loader not
decoding `\t` means a tab written in a shared fixture reached the Go
parser and not the TypeScript one. And decoding every column in one
runtime but only `input` in the other means an escape outside `input`
*means two different things in the two runtimes* —
`jsonic/test/AGENTS.md` documented this as a live hazard and told fixture
authors to keep escapes in the input column to avoid it.

A row that means two different things in two runtimes cannot pin agreement
on anything else. So there is one loader now, in two languages, written to
behave identically — following what `@tabnas/json`, the newest of the
three, already did — and a shared fixture set that tests the loaders
themselves, the same way the parsers' fixtures test the parsers.

## What's in it

Both runtimes expose the same four things:

- **The escape codec.** `\n`, `\r`, `\t` and `\\` decoded; every other
  backslash sequence passed through untouched, so a fixture can carry its
  own backslashes without a second layer of quoting.
- **The fixture loader.** Header row, blank lines, `#` comment lines,
  CRLF, BOM, raw columns with explicit per-column decoding, physical line
  numbers for failure messages, and directory discovery.
- **The expectation helpers.** `ERROR:<code>` parsing, `expected`-as-JSON
  parsing, and a JSON-semantics value comparison (key-order independent,
  `-0` NOT equal to `0`, `NaN` equal to itself, an integer equal to the
  float of the same magnitude).
- **The runner.** Fixture rows in, `node:test` / `*testing.T` subtests
  out, with the file and line in every failure message.
- **The divergence register.** Recorded TS/Go disagreements, executed by
  both ports, where a *fixed* divergence fails as loudly as a regressed one
  — so a row cannot outlive the difference it records.

## Depending on it without shipping it

This is test-support code; it should never reach a release artifact.

**TypeScript** — install it as a `devDependency` and import it only from
`test/`, so it is neither installed by consumers nor reachable from
`dist/`.

**Go** has no `devDependencies`, and does not need them: the guarantee
comes from the **import graph** instead of from metadata.

- Import it only from `_test.go` files. A package reached only from test
  files is never linked into `go build` output — not this module, and not
  the `testing` package it pulls in.
- Since Go 1.17, module graph pruning keeps a dependency's *test-only*
  dependencies out of everyone downstream, so importing your module does
  not give anyone this one.
- It does appear as a `require` line in your `go.mod`. That records what
  your tests need, not what your binary contains.

The rule is checkable, so check it — `go list -deps` walks the non-test
graph, so this prints nothing in a healthy repo:

```bash
go list -deps ./... | grep -x 'github.com/tabnas/support/go' && \
  echo 'LEAKED into the build graph' && exit 1
```

`go/adder` is a working demonstration: it uses the support module from
`adder_test.go` only, and neither the module nor `testing` appears in its
non-test import graph.

## Quick start

TypeScript:

```js
const Path = require('node:path')
const { Tabnas } = require('@tabnas/parser')
const { findSpecDir, makeRunner } = require('@tabnas/support')

const tn = new Tabnas({ plugins: [myGrammar] })

makeRunner({ parse: (src) => tn.parse(src) })
  .dir(Path.join(findSpecDir(__dirname), 'happy'))
```

Go:

```go
func TestSpec(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}

	tn := makeGrammar(t)
	support.Runner{Parse: tn.Parse}.Dir(t, filepath.Join(dir, "happy"))
}
```

Both walk up from where they are told to start until they find a
`test/spec` directory, run every `.tsv` in the named subdirectory, and
report per row with the fixture's own line numbers.

## The adder grammar

[`ts/src/adder.ts`](ts/src/adder.ts) and
[`go/adder/adder.go`](go/adder/adder.go) hold the integer-addition grammar
from the `@tabnas/parser` README (`1+2+3` => 6), as a plugin:

```
val = add            -- `val` holds the running total
add = NR [ PL add ]  -- each number adds to it; `+` repeats
```

It is the smallest grammar that is still a real one — two rules, one
custom token, a push and a repeat — which makes it the end-to-end check
that the two runtimes' utilities do the same thing. Both run it against
the same `test/spec/adder/*.tsv` rows, so a divergence anywhere in the
chain turns one of them red.

The Go plugin is a **separate module** (`github.com/tabnas/support/go/adder`).
The support module itself has no dependencies and never will: every tabnas
repo depends on it, so anything it required would land in all of them —
including the parser it is used to test.

## Documentation

- [Reference](doc/reference.md) — the fixture format and the full API in
  both languages, side by side.
- [Fixture guide](test/AGENTS.md) — how to write and place a fixture.
- [Agents guide](AGENTS.md) — repository map and the rules for changing it.

## Development

```bash
make build   # both runtimes
make test    # both runtimes, including the adder module
make vet     # go vet over both Go modules
```

CI runs on push and PR (`.github/workflows/ci.yml`), and a `ts/v*` tag
publishes to npm via OIDC trusted publishing
(`.github/workflows/release.yml`). Workflow changes are staged in
[`ci/`](ci/README.md) first — a maintainer moves them into `.github/`.

Releasing is three commands — `make version V=x.y.z` to move all four
version sites, then `make tag-ts V=x.y.z` (npm, via OIDC trusted
publishing) and `make publish-go V=x.y.z` (both Go modules). A plain
`vX.Y.Z` tag publishes nothing — see [AGENTS.md](AGENTS.md#release).

## License

MIT. Copyright (c) Richard Rodger.
