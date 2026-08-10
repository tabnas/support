# Agents Guide — support

## What this project is

The shared test-support utilities for the tabnas parser system, published
as `@tabnas/support` (npm) and `github.com/tabnas/support/go`.

Every tabnas package ships a canonical TypeScript implementation and a Go
port, and proves they agree by running **one** set of TSV fixtures from
`test/spec/` in **both**. The fixtures were already shared; the loaders
were not, and by the time there were three pairs of them they had drifted:
`jsonic`'s TypeScript loader did not decode `\t` while its Go loader did;
`parser`'s loaders skipped no comment lines while `jsonic`'s and `json`'s
did; the `parser` and `jsonic` TypeScript loaders decoded escapes in every
column while their Go loaders decoded only `input`; and neither of those
pairs decoded `\\` at all. A row that means two different things in two
runtimes cannot pin agreement on anything else.

This repo is the one loader, in two languages. Where the three disagreed,
it follows `@tabnas/json` — the newest pair, and the only one whose two
sides already matched.

## Repository map

| Path | What it is |
|---|---|
| `ts/` | **Canonical** TypeScript package (`@tabnas/support`). Source in `src/`: `escape.ts`, `spec.ts`, `expect.ts`, `runner.ts`, the `support.ts` entry point, and `adder.ts`. |
| `go/` | Go port. Module `github.com/tabnas/support/go`, **no dependencies**. Same four files: `escape.go`, `spec.go`, `expect.go`, `runner.go`, plus `support.go` for the package doc, `VERSION` and the pointer helpers. |
| `go/adder/` | **Separate module** (`github.com/tabnas/support/go/adder`) holding the adder grammar, which needs the parser. |
| `test/spec/` | Shared `.tsv` fixtures. See [`test/AGENTS.md`](test/AGENTS.md). |
| `doc/reference.md` | The fixture format and the full API in both languages, side by side. |

## Authority and alignment rules

1. **TypeScript is canonical.** When TS and Go disagree on behaviour, TS
   wins; change Go, and add a shared fixture when the behaviour is
   expressible as input → output.
2. **The Go support module takes no dependencies.** Every tabnas repo
   depends on it, so anything it required would land in all of them —
   including the parser it is used to test. This is why the adder grammar
   is a separate module, and why `Runner` reads a parse error's code by
   shape (a `Code() string` method or a `Code` string field) rather than
   by importing `*tabnas.TabnasError`.
3. **A behaviour difference between the runtimes is a defect until it is
   documented.** The unavoidable ones are marked **⚠ differs** in
   `doc/reference.md`; there are four, and each says why. Adding a fifth
   without documenting it silently breaks the guarantee the package
   exists to provide.
4. **The version is one number.** `ts/package.json`, `ts/src/support.ts`
   (`VERSION`) and `go/support.go` (`VERSION`) must agree; the version
   test in each runtime fails the build when they drift.

## Testing rules

- A new fixture must pass in BOTH runtimes. `make test` runs everything:
  `ts/`, `go/` and `go/adder/`.
- **An empty fixture and an empty fixture directory must fail.** A fixture
  that loads but holds nothing is a silent pass, and a silent pass is
  indistinguishable from coverage that was never there. Both runners
  assert this, and `ts/test/runner.test.js` / `go/runner_test.go` assert
  that the runner itself fails when it should — a runner that quietly
  passes is the one bug that hides every other one.
- The runner tests drive the failing path through `CheckRow` (Go) and
  `runner.row` (TS), which return or throw the failure rather than
  reporting it, so a failing case can be asserted without failing the
  suite.

## The mini plugin

`ts/src/adder.ts` and `go/adder/adder.go` hold the integer-addition
grammar from the `@tabnas/parser` README (`1+2+3` => 6):

```
val = add
add = NR [ PL add ]
```

It is not a toy kept for its own sake — it is the end-to-end check that
the two runtimes' utilities behave identically, run against the same
`test/spec/adder/*.tsv` rows by both. Keep the two implementations the
same shape: same rule names, same alternates, same declarative form.

Keep it minimal. If it needs a feature to stay in step with a parser
change, that is fine; if it needs one to demonstrate something, that
belongs in the parser's own docs instead.

## Release

The TypeScript package publishes to npm; the Go module is tagged
`go/vX.Y.Z`, and `go/adder` is tagged `go/adder/vX.Y.Z`. Both version
constants and `ts/package.json` must be updated together — see
`make publish-ts` and `make publish-go`.

CI lives in `.github/workflows/` and is promoted by a maintainer via
`tabnas/admin` (session credentials cannot write workflow files), so this
repo's workflows are added out of band, matching the org-standard
`polyglot-ci.yml` caller the other repos use.
