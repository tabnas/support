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
| `ts/` | **Canonical** TypeScript package (`@tabnas/support`). Source in `src/`: `escape.ts`, `spec.ts`, `expect.ts`, `runner.ts`, `census.ts`, the `support.ts` entry point, and `adder.ts`. |
| `go/` | Go port. Module `github.com/tabnas/support/go`, **no dependencies**. Same files: `escape.go`, `spec.go`, `expect.go`, `runner.go`, `census.go`, plus `support.go` for the package doc, `VERSION` and the pointer helpers. |
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
   `doc/reference.md`; there are six, and each says why. Adding a seventh
   without documenting it silently breaks the guarantee the package
   exists to provide.

   Some differences are worth *code* to erase rather than a note: Go's
   `ParseExpect` re-reads an out-of-range number so `1e400` gives ±Inf
   like `JSON.parse`, because otherwise a fixture row would run in one
   runtime and fail to load in the other. Others are the canonical
   runtime's own limits and are shared rather than papered over —
   integers beyond 2^53 are inexact in both, and making Go exact would
   make it *reject* rows TypeScript accepts.
4. **The version is one number, in four places, all four checked.**
   `ts/package.json` is the source of truth. `ts/test/version.test.js`
   pins `ts/src/support.ts` to it; `go/version_test.go` reads it off
   disk and pins both `go/support.go` (`VERSION`) and the `require` on
   the support module in `go/adder/go.mod`.

   That fourth site is the one a stale version breaks nothing local: a
   `replace` covers it for anyone building in this repo, but a `replace`
   in a dependency module is ignored by whoever imports it, so an
   external `go get` resolves the version named there and fails. It sat
   at `v0.1.0` through the 0.1.1 release because nothing looked. Now
   something does.

   `make version V=x.y.z` sets all four; `make publish-go` refuses to
   run when `ts/` has not been bumped first.
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
- **Every shared fixture must run in BOTH runtimes**, and the census
  tests (`go/census_test.go`, `ts/test/census.test.js`) enforce it.
  `test/spec/adder/` is discovered by directory listing in both, so a
  fixture added there runs in both automatically. `test/spec/util/` and
  `test/spec/census/` cannot be — each file has its own column shape and
  assertion, so each suite names the files it runs, and a fixture wired
  into one runtime only would otherwise be silent. The census is a
  static tripwire, not proof: it checks the fixture's name appears in
  that runtime's test sources, so a name in a comment would satisfy it.
  It catches the realistic mistake, which nothing else here would.

## The census helpers

`ts/src/census.ts` and `go/census.go` hold the coverage/parity
tripwires a consuming repo runs over its own data:

- `codesInSpecDir(dir, opts)` / `CodesInSpecDir` walks a fixture
  directory with the shared loader and returns the error codes its
  expectation cells exercise, sorted and unique. Only a **code-style**
  cell counts — `ERROR:` followed by a bare `[a-z][a-z0-9_]*` token,
  after any `@<row>:<col>` position suffix is stripped. A
  message-style expectation (`ERROR:bad token`, `ERROR:1:8`) and a bare
  `ERROR` assert a rejection without naming a code, and returning them
  would count coverage that is not there. `opts` selects the
  expectation column by position or header name; the default is each
  row's last column.
- `compareCatalogues(a, b)` / `CompareCatalogues` diffs two
  `{code: template}` maps — message catalogues, hint catalogues, or one
  runtime's against the other's: keys of `a` absent in `b` (`missing`),
  keys of `b` absent in `a` (`extra`), and shared keys whose templates
  differ byte for byte (`templateMismatch`).
- `coverage(declared, exercised)` / `Coverage` reports declared codes
  no fixture exercises (`uncovered`) and exercised codes nobody
  declares (`orphan`). Whether inherited base codes count as declared
  is the caller's choice — pass them in or leave them out.

Every input arrives as an argument. Nothing here fetches a catalogue or
imports the engine — rule 2 above — which is exactly why these helpers
can live in the one module every repo already depends on. Their
fixtures are `test/spec/census/`, a named-file family like `util/`,
covered by the same census tests.

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
| `ts/vX.Y.Z` | CI publishes `@tabnas/support` to npm via OIDC trusted publishing (`.github/workflows/release.yml`). No token is involved, and none is stored in this repo. |
| `go/vX.Y.Z` | Nothing runs — the Go module proxy serves the module from the tag directly. |
| `go/adder/vX.Y.Z` | Same, for the nested adder module. Without it the module is unresolvable, because Go finds a nested module only under its own path prefix. |

The whole release is three commands:

```bash
make version V=x.y.z    # all four version sites; commit and merge to main
make tag-ts V=x.y.z     # pushes ts/vX.Y.Z — CI publishes to npm with provenance
make publish-go V=x.y.z # pushes go/vX.Y.Z and go/adder/vX.Y.Z
```

Both tag targets tolerate a tag already at HEAD, so a half-done release
can be finished, and both refuse a tag that exists on a *different*
commit — that means the version shipped and the code has moved, so the
answer is a new version, not a moved tag.

**A plain `vX.Y.Z` tag publishes nothing**, because the workflow triggers
on `ts/v*`. That is not hypothetical: `v0.1.1` was tagged that way, no
workflow ran, and the package was then published by hand — which is why
`@tabnas/support@0.1.1` carries no attestations while
`@tabnas/parser@0.8.1` does. `npm run repo-tag` used to create exactly
that wrong tag and `repo-publish-quick` used to `npm publish` locally,
bypassing OIDC; both now go through the `ts/v*` tag instead.

`make publish-ts` still publishes straight from a workstation. It is the
last resort — it produces an unattested release, which is the failure
above.

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

`.github/workflows/ci.yml` and `.github/workflows/release.yml` are both
promoted and live. `release.yml` is byte-identical to the other repos'
from `name:` onward; keep it that way.

Beyond the org-standard `polyglot-ci.yml` caller, `ci.yml` carries one
repo-specific job, `go-adder`: the shared workflow runs `go test ./...`
in `go/` only, and `./...` does not cross a module boundary, so the
`go/adder` suite — the end-to-end check that the two runtimes agree —
would otherwise silently not run.

Keep that job in step with the Makefile: `make test` and CI must cover
the same three trees (`ts/`, `go/`, `go/adder/`). If a second tabnas repo
ever grows a nested module, move the job upstream as a `go-test-dirs`
input to `polyglot-ci.yml` rather than copying it.

## Releasing

Publishing is **dispatch-driven and runs in CI**, never locally:
[`.github/workflows/release.yml`](.github/workflows/release.yml) publishes
`@tabnas/support` to npm over GitHub OIDC trusted publishing (no token,
provenance attached), and a `go/v*` tag is the Go module release —
proxy.golang.org serves it straight from the tag. A local `npm publish` goes
out over a token and bypasses OIDC entirely — do not use it for a release.

### Dispatch it; do not push the tag

**Run the workflow with `workflow_dispatch` on `main`, with the `go` input
true.** That is the path the workflow's own header calls normal, and it is
the only one an agent can take: **a session's credentials cannot push tag
refs — `git push origin ts/v…` fails with HTTP 403**, while branch pushes
from the same credentials succeed. It is a ref-type boundary, not a broken
token or a network fault. Nothing is lost by never touching a tag, because
the workflow creates both tags itself — in one atomic push, *after* npm
accepts the publish. Pushing a tag by hand is the orchestrator's path
(`admin/publish.sh`), not yours.

The steps, in order:

1. Bump all **three** version sites together — `ts/package.json`, `VERSION`
   in `ts/src/support.ts` and `const VERSION` in `go/support.go`. They are
   held equal by `ts/test/version.test.js` and `go/version_test.go`.
2. Verify, building first:

   ```bash
   (cd ts && npm run build && npm test)
   (cd go && GOWORK=off go test ./...)   # only sound with no `replace` — see below
   ```

   This package's `npm test` happens to run `npm run build` first, so the
   build above is redundant here — keep it anyway, because the sibling
   repos' do not and the habit is what travels.
3. **Merge the bump through a reviewed PR.** That is the house convention —
   `CONTRIBUTING.md` squash-merges PRs and takes the title as the commit
   message — and what `release.yml`'s own header describes. A direct push to
   `main` is a recovery path, not the normal one: CI still gates it, but
   nothing reviews it, and step 5 then publishes that unreviewed commit
   immutably. If you take it, say so.
4. **Wait for `main` CI to go green on the bump commit.** The release
   workflow **has no test step** — it reads `main`, builds against
   already-published dependencies, publishes and tags. `ci.yml` on the bump
   PR is the only gate there is. An npm version is immutable, and a Go
   module tag is worse: proxy.golang.org caches module versions permanently,
   so a `go/vX.Y.Z` naming the wrong commit cannot be moved, only
   superseded.
5. Dispatch `release.yml` on `main` with `go: true`.
6. Confirm `npm view @tabnas/support@$V version`, and **query both tags
   exactly**:

   ```bash
   V=x.y.z
   git ls-remote --tags origin "refs/tags/ts/v$V" "refs/tags/go/v$V" | wc -l   # want 2
   ```

   `git ls-remote --tags origin | grep v$V` is not a check. `grep` exits 0
   if *either* ref matches, so it reports success in precisely the
   half-finished state — npm published and `ts/v` written, `go/v` not — that
   the workflow is built to let you repair by re-dispatching.

The workflow fails closed on a dispatch from any ref but `main`, and when
every tag it would create already exists (the "you forgot to bump" signal).
It fails *open* on an already-published npm version, so a run that published
and then died before tagging is repairable by re-dispatching rather than
stuck.

### Verifying against the published module, not your checkout

`GOWORK=off` is necessary and **not sufficient**. It disables the workspace
and nothing else — it does *not* neutralise a `replace` in `go.mod`, because
a replacement with no version on the left applies to every version. The
`require` then still resolves to the sibling directory, and the suite goes
green against the very checkout you were trying to stop using:

```
$ GOWORK=off go list -m github.com/tabnas/parser/go
github.com/tabnas/parser/go v0.9.6 => /…/parser/go
```

Assert the absence first, and only then believe the run:

```bash
cd go
go mod edit -json | grep -q '"Replace": null' || { echo 'go.mod still has a replace'; exit 1; }
GOWORK=off go test ./...
```

The TypeScript equivalent is `ts/package-lock.json`: it is gitignored, it
pins the previous versions, and `npm install` after a dependency bump will
happily keep them — the suite then passes against the packages you were
replacing. Delete it before verifying. Both of these produce a green local
run against the wrong version, which is the only kind of green worth
distrusting.

### Never commit the local wiring

Testing against unreleased siblings means symlinked `node_modules`,
`replace` directives and a workspace. None of it may reach a commit, and
`git add -A` is how it does:

- `go mod edit -replace …=/abs/path` — CI reports it as `replacement
  directory /… does not exist`.
- **`go.sum`, after the replace comes out.** A `replace` makes the sibling's
  sums unused, so `go mod tidy` drops them; reverting `go.mod` alone then
  leaves `missing go.sum entry` — a *different* error on the commit meant to
  fix the first one. Revert both, and diff them against the last release
  commit.
- **A `go.work` belongs outside every repo**, one level up, `use`ing each
  module, so no repo can track it. It also never consults `go.sum`, so it
  cannot tell you whether a *declared* version is sound.
- Scratch files — anything written to measure something.

Stage deliberately (`git add <path>`) and read `git status --short` before
every commit. This bites hardest on a PR whose CI is *expected* red for a
known dependency: a fresh breakage hides inside the expected failure.

### `make publish-ts` and `make publish-go` are not the release path

They predate `release.yml`. Read what each actually does before using
either:

- `publish-ts` runs a local `npm publish`, which goes out over a token and
  bypasses the OIDC trusted publishing the workflow uses.
- `publish-go V=x.y.z` is the sound one here — it refuses unless
  `ts/package.json` already reads `V`, and it runs the Go suites *after* the
  bump rather than before. It still pushes a tag, which a session cannot do;
  that is the only reason it is not your path.

They stay in the Makefile because removing them is a separate change.

## Agent tooling

An agent working in this repository does not have to drive it by hand. The
org ships two things that already understand these grammars:

- **[`@tabnas/mcp`](https://github.com/tabnas/mcp)** — an MCP server (stdio)
  and the unified `tabnas` CLI: parse, validate and inspect any tabnas
  format, this one included.
- **[`tabnas/skills`](https://github.com/tabnas/skills)** — Agent Skills for
  working on tabnas grammars and plugins.

Prefer them over ad-hoc scripts when exploring a grammar or checking a parse
result.
