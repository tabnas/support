# ci/

The scripts the CI workflows run, and what each promoted workflow does.

To change CI, edit `.github/workflows/` in a reviewed pull request.
Session credentials push workflow files (admin `DECISIONS.md` ADR-8, as
amended 2026-09-24), so staging a workflow here first for a maintainer
to promote is optional. Sessions still cannot push tags, so a maintainer
pushes any tag that a tag-triggered workflow needs.

Some of the workflows are maintained in admin as well, and an edit made
only in this repository does not last. A workflow with a template in
admin `rollout/workflows/`, named `support__<file>`, changes in that
template too, in a pull request to admin. Today that is `ci.yml`,
`release.yml`, `crates-release.yml`, `github-release.yml` and
`deps-gate.yml`. Admin `scripts/verify.sh` reports a deployed copy that
differs from its template, and the next
`rollout/apply-workflows.sh --apply` writes the template back over it.

Every workflow once staged here has been promoted, and the rollout
deleted each staged copy as it went, so `ci/` holds only this file and
the Rust gate script, `rust/run.sh`.

## Promoted

All four live in `.github/workflows/`:

- **`ci.yml`** — the org-standard thin caller delegating to
  `tabnas/.github/.github/workflows/polyglot-ci.yml@main` with
  `deps: "parser"`, plus the repo-specific `go-adder` job described below.
- **`release.yml`** — publishes `@tabnas/support` to npm via GitHub OIDC
  Trusted Publishing, on a `workflow_dispatch` from `main` (the normal
  path, which also writes the release tags) or on a `ts/v*` tag push
  (the orchestrator's path, where the tag steps do nothing). No
  `NPM_TOKEN`, no secret in this repo. It tracks the `release.yml` in
  `json`, `jsonic` and `expr` from `name:` onward, differing only in the
  package name its `npm view` calls name, and so creates the same two
  tags they do: `ts/v$V` and `go/v$V`.

  It creates no tag for `go/adder`. That nested module is a private
  internal test module, never tagged or published, by the maintainer's
  decision (admin#19); the `go-adder` job below builds it from source,
  and nothing outside this repository consumes it. Its two old tags,
  v0.2.0 and v0.3.0, predate the decision and stay, since a Go tag is
  immutable once the proxy has served it
  ([#21](https://github.com/tabnas/support/issues/21)).
  `ts/test/release.test.js` fails if the deployed file, or a staged
  candidate whenever there is one, names `go/adder` outside a comment.

- **`rust.yml`** — the Rust gate: `ci/rust/run.sh` over the `rs/` crate
  and the `rs/adder/` crate (formatting, both lockfiles, build, tests,
  doctests, clippy), under the MSRV toolchain, with `tabnas/parser`
  `main` cloned as a sibling because the adder crate takes the engine by
  path. Standalone rather than an arm of `ci.yml`, since the shared
  polyglot workflow takes no Rust input. Runs on `rs/**`, `test/spec/**`,
  `ts/package.json`, `ci/rust/**` and its own file.

- **`docs.yml`** — the prose gate: Vale over the reader-facing pages at
  the levels set in `.vale.ini`, on the file list
  `ts/scripts/gated-docs.cjs` produces, then `ts/scripts/vale-counts.cjs`
  over the counts recorded for it. See `docs/STYLE-GUIDE.md`.

  It needs no sibling checkouts and no secrets, and pins its own Vale
  version. Errors fail the job; warnings go to the run summary as a
  report. `make prose` runs the identical check locally, and the other
  half of the gate (`ts/test/docs.test.js`) runs in `make test` and in
  `ci.yml`. Runs on the gated pages, the style guide, the Vale
  configuration, the two scripts and its own file.

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
`tabnas/.github` rather than adding a local override.

Everything CI runs is runnable locally except `deps-gate.yml`, whose script
is inline in the reusable workflow in `tabnas/.github` rather than in this
checkout:

```bash
make test         # ts/, go/, go/adder/, rs/ and rs/adder/
make vet          # go vet over both modules
ci/rust/run.sh    # the Rust gate exactly as rust.yml runs it
```
