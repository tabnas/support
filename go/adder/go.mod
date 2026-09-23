// The adder grammar is a SEPARATE module from github.com/tabnas/support/go
// on purpose. The support module is a dependency of every tabnas repo, so
// it carries no dependencies of its own; this one needs the parser, and
// splitting them is what lets both facts hold — and what keeps the two
// repos from requiring each other.
//
// `go test ./...` in go/ does not descend here; the Makefile runs it.
//
// It is a PRIVATE INTERNAL TEST MODULE: never tagged and never published,
// by the maintainer's decision (admin#19; see AGENTS.md#release). Only
// this repository builds it, from source. Do not add a go/adder tag.
module github.com/tabnas/support/go/adder

go 1.24.7

// The support requirement is a REAL published version, not a v0.0.0
// placeholder. The replace below is what the local build uses, but a
// replace in a dependency module is ignored by whoever imports it. No
// release tags this module, yet the repository is public, so Go can still
// fetch any commit of it as a pseudo-version, and that fetch resolves the
// version named here: a placeholder would fail before anything compiled.
// Bump it with the support module's version at every release
// (`make version`).
require (
	github.com/tabnas/parser/go v0.9.0
	github.com/tabnas/support/go v0.3.4
)

replace github.com/tabnas/support/go => ../
