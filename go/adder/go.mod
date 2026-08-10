// The adder grammar is a SEPARATE module from github.com/tabnas/support/go
// on purpose. The support module is a dependency of every tabnas repo, so
// it carries no dependencies of its own; this one needs the parser, and
// splitting them is what lets both facts hold — and what keeps the two
// repos from requiring each other.
//
// `go test ./...` in go/ does not descend here; the Makefile runs it.
module github.com/tabnas/support/go/adder

go 1.24.7

require (
	github.com/tabnas/parser/go v0.8.0
	github.com/tabnas/support/go v0.0.0
)

replace github.com/tabnas/support/go => ../
