# ci/

Staging area for GitHub Actions workflow changes.

## Pending

- **`workflows/release.yml`** — the deployed copy plus one tag. It is
  the promoted file with `go/adder/v$V` added to the tag list, and
  nothing else changed.

  `go/adder` is a separate Go module, and Go finds a nested module only
  under its own path prefix, so `go/adder/vX.Y.Z` is the whole of that
  module's release. The deployed workflow writes `ts/v` and `go/v` and
  stops, which left `github.com/tabnas/support/go/adder` unresolvable at
  v0.3.1 through v0.3.4: npm published, both tags landed, and the
  release-time confirmation passed every time
  ([#21](https://github.com/tabnas/support/issues/21)). Only
  `make publish-go` creates the third tag, so a dispatch-driven release
  needs a maintainer hand-off until this is promoted.

  The tag goes in the one list the already-released guard, the anchor
  choice and the atomic push all read, so it is guarded and pushed with
  the other two rather than beside them. The `go/*` case arm needs no
  change: it is a prefix glob, so it gates the new tag behind the `go`
  input already. `ts/test/release.test.js` asserts all of that against
  this file, so the fix cannot be edited back out unnoticed.

- **`workflows/rust.yml`** — the Rust gate: `ci/rust/run.sh` over the
  `rs/` crate and the `rs/adder/` crate (formatting, both lockfiles,
  build, tests, doctests, clippy), with `tabnas/parser` cloned as a
  sibling because the adder crate takes the engine by path. Standalone
  rather than an arm of `ci.yml`, since the shared polyglot workflow
  takes no Rust input. Runs on `rs/**`, `test/spec/**`, `ts/package.json`
  and its own files.

- **`workflows/docs.yml`** — the prose gate: Vale over the reader-facing
  pages at the levels set in `.vale.ini`, on the file list
  `ts/scripts/gated-docs.cjs` produces. See `docs/STYLE-GUIDE.md`.

  It needs no sibling checkouts and no secrets, and pins its own Vale
  version. Errors fail the job; warnings go to the run summary as a
  report. `make prose` runs the identical check locally, and the other
  half of the gate (`ts/test/docs.test.js`) already runs in `make test`,
  so promoting this adds the spelling and Google-convention arm rather
  than the whole gate.

## Promoted

Both earlier staged workflows now live in `.github/workflows/`:

- **`ci.yml`** — the org-standard thin caller delegating to
  `tabnas/.github/.github/workflows/polyglot-ci.yml@main` with
  `deps: "parser"`, plus the repo-specific `go-adder` job described below.
- **`release.yml`** — publishes `@tabnas/support` to npm on a `ts/v*` tag
  push via GitHub OIDC Trusted Publishing. No `NPM_TOKEN`, no secret in
  this repo. It tracks the `release.yml` in `parser`, `json`, `jsonic`
  and `expr` from `name:` onward, differing only in the package name its
  `npm view` calls name. The staged copy above adds a second intended
  difference, the nested module's tag, which no other repo needs.

This directory exists because session credentials cannot write
`.github/workflows/*` — see admin `DECISIONS.md` ADR-8. To change CI:

1. Put the intended workflow file in `workflows/`.
2. A maintainer promotes it with the admin `rollout/apply-ci-folders.sh`
   script.

## The `go-adder` job

Worth knowing why `ci.yml` is not the one-line caller every other repo
has. The shared workflow runs `go test ./...` in `go/` only, and `./...`
does not cross a module boundary. `go/adder` is a **separate module** —
that is what keeps the support module dependency-free — so without a
second job the adder suite does not run at all. That suite is the
end-to-end check that the two runtimes agree, and a test that quietly
does not run reports a green tick that is a lie.

The job clones `parser` as a sibling and wires it in through a `go.work`,
so the grammar is tested against parser *source* rather than the
published module — the same reasoning behind the shared workflow's own
sibling clones. The workspace also covers the support module itself.

It sets `cache: false` on `setup-go`. The cache step reads
`$GITHUB_WORKSPACE` directly for a `go.mod` — non-recursively, without
walking up — and `checkout` puts the repo one level down, so it finds
nothing and annotates every run with "Restore cache failed". setup-go
catches that and carries on, so it is a warning rather than a failure,
but a warning that cries wolf on every run is how the real ones get
ignored. There is little to cache in any case: the support module has no
dependencies, `go/adder/go.sum` pins exactly one, and the workspace
overrides it with sibling source.

This lives in the caller rather than as an input to the shared workflow
because no other tabnas repo has a nested module. If a second one
appears, move it upstream as a `go-test-dirs` input rather than copying
it.

## Note

Most CI behaviour (the OS matrix, Node and Go versions, `core.autocrlf
false`, the sibling-linking that makes cross-repo changes testable
before release) lives in the shared reusable workflow — change it in
`tabnas/.github` rather than staging a local override.

Everything CI runs is runnable locally:

```bash
make test         # ts/, go/, go/adder/, rs/ and rs/adder/
make vet          # go vet over both modules
ci/rust/run.sh    # the Rust gate exactly as the staged workflow runs it
```
