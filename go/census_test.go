// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// census_test.go — every shared fixture runs in BOTH runtimes.
//
// test/spec/adder/ needs no check: both runtimes discover it by
// directory listing, so a fixture added there runs in both without
// anyone touching a runner.
//
// test/spec/util/ cannot work that way — each file has its own column
// shape and its own assertion, so each suite names the files it runs.
// That leaves a gap the rest of the suite cannot see: a fixture added to
// disk and wired into ONE runtime passes everywhere, and the row it pins
// is then agreed by nobody.
//
// This is a static tripwire rather than proof. It checks that each
// fixture's name appears somewhere in this runtime's test sources; a
// name sitting in a comment would satisfy it. What it does catch is the
// realistic mistake — adding a fixture and wiring up one side — which
// nothing else here would.
//
// ts/test/census.test.js asserts the same thing over the TypeScript
// sources.

func TestCensusNamesEveryUtilFixture(t *testing.T) {
	utilDir := filepath.Join(specDir(t), "util")

	entries, err := os.ReadDir(utilDir)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var fixtures []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tsv") {
			fixtures = append(fixtures, e.Name())
		}
	}

	// A corpus that vanished must not pass quietly either.
	if 0 == len(fixtures) {
		t.Fatalf("no fixtures in %s", utilDir)
	}

	// The working directory under `go test` is the package directory,
	// which is where this module's test sources live.
	sources, err := readTestSources(".")
	if err != nil {
		t.Fatalf("%v", err)
	}

	for _, name := range fixtures {
		if !strings.Contains(sources, name) {
			t.Errorf("fixture %q is not named by any test in go/ — wire it in "+
				"here AND in ts/, or the row is agreed by nobody", name)
		}
	}
}

// TestCensusAdderIsDiscoveredByListing records that the asymmetry above
// is deliberate rather than forgotten: the adder directory needs no
// census because adding a file to it runs it in both runtimes.
func TestCensusAdderIsDiscoveredByListing(t *testing.T) {
	adderDir := filepath.Join(specDir(t), "adder")

	entries, err := os.ReadDir(adderDir)
	if err != nil {
		t.Fatalf("%v", err)
	}

	found := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tsv") {
			found++
		}
	}
	if 0 == found {
		t.Fatalf("no fixtures in %s", adderDir)
	}

	adderTest := filepath.Join(repoRoot(t), "go", "adder", "adder_test.go")
	data, err := os.ReadFile(adderTest)
	if err != nil {
		t.Fatalf("read %s: %v", adderTest, err)
	}
	if !strings.Contains(string(data), ".Dir(t,") {
		t.Errorf("%s must run the directory, not named files", adderTest)
	}
}

// readTestSources concatenates every *_test.go file in a directory.
func readTestSources(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return "", err
		}
		b.Write(data)
		b.WriteByte('\n')
	}

	return b.String(), nil
}
