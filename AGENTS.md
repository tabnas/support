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
   `doc/reference.md`; there are five, and each says why. Adding a sixth
   without documenting it silently breaks the guarantee the package
   exists to provide.

   Some differences are worth *code* to erase rather than a note: Go's
   `ParseExpect` re-reads an out-of-range number so `1e400` gives ±Inf
   like `JSON.parse`, because otherwise a fixture row would run in one
   runtime and fail to load in the other. Others are the canonical
   runtime's own limits and are shared rather than papered over —
   integers beyond 2^53 are inexact in both, and making Go exact would
   make it *reject* rows TypeScript accepts.
4. **The version is one number.** `ts/package.json`, `ts/src/support.ts`
   (`VERSION`) and `go/support.go` (`VERSION`) must agree; the version
   test in each runtime fails the build when they drift.
   `go/adder/go.mod`'s `require` on the support module is a fourth place
   the number appears — it must name a real published version, because a
   `replace` in a dependency module is ignored by whoever imports it.
   `make publish-go` updates it and refuses to run when `ts/` has not
   been bumped first.
5. **This is test-support code and must never reach a release
   artifact.** In TypeScript that means a `devDependency` imported only
   from `test/`; in Go it means importing it only from `_test.go` files,
   which the import graph then enforces — see
   `go/README.md#keeping-it-out-of-your-build` for the `go list -deps`
   check that consuming repos should run in CI.

## Testing rules

- A new fixture must pass in BOTH runtimes. `make test` runs everything:
  `ts/`, `go/` and `go/adder/`.
- **An empty fixture and an empty fixture directory must fail.** A fixture
  that loads but holds nothing is a silent pass, and a silent pass is
  indistinguishable from coverage that was never there. The empty
  directory is rejected by `loadSpecDir` / `LoadSpecDir`; the empty
  fixture by `checkSpec` / `CheckSpec`.
- **A guard that can only fail a test cannot be tested.** That is why
  both of those are ordinary functions that throw or return an error,
  rather than checks buried in a `t.Fatalf` or an `it()` — and why the
  per-row comparison is `CheckRow` (Go) / `runner.row` (TS). Every one of
  them has a test asserting it fails when it should. A runner that
  quietly passes is the one bug that hides every other one, so no guard
  here is allowed to be unassertable.

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

Three tags, and each one means something different:

| Tag | Effect |
|---|---|
| `ts/vX.Y.Z` | CI publishes `@tabnas/support` to npm via OIDC trusted publishing (`ci/workflows/release.yml`). No token is involved, and none is stored in this repo. |
| `go/vX.Y.Z` | Nothing runs — the Go module proxy serves the module from the tag directly. |
| `go/adder/vX.Y.Z` | Same, for the nested adder module. Without it the module is unresolvable, because Go finds a nested module only under its own path prefix. |

`make publish-go V=x.y.z` creates the two Go tags together. It bumps the
Go version sites if they are behind and commits that, and is equally
happy when `make version` already set them and the commit is in — the
normal case. `make publish-ts` is a manual fallback only; the `ts/v*`
tag is the intended path, and it needs no credentials.

**A plain `vX.Y.Z` tag publishes nothing.** `npm run repo-tag` in
`ts/package.json` creates exactly that. It is the org-wide script and
predates the release workflow, which triggers on `ts/v*` — which is how
`v0.1.1` came to exist here with no `ts/v0.1.1` beside it. Release
through the orchestrator (`admin/publish.sh`), or tag `ts/vX.Y.Z` by
hand.

### Bump the version with `make version V=x.y.z`

The version appears in **four** places: `ts/package.json`,
`ts/src/support.ts`, `go/support.go`, and the `require` on the support
module in `go/adder/go.mod`. Moving some but not all of them leaves the
repo **failing**, not merely inconsistent — the version test in each
runtime compares against `ts/package.json`.

That is not hypothetical. `v0.1.1` shipped with `go/support.go` still
reading `0.1.0`, which turned `go test ./...` red on `main`; and because
`publish-go` ran the tests as prerequisites, the target that would have
fixed it refused to start. `make version` moves all four at once, and
`publish-go` now tests after the bump rather than before.

## CI

CI lives in `.github/workflows/` and is promoted by a maintainer via
`tabnas/admin` (session credentials cannot write workflow files), so
workflow changes are **staged in [`ci/workflows/`](ci/README.md)** and
moved across out of band.

Both `ci/workflows/ci.yml` and `ci/workflows/release.yml` are staged and
**not yet promoted — this repo has no CI and no release automation until
they are**. `release.yml` is byte-identical to the other repos' from
`name:` onward; keep it that way.

Beyond the org-standard `polyglot-ci.yml` caller, `ci.yml`
carries one repo-specific job, `go-adder`: the shared workflow runs `go
test ./...` in `go/` only, and `./...` does not cross a module boundary,
so the `go/adder` suite — the end-to-end check that the two runtimes
agree — would otherwise silently not run.

Keep that job in step with the Makefile: `make test` and CI must cover
the same three trees (`ts/`, `go/`, `go/adder/`). If a second tabnas repo
ever grows a nested module, move the job upstream as a `go-test-dirs`
input to `polyglot-ci.yml` rather than copying it.
