// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"fmt"
	"regexp"
	"sort"
)

// census.go — coverage and parity tripwires over error codes.
//
// Every tabnas package declares a set of error codes, renders each
// through a {code: template} message catalogue, and pins its rejections
// in fixtures as ERROR:<code> rows. Three lists, three ways to drift: a
// declared code no fixture ever exercises, two catalogues whose keys or
// templates have come apart, a fixture pinning a code nobody declares.
// These helpers compute those gaps from real loaded data.
//
// Every input arrives as an argument. Nothing here fetches a catalogue
// or imports the engine — this module has no dependencies, and never
// will — and that is exactly why these tripwires can live in the one
// module the repos already share.
//
// ts/src/census.ts mirrors all of this.

// codeToken matches what a code-style expectation names: a bare
// lowercase token, which is how every tabnas error code is spelt.
// "ERROR:bad token" and "ERROR:1:8" are message- and position-style
// expectations — real rejections, but not codes, and a census that
// returned them would count coverage that is not there.
var codeToken = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// CensusOpts says which column holds the expectation. The default is
// each row's last column — the ordinary two-column input/expected
// fixture needs no selection at all — but a wider fixture can keep the
// expectation anywhere, which is why the selection exists.
type CensusOpts struct {
	// Col selects the expectation column by position. A pointer for the
	// same reason Runner.Expected is one: 0 is a real column, and a plain
	// int could not tell it from "not set". Int(0) builds one.
	Col *int

	// Name selects the expectation column by header name, and wins when
	// set.
	Name string
}

// CodesInSpecDir collects every code named by a code-style expectation
// cell under a fixture directory: sorted, unique. Message-style
// expectations and bare ERROR cells assert a rejection without naming a
// code, so they are not collected; a value row is not an expectation at
// all.
//
// The directory is read with the shared loader, so an empty directory
// fails here the way it fails everywhere else — a census over nothing
// must not report "no codes" as if it had looked.
func CodesInSpecDir(dir string, opts CensusOpts) ([]string, error) {
	specs, err := LoadSpecDir(dir, nil)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	for _, spec := range specs {
		for _, row := range spec.Rows {
			col, err := expectationCol(spec, row, opts)
			if err != nil {
				return nil, err
			}

			cell := row.Col(col)
			if !IsErrorExpect(cell) {
				continue
			}

			code, err := ErrorCode(cell)
			if err != nil {
				return nil, err
			}
			if codeToken.MatchString(code) {
				seen[code] = true
			}
		}
	}

	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	return codes, nil
}

// expectationCol resolves the expectation column for one row: the named
// column when Name is set, else the given position, else the row's own
// last column. An unknown name is an error — that is a defect in the
// caller, and a census silently reading the wrong column would report
// coverage nobody has.
func expectationCol(spec *File, row *Row, opts CensusOpts) (int, error) {
	if "" != opts.Name {
		i := row.IndexOf(opts.Name)
		if -1 == i {
			return -1, fmt.Errorf("%s: no column named %q (header: %v)",
				spec.Name, opts.Name, spec.Header)
		}
		return i, nil
	}
	if nil != opts.Col {
		return *opts.Col, nil
	}
	return len(row.Cols) - 1, nil
}

// CatalogueDiff is what CompareCatalogues found. All three lists are
// sorted, and empty rather than nil when there is nothing to report, so
// the two runtimes render the same answer the same way.
type CatalogueDiff struct {
	// Missing holds keys of a absent in b.
	Missing []string

	// Extra holds keys of b absent in a.
	Extra []string

	// TemplateMismatch holds shared keys whose templates differ.
	TemplateMismatch []string
}

// CompareCatalogues diffs two {code: template} catalogues — message
// catalogues, hint catalogues, or one runtime's against the other's.
// Templates compare byte for byte: two templates that merely "mean the
// same" have still drifted, and the byte diff is what a maintainer has
// to reconcile.
func CompareCatalogues(a, b map[string]string) CatalogueDiff {
	missing := []string{}
	extra := []string{}
	templateMismatch := []string{}

	for code, template := range a {
		other, ok := b[code]
		if !ok {
			missing = append(missing, code)
		} else if template != other {
			templateMismatch = append(templateMismatch, code)
		}
	}

	for code := range b {
		if _, ok := a[code]; !ok {
			extra = append(extra, code)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(templateMismatch)

	return CatalogueDiff{
		Missing:          missing,
		Extra:            extra,
		TemplateMismatch: templateMismatch,
	}
}

// CoverageReport is what Coverage found. Both lists are sorted, and
// empty rather than nil when there is nothing to report.
type CoverageReport struct {
	// Uncovered holds declared codes exercised by no fixture.
	Uncovered []string

	// Orphan holds exercised codes declared by nobody.
	Orphan []string
}

// Coverage compares the codes a package declares against the codes its
// fixtures exercise (typically CodesInSpecDir's answer). Whether
// inherited base codes count as declared is the caller's choice — pass
// them in or leave them out.
//
// An uncovered code is a rejection nobody has pinned; an orphan is a
// fixture pinning a code the package does not declare — a misspelt
// code, or one that has since been removed. Both lists empty is what
// "the fixtures and the declarations agree" means.
func Coverage(declared, exercised []string) CoverageReport {
	dset := map[string]bool{}
	for _, code := range declared {
		dset[code] = true
	}
	eset := map[string]bool{}
	for _, code := range exercised {
		eset[code] = true
	}

	uncovered := []string{}
	for code := range dset {
		if !eset[code] {
			uncovered = append(uncovered, code)
		}
	}
	orphan := []string{}
	for code := range eset {
		if !dset[code] {
			orphan = append(orphan, code)
		}
	}

	sort.Strings(uncovered)
	sort.Strings(orphan)

	return CoverageReport{Uncovered: uncovered, Orphan: orphan}
}
