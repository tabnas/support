// Copyright (c) 2026 tabnas, MIT License

// Package tabnassupport holds the shared test-support utilities for the
// tabnas parser system: the TSV spec-fixture loader, the escape codec,
// the expectation helpers and the table-driven test runner.
//
// Every tabnas package (parser, json, jsonic, abnf, csv, ...) proves its
// two runtimes agree by running one set of TSV fixtures from test/spec/
// in both. This package is the machinery that reads those fixtures, so
// there is one loader with one set of rules instead of a copy per repo
// quietly drifting from its TypeScript twin.
//
// The TypeScript half is @tabnas/support, and the two are written to
// behave identically — same escape codec, same comment and blank-line
// handling, same ERROR:<code> contract, same value comparison. Where a
// difference is unavoidable it is documented at the point it appears.
//
// A typical Go suite is three lines:
//
//	dir, _ := tabnassupport.FindSpecDir("")
//	tabnassupport.Runner{Parse: tn.Parse}.Dir(t, filepath.Join(dir, "adder"))
//
// This module has no dependencies, and never will: every tabnas repo
// depends on it, so anything it required would land in all of them. The
// adder grammar that exercises it end to end therefore lives in the
// separate go/adder module, which may depend on the parser.
package tabnassupport

// VERSION is the release version, kept in step with ts/package.json —
// see version_test.go, which fails the build when they drift.
const VERSION = "0.3.1"

// Bool returns a pointer to b, for the *bool fields in Options. Go has no
// literal address-of, and a helper reads better than a named temporary at
// every call site (the same helper the parser's option structs need).
func Bool(b bool) *bool { return &b }

// Int returns a pointer to i, for the *int fields in Runner. A pointer is
// what distinguishes "column 0" from "not set", and column 0 is a real
// answer.
func Int(i int) *int { return &i }
