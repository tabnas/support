// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// census_test.go — every shared fixture runs in BOTH runtimes.
//
// test/spec/adder/ needs no check: both runtimes discover it by
// directory listing, so a fixture added there runs in both without
// anyone touching a runner.
//
// test/spec/util/ and test/spec/census/ cannot work that way — each file
// has its own column shape and its own assertion, so each suite names
// the files it runs. That leaves a gap the rest of the suite cannot see:
// a fixture added to disk and wired into ONE runtime passes everywhere,
// and the row it pins is then agreed by nobody.
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
	censusNamesEveryFixture(t, "util")
}

func TestCensusNamesEveryCensusFixture(t *testing.T) {
	censusNamesEveryFixture(t, "census")
}

// Added in the same change that created test/spec/register. A new family
// that is not listed here is invisible to the tripwire, so the safeguard
// would pass while a fixture ran in one port only — which is the
// safeguard's whole subject.
func TestCensusNamesEveryRegisterFixture(t *testing.T) {
	censusNamesEveryFixture(t, "register")
}

// censusNamesEveryFixture is the tripwire itself, shared by the
// named-file families: every *.tsv in the family's directory must be
// named somewhere in this runtime's test sources.
func censusNamesEveryFixture(t *testing.T, family string) {
	t.Helper()

	familyDir := filepath.Join(specDir(t), family)

	entries, err := os.ReadDir(familyDir)
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
		t.Fatalf("no fixtures in %s", familyDir)
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

// The census helpers, over the test/spec/census/ fixtures: codes.tsv is
// the ordinary two-column shape with the expectation last; named-col.tsv
// is three-column with the expectation in the middle, so its trailing
// note column can bait the default column selection.
// ts/test/census.test.js asserts the same lists.

func TestCodesInSpecDir(t *testing.T) {
	dir := filepath.Join(specDir(t), "census")

	// With the expectation column named, both files are read right:
	// codes.tsv contributes unexpected and unterminated_string (its
	// message-style, bare-ERROR and value rows contribute nothing), and
	// named-col.tsv contributes named_only plus a repeat of unexpected.
	want := []string{"named_only", "unexpected", "unterminated_string"}

	got, err := CodesInSpecDir(dir, CensusOpts{Name: "expected"})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Name: got %v, want %v", got, want)
	}

	// Column 1 is `expected` in both files, so the answer is the same.
	got, err = CodesInSpecDir(dir, CensusOpts{Col: Int(1)})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Col: got %v, want %v", got, want)
	}

	// The default is each row's last column. codes.tsv's last column IS
	// the expectation; named-col.tsv's is note, whose code-shaped bait is
	// then collected and whose real codes are missed — the mistake the
	// column selection exists to prevent, pinned so it stays visible.
	wantLast := []string{
		"another_trap", "trap_note", "unexpected", "unterminated_string",
	}
	got, err = CodesInSpecDir(dir, CensusOpts{})
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !reflect.DeepEqual(wantLast, got) {
		t.Errorf("default: got %v, want %v", got, wantLast)
	}
}

func TestCodesInSpecDirRejects(t *testing.T) {
	dir := filepath.Join(specDir(t), "census")

	// An unknown column name is a defect in the caller, not "no codes".
	_, err := CodesInSpecDir(dir, CensusOpts{Name: "nope"})
	if nil == err || !strings.Contains(err.Error(), "no column named") {
		t.Errorf("unknown name: err = %v", err)
	}

	// The shared loader's guard: a census over nothing must not report
	// "no codes" as if it had looked.
	_, err = CodesInSpecDir(filepath.Join(specDir(t), "no-such-dir"),
		CensusOpts{})
	if nil == err || !strings.Contains(err.Error(), "spec directory not found") {
		t.Errorf("missing dir: err = %v", err)
	}
}

func TestCompareCatalogues(t *testing.T) {
	empty := CatalogueDiff{
		Missing:          []string{},
		Extra:            []string{},
		TemplateMismatch: []string{},
	}

	// Identical catalogues are identical.
	cat := map[string]string{
		"unexpected": "unexpected {token}", "unprintable": "oops",
	}
	if got := CompareCatalogues(cat, cat); !reflect.DeepEqual(empty, got) {
		t.Errorf("identical: got %+v", got)
	}
	if got := CompareCatalogues(nil, nil); !reflect.DeepEqual(empty, got) {
		t.Errorf("empty: got %+v", got)
	}

	// A missing key, an extra key and a changed template, each detected.
	got := CompareCatalogues(
		map[string]string{"alpha": "A", "beta": "B", "gamma": "C"},
		map[string]string{"beta": "B!", "gamma": "C", "delta": "D"})
	want := CatalogueDiff{
		Missing:          []string{"alpha"},
		Extra:            []string{"delta"},
		TemplateMismatch: []string{"beta"},
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// All three lists are sorted, whatever order the maps iterate in.
	some := map[string]string{"zz": "1", "mm": "1", "aa": "1"}
	drifted := map[string]string{"zz": "2", "mm": "2", "aa": "2"}
	sorted := []string{"aa", "mm", "zz"}
	if got := CompareCatalogues(some, nil).Missing; !reflect.DeepEqual(sorted, got) {
		t.Errorf("Missing: got %v", got)
	}
	if got := CompareCatalogues(nil, some).Extra; !reflect.DeepEqual(sorted, got) {
		t.Errorf("Extra: got %v", got)
	}
	if got := CompareCatalogues(some, drifted).TemplateMismatch; !reflect.DeepEqual(sorted, got) {
		t.Errorf("TemplateMismatch: got %v", got)
	}

	// Templates compare byte for byte: one space of drift is still
	// drift — "means the same" is not the contract, identical bytes is.
	got = CompareCatalogues(
		map[string]string{"near": "near {x}"},
		map[string]string{"near": "near  {x}"})
	if !reflect.DeepEqual([]string{"near"}, got.TemplateMismatch) {
		t.Errorf("byte-for-byte: got %+v", got)
	}
}

// TestCensusSortsByCodePoint pins the order both runtimes give a
// non-BMP key. sort.Strings compares UTF-8 bytes, which for valid UTF-8
// is code-point order, and puts "\U00010000" after "�"; TypeScript's
// default Array sort compares UTF-16 code units, which would put the
// surrogate pair FIRST, so the TS census sorts by code point to match.
// ts/test/census.test.js pins the same keys.
func TestCensusSortsByCodePoint(t *testing.T) {
	want := []string{"�", "\U00010000"}

	got := CompareCatalogues(
		map[string]string{"\U00010000": "x", "�": "y"}, nil).Missing
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Missing: got %q, want %q", got, want)
	}

	cov := Coverage([]string{"\U00010000", "�"}, nil).Uncovered
	if !reflect.DeepEqual(want, cov) {
		t.Errorf("Uncovered: got %q, want %q", cov, want)
	}
}

func TestCoverage(t *testing.T) {
	// A clean package is clean.
	got := Coverage([]string{"a", "b"}, []string{"b", "a"})
	want := CoverageReport{Uncovered: []string{}, Orphan: []string{}}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("clean: got %+v", got)
	}

	// Uncovered and orphan codes, sorted.
	got = Coverage(
		[]string{"unterminated_string", "unexpected", "unprintable"},
		[]string{"unexpected", "mystery", "another"})
	want = CoverageReport{
		Uncovered: []string{"unprintable", "unterminated_string"},
		Orphan:    []string{"another", "mystery"},
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// TestCoverageTiesOutAgainstCodeCensus feeds CodesInSpecDir's answer to
// Coverage, the way a consuming repo would.
func TestCoverageTiesOutAgainstCodeCensus(t *testing.T) {
	exercised, err := CodesInSpecDir(filepath.Join(specDir(t), "census"),
		CensusOpts{Name: "expected"})
	if err != nil {
		t.Fatalf("%v", err)
	}

	got := Coverage(
		[]string{"named_only", "unexpected", "unreached", "unterminated_string"},
		exercised)
	want := CoverageReport{
		Uncovered: []string{"unreached"},
		Orphan:    []string{},
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("got %+v, want %+v", got, want)
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
