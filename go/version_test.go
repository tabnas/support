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
// The Go module has no package.json of its own, so it checks its constant
// against ts/package.json on disk. That leaves the published npm version
// as the single source of truth for both, and turns a half-done release
// into a failing build rather than a silent mismatch.

func TestVersionMatchesPackageJSON(t *testing.T) {
	repo, err := FindSpecDir("")
	if err != nil {
		t.Fatalf("%v", err)
	}
	// FindSpecDir returns <repo>/test/spec.
	pkgPath := filepath.Join(repo, "..", "..", "ts", "package.json")

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

func TestVersionIsSemverTriple(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(VERSION) {
		t.Errorf("VERSION = %q", VERSION)
	}
}
