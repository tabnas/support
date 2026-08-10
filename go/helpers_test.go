// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"testing"
	"unicode/utf8"
)

// helpers_test.go — the small shared helpers the suites in this package
// use, so no single suite owns them.

// specDir returns this repo's shared fixture directory, found by walking
// up from the package directory (the working directory under `go test`).
func specDir(t *testing.T) string {
	t.Helper()

	dir, err := FindSpecDir("")
	if err != nil {
		t.Fatalf("%v", err)
	}
	return dir
}

// mustLoad loads a fixture file or fails the test.
func mustLoad(t *testing.T, path string) *File {
	t.Helper()

	spec, err := LoadSpec(path, nil)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return spec
}

// mustParse parses fixture text or fails the test.
func mustParse(t *testing.T, name, text string, opts *Options) *File {
	t.Helper()

	spec, err := ParseSpec(name, text, opts)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return spec
}

func isValidUTF8(s string) bool { return utf8.ValidString(s) }

// codeError is a stand-in for *tabnas.TabnasError: an error carrying a
// string Code field, which is the shape Runner reads without importing
// the parser.
type codeError struct {
	Code string
	Msg  string
}

func (e *codeError) Error() string { return e.Msg }

// methodError carries its code behind a method instead of a field.
type methodError struct{ code string }

func (e *methodError) Error() string { return e.code }
func (e *methodError) Code() string  { return e.code }
