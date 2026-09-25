# Agents Guide — support

## Core principle: dependencies change only on explicit instruction

**Dependencies may only be changed by explicit instruction from the
maintainer.** This covers every dependency this repository declares, in
every runtime and every manifest:

- `package.json` `dependencies`, `peerDependencies` and `devDependencies`,
  and their lockfiles;
- `go.mod` `require` and `replace` lines, their versions, and `go.sum`;
- `Cargo.toml` dependency tables and `Cargo.lock`;
- any other manifest here, nested test modules included.

Adding, removing, re-pointing or re-versioning any of them is a
dependency change.

- **A dependency never arrives as a side effect.** Watch for an import,
  `go mod tidy`, `npm install`, `cargo update`, a stamped template, or a
  fix for something else. If a change would alter a dependency, stop and
  ask before making it. Do not make it and explain afterwards.
- **An explicit instruction names the change**, for example "bump the
  parser requirement in X to 0.12" or "cascade the parser release". A
  goal is not an instruction for its means. "Make CI green", "ship the C
  library" or "fix the build" does not authorise a dependency change,
  however direct the route through one looks.
- **This repository's own version sites are not dependencies.** They
  include the root entry of its own lockfile. A release bump moves them.
- **Versions track the latest release.** Every dependency is kept at
  its latest published version, and none is held on an older one. That
  is the maintainer's standing instruction, so moving a dependency to
  its latest version needs no further one. Holding a dependency back,
  or adding, removing or re-pointing one, still does.

## Core principle: transient tasks report progress

**Every transient task produces status output at least every 30 seconds,
with an estimate of how far through it is, as a percentage, where one can
be made.** This is the maintainer's instruction. A transient task is any
work that runs for a while and then ends: a build, a test or conformance
sweep, an install or a fetch, a release, a wait on CI, a benchmark, a
script or loop you write, and anything sent to the background.

- **Minimal is enough.** One line with the step and a count, such as
  `conformance: 412 of 1500 (27%)`, meets it. When no total is known, print
  what is known (the step, the current item, the elapsed time) and say the
  percentage is unknown rather than inventing one.
- **Build it into what you write.** A script or loop prints a line per
  item or per interval. A quiet tool gets its progress or verbose flag, or
  a wrapper that prints a heartbeat, so that nothing runs silent for more
  than 30 seconds.
- **Silence reads as a hang.** Whoever is watching, a person or an agent,
  cannot tell a slow task from a stuck one without it, and so cannot
  decide whether to wait or to stop it.

A quick command that finishes within 30 seconds needs nothing extra.

## What this project is

The shared test-support utilities for the tabnas parser system, published
as `@tabnas/support` (npm) and `github.com/tabnas/support/go`.

Every tabnas package ships a canonical TypeScript implementation and
ports of it (Go, and increasingly Rust), and proves they agree by running
**one** set of TSV fixtures from `test/spec/` in **every runtime**. The fixtures were already shared; the loaders
were not, and by the time there were three pairs of them they had drifted:
`jsonic`'s TypeScript loader did not decode `\t` while its Go loader did;
`parser`'s loaders skipped no comment lines while `jsonic`'s and `json`'s
did; the `parser` and `jsonic` TypeScript loaders decoded escapes in every
column while their Go loaders decoded only `input`; and neither of those
pairs decoded `\\` at all. A row that means two different things in two
runtimes cannot pin agreement on anything else.

This repo is the one loader, in three languages. Where the three disagreed,
it follows `@tabnas/json` — the newest pair, and the only one whose two
sides already matched.

## Repository map

| Path | What it is |
|---|---|
| `ts/` | **Canonical** TypeScript package (`@tabnas/support`). Source in `src/`: `escape.ts`, `spec.ts`, `expect.ts`, `runner.ts`, `census.ts`, the `support.ts` entry point, and `adder.ts`. |
| `go/` | Go port. Module `github.com/tabnas/support/go`, **no dependencies**. Same files: `escape.go`, `spec.go`, `expect.go`, `runner.go`, `census.go`, plus `support.go` for the package doc, `VERSION` and the pointer helpers. |
| `go/adder/` | **Separate module** (`github.com/tabnas/support/go/adder`) holding the adder grammar, which needs the parser. **Private**: an internal test module, never tagged or published (see [Release](#release)). |
| `rs/` | Rust port. Crate `tabnas-support` (library `tabnas_support`), **no required dependencies**: `escape.rs`, `spec.rs`, `expect.rs`, `runner.rs`, `register.rs`, `census.rs`, plus `value.rs`, the fixture data model with its own JSON reader and writer. See [`rs/AGENTS.md`](rs/AGENTS.md). |
| `rs/adder/` | **Separate crate** (`tabnas-support-adder`) holding the adder grammar, which needs the engine as a sibling checkout (`../../../parser/rs`). |
| `test/spec/` | Shared `.tsv` fixtures. See [`test/AGENTS.md`](test/AGENTS.md). |
| `doc/reference.md` | The fixture format and the full API in both languages, side by side. |

## Authority and alignment rules

1. **TypeScript is canonical.** When a port and TS disagree on
   behaviour, TS wins; change the port, and add a shared fixture when the
   behaviour is expressible as input → output. Go and Rust are both
   ports, and neither is canonical for the other: a difference between
   them is two questions about TS, not one about each other.
2. **The Go support module and the Rust support crate take no
   dependencies.** Every tabnas repo depends on them, so anything they
   required would land in all of them — including the parser they are
   used to test. This is why the adder grammar is a separate module and a
   separate crate, why Go's `Runner` reads a parse error's code by shape
   (a `Code() string` method or a `Code` string field) rather than by
   importing `*tabnas.TabnasError`, and why the Rust runner takes a
   `Failure` struct the suite builds at the boundary. The Rust crate's
   only dependency is the optional, off-by-default `serde_json` feature.
3. **A behaviour difference between the runtimes is a defect until it is
   documented.** The unavoidable ones are marked **⚠ differs** in
   `doc/reference.md`, and each says why. There are **six** between
   TypeScript and Go, and a further **six** in the Rust section, which
   `rs/README.md` repeats. Adding a seventh to either set without
   documenting it silently breaks the guarantee the package exists to
   provide.

   Some differences are worth *code* to erase rather than a note: Go's
   `ParseExpect` re-reads an out-of-range number so `1e400` gives ±Inf
   like `JSON.parse`, because otherwise a fixture row would run in one
   runtime and fail to load in the other. Others are the canonical
   runtime's own limits and are shared rather than papered over —
   integers beyond 2^53 are inexact in both, and making Go exact would
   make it *reject* rows TypeScript accepts.
4. **The version is one number, in eight places, seven of them checked.**
   `ts/package.json` is the source of truth. `ts/test/version.test.js`
   pins `ts/src/support.ts` to it; `go/version_test.go` reads it off
   disk and pins both `go/support.go` (`VERSION`) and the `require` on
   the support module in `go/adder/go.mod`; `rs/tests/version_test.rs`
   pins `rs/Cargo.toml`, `rs/src/lib.rs` (`VERSION`) and
   `rs/adder/Cargo.toml`.

   That fourth site is the one a stale version breaks nothing local: a
   `replace` covers it for anyone building in this repo, but a `replace`
   in a dependency module is ignored by whoever imports it. No release
   tags the adder module, but the repository is public, so Go can still
   fetch any commit of it as a pseudo-version, and that fetch resolves
   the version named there and fails. It sat at `v0.1.0` through the
   0.1.1 release because nothing looked. Now something does.

   The eighth is `ts/package-lock.json`, which nothing asserts; see
   [Releasing](#releasing) step 1.

   `make version V=x.y.z` sets all eight; `make publish-go` refuses to
   run when `ts/` has not been bumped first.
5. **This is test-support code and must never reach a release
   artifact.** In TypeScript that means a `devDependency` imported only
   from `test/`; in Go it means importing it only from `_test.go` files,
   which the import graph then enforces — see
   `go/README.md#keeping-it-out-of-your-build` for the `go list -deps`
   check that consuming repos should run in CI.

## Testing rules

- A new fixture must pass in EVERY runtime. `make test` runs everything:
  `ts/`, `go/`, `go/adder/`, `rs/` and `rs/adder/`.
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
- **Every shared fixture must run in EVERY runtime**, and the census
  tests (`go/census_test.go`, `ts/test/census.test.js`,
  `rs/tests/census_test.rs`) enforce it.
  `test/spec/adder/` is discovered by directory listing in every
  runtime, so a fixture added there runs everywhere automatically.
  `test/spec/util/`, `test/spec/census/` and `test/spec/register/`
  cannot be — each file has its own column shape and
  assertion, so each suite names the files it runs, and a fixture wired
  into one runtime only would otherwise be silent. The census is a
  static tripwire, not proof: it checks the fixture's name appears in
  that runtime's test sources, so a name in a comment would satisfy it.
  It catches the realistic mistake, which nothing else here would.

## The census helpers

`ts/src/census.ts`, `go/census.go` and `rs/src/census.rs` hold the
coverage/parity tripwires a consuming repo runs over its own data:

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

`ts/src/adder.ts`, `go/adder/adder.go` and `rs/adder/src/lib.rs` hold
the integer-addition grammar from the `@tabnas/parser` README
(`1+2+3` => 6):

```
val = add
add = NR [ PL add ]
```

It is not a toy kept for its own sake — it is the end-to-end check that
the three runtimes' utilities behave identically, run against the same
`test/spec/adder/*.tsv` rows by each. Keep the three implementations the
same shape: same rule names, same alternates, same declarative form.

Keep it minimal. If it needs a feature to stay in step with a parser
change, that is fine; if it needs one to demonstrate something, that
belongs in the parser's own docs instead.

## Release

Two tags, and each one means something different:

| Tag | Effect |
|---|---|
| `ts/vX.Y.Z` | CI publishes `@tabnas/support` to npm via OIDC trusted publishing (`.github/workflows/release.yml`). No token is involved, and none is stored in this repo. |
| `go/vX.Y.Z` | Nothing runs — the Go module proxy serves the module from the tag directly. |

**`go/adder` gets no tag.** It is a private internal test module, never
tagged and never published, by the maintainer's decision (admin#19,
recorded beside the nested-module handling in `admin/publish.sh`). Only
this repository consumes it: `make test` builds it through its
`replace github.com/tabnas/support/go => ../`, and CI's `go-adder` job
through a workspace, both from source. A release moves the support
`require` in `go/adder/go.mod` (below), so the module keeps building
against the version being released, and does nothing else to it.

Two adder tags exist from before that decision, `go/adder/v0.2.0`
(`ef40e438`) and `go/adder/v0.3.0` (`a5df8829`). They stay where they
are: a Go tag is immutable once proxy.golang.org has served it, so
deleting one would only make Git and the proxy disagree. Do not create
another, and do not read the missing ones for v0.3.1 onward as a gap to
backfill ([#21](https://github.com/tabnas/support/issues/21)). Because
the repository is public, Go can still fetch the module as a
pseudo-version; that is Go's behaviour, not a release.
`ts/test/release.test.js` fails if `release.yml` names the module
outside a comment, or if any Markdown page in the repository or the
Makefile describes a per-release adder tag.

The whole release is three commands:

```bash
make version V=x.y.z    # all four version sites; commit and merge to main
make tag-ts V=x.y.z     # pushes ts/vX.Y.Z — CI publishes to npm with provenance
make publish-go V=x.y.z # pushes go/vX.Y.Z (and nothing for go/adder)
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

The version appears in **eight** places: `ts/package.json`,
`ts/package-lock.json`, `ts/src/support.ts`, `go/support.go`, the
`require` on the support module in `go/adder/go.mod`, `rs/Cargo.toml`,
`rs/src/lib.rs` and `rs/adder/Cargo.toml`. Moving some but not all of
them leaves the repo **failing**, not merely inconsistent — the version
test in each runtime compares against `ts/package.json`.

That is not hypothetical. `v0.1.1` shipped with `go/support.go` still
reading `0.1.0`, which turned `go test ./...` red on `main`; and because
`publish-go` ran the tests as prerequisites, the target that would have
fixed it refused to start. `make version` moves all eight at once (the
Rust three by way of `make version-rs`, the lockfile as a side effect of
the `npm version` it runs), and `publish-go` now tests after the bump
rather than before.

## CI

CI lives in `.github/workflows/`. To change it, edit the workflow there
in a reviewed pull request: session credentials push workflow files
(admin `DECISIONS.md` ADR-8, as amended 2026-09-24), so staging a change
in `ci/workflows/` first is optional (see [`ci/README.md`](ci/README.md)).
Sessions still cannot push tags, so a maintainer pushes any tag that a
tag-triggered workflow needs.

The four workflows once staged in `ci/` are promoted and live: `ci.yml`,
`release.yml`, `rust.yml` and `docs.yml`. `release.yml` tracks the fleet
copy (the one in `json`, `jsonic` and `expr`) from `name:` onward with
exactly one intended difference: the package name in its `npm view`
calls. Keep it to that one. In particular it creates the fleet's two
tags and no third for `go/adder`, which is private (see
[Release](#release)).

Beyond the org-standard `polyglot-ci.yml` caller, `ci.yml` carries one
repo-specific job, `go-adder`: the shared workflow runs `go test ./...`
in `go/` only, and `./...` does not cross a module boundary, so the
`go/adder` suite — the end-to-end check that the two runtimes agree —
would otherwise silently not run.

Keep that job in step with the Makefile: `make test` and CI must cover
the same trees. `make test` covers five (`ts/`, `go/`, `go/adder/`,
`rs/` and `rs/adder/`); `ci.yml` covers the first three, and `rust.yml`
runs `ci/rust/run.sh` over the other two. `rust.yml` is path-filtered
(`rs/**`, `test/spec/**`, `ts/package.json`, `ci/rust/**` and itself),
so a change outside those paths does not run it, and it builds the
adder crate against parser `main` rather than a release, so an engine
change can turn it red with nothing changed here.

If a second tabnas repo ever grows a nested module, move the `go-adder`
job upstream as a `go-test-dirs` input to `polyglot-ci.yml` rather than
copying it.

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
the workflow creates the tags itself, in one atomic push, *after* npm
accepts the publish. Pushing a tag by hand is the orchestrator's path
(`admin/publish.sh`), not yours.

The steps, in order:

1. Bump all **eight** version sites together — `ts/package.json`, `VERSION`
   in `ts/src/support.ts`, `const VERSION` in `go/support.go`,
   `ts/package-lock.json` (regenerated, not hand-edited), the
   `github.com/tabnas/support/go` requirement in `go/adder/go.mod`, and the
   Rust three: `rs/Cargo.toml`, `VERSION` in `rs/src/lib.rs` and
   `rs/adder/Cargo.toml`.
   The `go/adder/go.mod` one is easy to miss because `go/adder/` is a
   separate module: it pins the version of this one, so leaving it behind
   fails `TestVersionMatchesAdderRequire` in step 2, before you can merge.
   `make version V=x.y.z` moves all of them — the Rust three by way of
   `make version-rs`, the lockfile as a side effect of the `npm version` it
   runs.

   Drift is caught for **seven** of the eight, not all:
   `ts/test/version.test.js` pins `ts/src/support.ts` to `ts/package.json`,
   `go/version_test.go` pins `go/support.go` and the `go/adder/go.mod`
   require to it, and `rs/tests/version_test.rs` pins `rs/Cargo.toml`,
   `rs/src/lib.rs` and `rs/adder/Cargo.toml` to it. Nothing
   asserts the lockfile's own version field — `ts/test/enginepin.test.js`
   does read the lockfile, but only for the `@tabnas/parser` pin. So a bump
   made by hand instead of by `make version` can leave `ts/package-lock.json`
   behind with every test named here still green. Use `make version`; if you
   edit by hand anyway, check the lockfile yourself.

   Each Rust crate's own entry in its `Cargo.lock` is a ninth and tenth
   site, moved by any `cargo` command and checked by `ci/rust/run.sh`
   (`check_lock_version`) rather than by a version test. `make version-rs`
   runs `cargo metadata` for exactly that.
2. Verify against the **published** dependencies rather than your checkout.
   The release runner installs fresh from the registry; a working tree
   usually does not, so reproduce that before believing anything:

   ```bash
   (
     cd ts
     # package-lock.json is TRACKED here — regenerate it, do not delete it
     rm -rf node_modules
     npm install
     npm test
   )
   ```

   **Removing the lockfile is not enough on its own.** It does not touch
   `node_modules`, and the sibling symlinks that make local development work
   (`ts/node_modules/@tabnas/…` pointing at a checkout) survive it — the
   suite then passes against unreleased code while appearing to verify the
   published one. Reinstalling is the part that matters.

   `npm test` already compiles here — the `test` script itself begins with
   `npm run build`. No separate build step is needed.

   On the Go side, `GOWORK=off` is necessary and **not sufficient** — it
   disables the workspace and nothing else. A `replace` carrying no version
   on the left applies to every version, so the `require` still resolves to
   the sibling directory. Assert its absence first:

   ```bash
   (
     cd go
     go mod edit -json | grep -q '"Replace": null' || { echo 'go.mod has a replace'; exit 1; }
     GOWORK=off go test -count=1 ./...
     (cd adder && GOWORK=off go test -count=1 ./...)
   )
   ```

   `-count=1` because shared fixtures live outside the Go module, so a
   changed corpus does not invalidate the test cache.

   **`./...` from `go/` does not reach `go/adder`.** It is a separate module,
   and `go test ./...` stops at the module boundary — verified: the run above
   reports only `github.com/tabnas/support/go`. A change that breaks only the
   adder passes this step unless you enter that directory, so the second line
   is not redundant. `make test-go test-go-adder` runs both. Note `adder`'s
   own `go.mod` carries `replace github.com/tabnas/support/go => ../` by
   design, so the no-replace assertion above applies to `go/` only.
3. **Merge the bump through a reviewed PR.** That is the house convention —
   `CONTRIBUTING.md` squash-merges PRs and takes the title as the commit
   message — and what `release.yml`'s own header describes. A direct push to
   `main` is a recovery path, not the normal one: CI still gates it, but
   nothing reviews it, and step 5 then publishes that unreviewed commit
   immutably. If you take it, say so.
4. **Wait for `main` CI to go green on the bump commit.** The release
   workflow **has no test step** — it reads `main`, builds against
   already-published dependencies, publishes and tags. `ci.yml`,
   `deps-gate.yml` and `rust.yml` on the bump commit are the only gates
   there are (`rust.yml` runs because its path filter matches the bump's
   `ts/package.json` change). An npm version is immutable, and a Go
   module tag is worse: proxy.golang.org caches module versions permanently,
   so a `go/vX.Y.Z` naming the wrong commit cannot be moved, only
   superseded.
5. **Record the release commit, then dispatch.** The confirmation
   below compares each tag against the commit you released, and a run
   that publishes and then fails to tag can be followed by `main`
   moving — so capture it *before* the dispatch, and read it from the
   remote rather than a local ref that may be stale:

   ```bash
   REL=$(git ls-remote origin refs/heads/main | cut -f1)
   ```

   Then dispatch `release.yml` on `main` with `go: true`.

   Keep that SHA. If a later run has to repair this release, the comparison
   must still be against the commit npm actually served — re-reading `main`
   at repair time gives you whatever it has become, which is exactly the
   value the faulty anchor would also produce, so the check would agree with
   itself and pass. If you no longer have it, recover it from the original
   run: the `head_sha` of that `release.yml` run is the commit it published.
6. Confirm — and make the check **fail**, not merely print:

   ```bash
   V=x.y.z
   npm view @tabnas/support@$V version
   npm view @tabnas/support@$V dist.attestations   # empty = unattested
   GH=$(npm view @tabnas/support@$V gitHead)
   [ -n "$GH" ] || { echo "npm records no gitHead for $V"; exit 1; }
   for T in "ts/v$V" "go/v$V"; do
     S=$(git ls-remote origin "refs/tags/$T" | cut -f1)
     [ -n "$S" ] || { echo "missing tag $T"; exit 1; }
     [ "$S" = "$GH" ] || { echo "$T is $S, but npm shipped $GH"; exit 1; }
   done
   [ "$GH" = "$REL" ] || { echo "shipped $GH, not the $REL you cleared"; exit 1; }
   ```

   **Two tags, not three.** `go/adder` is private and never tagged (see
   [Release](#release)), so neither the dispatch nor `make publish-go`
   creates an adder tag and this script does not look for one.
   `ts/test/release.test.js` holds the workflow's tag list, and this loop,
   to exactly `ts/v` and `go/v`.

   **Check `dist.attestations`, not just that the version exists.** The
   workflow fails *open* on an already-published version, so a version that
   reached npm by some other route satisfies `npm view … version` while
   carrying no provenance — which is exactly how 0.1.1 shipped unattested,
   as the Makefile records. An empty field means the artifact is not
   attested, whatever the tags say.

   Counting the refs is not enough either. `grep v$V` exits 0 when *either*
   ref matches; a bare `wc -l` prints the count and exits 0 regardless; and
   a bare count passes in the case this section warns about, because an
   anchor fallback writes the tags on a commit npm never served — and wrong
   tags count the same as right ones. Comparing each against the commit you
   released is what catches that, and it also catches a `make publish-go`
   run from a `main` that has since moved.

   The refs carry the commit directly: both `release.yml` (`git tag "$T"
   "$ANCHOR"`) and `make publish-go` create lightweight tags, so there is no
   `^{}` to peel.

   `$REL` is deliberately not what the tags are measured against. It is
   your record of what you meant to release, and a repair can make the
   tags agree with it while npm serves something else: publish from A,
   lose the atomic tag push, re-capture `main` at B, and the repair tags
   B — so a `$REL`-only loop passes while the registry still serves A.
   `gitHead` is npm's own record of the commit the tarball was built from,
   so that is what the tags are checked against, and `$REL` is checked
   separately, as the CI question it actually is.

   When the script exits nonzero, the line that failed says what to do. A
   tag that is not `$GH` is wrong, and the two are not equally
   recoverable. A wrong `ts/v$V` simply moves: npm resolves from the
   registry, so the tag is a signpost and nothing reads it. A wrong
   `go/v$V` does not. `proxy.golang.org` caches a module version's content
   immutably, so once anything has fetched `v$V` that content is what
   consumers get for good, and a corrected tag only makes Git and the
   proxy disagree — and you cannot find out whether it has been fetched
   without causing it, because asking the proxy is itself a fetch. Leave
   that tag where it is and release the next patch from the right commit,
   carrying `retract v$V` in its `go/go.mod`: the cached content stays,
   but `go get` stops selecting the bad version and reports it as
   retracted.

   The last line is a different failure. The tags are honest and `$REL` is
   the stale capture — `main` moved before the run checked out — but what
   shipped is then a commit you never cleared CI on, and `release.yml`
   runs no tests of its own. Confirm `$GH` is green on `main` before
   calling the release good.

   **The dispatch also creates the GitHub Release (admin ADR-19).** Once
   the tags are on the remote, `release.yml` calls
   `.github/workflows/github-release.yml`, which creates a notes-only
   Release on `go/v$V` (on `ts/v$V` where there is no Go module). The release
   is done when that Release is published. If the `github-release` job
   failed after npm and Go had shipped, fix the cause, then dispatch
   `github-release.yml` on `main` with the tag: it creates the Release if it
   is missing and leaves an existing one alone. `release.yml` itself cannot
   do this, because it refuses a re-dispatch once every tag exists.

### When a dispatch dies half-way

The workflow fails closed on a dispatch from any ref but `main`, and when
every tag it would create already exists (the "you forgot to bump" signal).
It fails *open* on an already-published npm version, so a run that published
and then died before tagging can be re-dispatched — **but only while `main`
still points at the release commit.**

That caveat is the sharp edge. The repair logic anchors new tags to an
*existing* tag. If the run published to npm and died before the atomic push,
neither tag exists to supply that anchor — so if `main` has moved on, the
anchor falls back to the new `HEAD` while the publish step skips the version
already on npm. Both tags then land on a commit that is not the one npm
serves, and for the Go module that is permanent. In that state, recover the
original SHA and tag it by hand, or bump to the next patch. Do not just
re-dispatch.

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
- **A `go.work` belongs outside every repo**, one level up. Be precise about
  what it does and does not check: it still consults the `go.sum` files of
  its member modules and writes any missing sums to `go.work.sum`. What it
  skips is validating the *declared version* of a module it replaces with a
  local one — which is exactly the part that hides a bad dependency bump,
  and why the `GOWORK=off` run above exists.
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
  `ts/package.json` already reads `V`, and runs the Go suites *after* the
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
