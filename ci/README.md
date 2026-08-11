# ci/

Staging area for GitHub Actions workflow changes.

This directory exists because session credentials cannot write
`.github/workflows/*` — see admin `DECISIONS.md` ADR-8. To change CI:

1. Put the intended workflow file in `workflows/`.
2. A maintainer promotes it with the admin `rollout/apply-ci-folders.sh`
   script.

## Pending

**Neither file is promoted yet. This repo currently has no CI and no
release automation at all**, so nothing here is verified on push or PR,
and no tag publishes anything, until a maintainer moves both files into
`.github/workflows/`.

### `workflows/release.yml`

Publishes `@tabnas/support` to npm on a `ts/v*` tag push, via GitHub OIDC
Trusted Publishing — no `NPM_TOKEN`, no secret in this repo at all. It is
**byte-identical** to the `release.yml` in `parser`, `json`, `jsonic` and
`expr` from `name:` onward, so promoting it is a copy rather than a
review.

Two things follow from the trigger being `ts/v*`:

- A plain `vX.Y.Z` tag publishes **nothing**. `npm run repo-tag` in
  `ts/package.json` creates exactly that (it is the org-wide script, and
  it predates the release workflow), which is how `v0.1.1` came to exist
  here with no `ts/v0.1.1` beside it. Release through the orchestrator,
  or tag `ts/vX.Y.Z` by hand.
- The Go modules are not published by any workflow. The module proxy
  serves them from their tags directly, and `make publish-go` creates
  both `go/vX.Y.Z` and `go/adder/vX.Y.Z`.

### `workflows/ci.yml`

It has two jobs:

- **`ci`** — the org-standard thin caller that delegates to
  `tabnas/.github/.github/workflows/polyglot-ci.yml@main`, with
  `deps: "parser"`. `build-order` is omitted because the default,
  `"<deps> <this repo>"`, is already `"parser support"`.

  `@tabnas/parser` is a devDependency of `ts/` (for the adder plugin and
  its tests) and the dependency of the `go/adder` module. The support
  module itself has no dependencies and never will — every tabnas repo
  depends on it, so anything it required would land in all of them.

- **`go-adder`** — a repo-specific job the shared workflow cannot cover.

  The shared workflow runs `go test ./...` in `go/` only. `go/adder` is a
  **separate module** — that is what keeps the support module
  dependency-free — and `./...` does not cross a module boundary, so
  without this job the adder suite would silently not run. That suite is
  the end-to-end check that the two runtimes agree, and a test that
  quietly does not run reports a green tick that is a lie.

  It clones `parser` as a sibling and wires it in through a `go.work`, so
  the grammar is tested against parser *source* rather than the published
  module — same reasoning as the shared workflow's own sibling clones.
  The workspace also covers the support module itself, whose version
  `go/adder/go.mod` requires may not be published yet.

  This is a job in the caller rather than an input to the shared
  workflow because no other tabnas repo has a nested module. If a second
  one appears, move it upstream as a `go-test-dirs` input rather than
  copying it.

  It sets `cache: false` on `setup-go`. The cache step looks for a
  `go.mod` in `$GITHUB_WORKSPACE` — non-recursively, and without walking
  up — and `checkout` puts the repo one level down, so it finds nothing
  and annotates every run with "Restore cache failed". setup-go catches
  that and carries on, so it is a warning rather than a failure (the
  shared workflow's own go job has the same shape and runs fine), but a
  warning that cries wolf on every run is how the real ones get ignored.
  There is little to cache in any case: the support module has no
  dependencies, `go/adder/go.sum` pins exactly one, and the workspace
  overrides it with sibling source.

## Note

Most CI behaviour (the OS matrix, Node and Go versions, `core.autocrlf
false`, the sibling-linking that makes cross-repo changes testable
before release) lives in the shared reusable workflow, not here — change
it in `tabnas/.github` rather than staging a local override.

Everything the workflow runs is runnable locally today:

```bash
make test    # ts/, go/ and go/adder/
make vet     # go vet over both modules
```
