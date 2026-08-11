# ci/

Staging area for GitHub Actions workflow changes.

**Currently empty — nothing is pending.** Both staged workflows have been
promoted and now live in `.github/workflows/`:

- **`ci.yml`** — the org-standard thin caller delegating to
  `tabnas/.github/.github/workflows/polyglot-ci.yml@main` with
  `deps: "parser"`, plus the repo-specific `go-adder` job described below.
- **`release.yml`** — publishes `@tabnas/support` to npm on a `ts/v*` tag
  push via GitHub OIDC Trusted Publishing. No `NPM_TOKEN`, no secret in
  this repo. Byte-identical to the `release.yml` in `parser`, `json`,
  `jsonic` and `expr` from `name:` onward.

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
make test    # ts/, go/ and go/adder/
make vet     # go vet over both modules
```
