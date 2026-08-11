// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// version_test.go — the two runtimes ship one version number.
//
// It appears in FOUR places: ts/package.json, ts/src/support.ts,
// go/support.go, and the require on the support module in
// go/adder/go.mod. ts/package.json is the source of truth; the tests
// here pin the two Go-side copies to it, and ts/test/version.test.js
// pins ts/src/support.ts to it.
//
// The Go module has no package.json of its own, so it reads the
// TypeScript one on disk. That leaves the published npm version as the
// single source of truth for both, and turns a half-done release into a
// failing build rather than a silent mismatch.

// repoRoot returns the repository root — FindSpecDir gives <repo>/test/spec.
func repoRoot(t *testing.T) string {
	t.Helper()

	specDir, err := FindSpecDir("")
	if err != nil {
		t.Fatalf("%v", err)
	}
	return filepath.Join(specDir, "..", "..")
}

func TestVersionMatchesPackageJSON(t *testing.T) {
	pkgPath := filepath.Join(repoRoot(t), "ts", "package.json")

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("read %s: %v", pkgPath, err)
	}

	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatalf("parse %s: %v", pkgPath, err)
	}

	if "@tabnas/support" != pkg.Name {
		t.Errorf("package name: got %q", pkg.Name)
	}
	if VERSION != pkg.Version {
		t.Errorf("VERSION = %q, ts/package.json = %q", VERSION, pkg.Version)
	}
}

// TestVersionMatchesAdderRequire pins the fourth version site: the
// require on this module in go/adder/go.mod.
//
// It is the one place a stale version breaks nothing local. A `replace`
// covers the version for anyone building in this repo, so the tests stay
// green — but a replace in a dependency module is ignored by whoever
// imports it, so an external `go get` of the adder module resolves the
// version named there and fails. It sat at v0.1.0 through the 0.1.1
// release for exactly that reason: nothing looked.
func TestVersionMatchesAdderRequire(t *testing.T) {
	modPath := filepath.Join(repoRoot(t), "go", "adder", "go.mod")

	data, err := os.ReadFile(modPath)
	if err != nil {
		t.Fatalf("read %s: %v", modPath, err)
	}

	// The require line, in either the block form (indented) or the
	// single-line form. Deliberately not the `replace` line, which ends
	// in `=> ../` and so fails the end anchor, nor the prose in the
	// comments above it, which does not start with the module path.
	re := regexp.MustCompile(
		`(?m)^(?:require[ \t]+)?[ \t]*github\.com/tabnas/support/go[ \t]+(v\S+)[ \t]*$`)

	matches := re.FindAllStringSubmatch(string(data), -1)
	if 1 != len(matches) {
		t.Fatalf("%s: found %d require lines for the support module, want exactly 1",
			modPath, len(matches))
	}

	if want := "v" + VERSION; matches[0][1] != want {
		t.Errorf("go/adder/go.mod requires support/go %s, but VERSION is %q (want %s)",
			matches[0][1], VERSION, want)
	}
}

func TestVersionIsSemverTriple(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(VERSION) {
		t.Errorf("VERSION = %q", VERSION)
	}
}
