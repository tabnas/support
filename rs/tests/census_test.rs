// The code census over the shared fixtures, and the static tripwire that
// every named-file fixture is wired into this runtime's suite.
// `ts/test/census.test.js` and `go/census_test.go` assert the same.

mod common;

use std::collections::HashMap;
use std::fs;
use std::path::Path;

use tabnas_support::{codes_in_spec_dir, compare_catalogues, coverage, CensusOptions};

#[test]
fn spec_census_codes_and_named_col() {
    // The default: the expectation is each row's last column. Over the
    // census directory that reads codes.tsv correctly and the trap column
    // of named-col.tsv, which is the bait the next assertion avoids.
    let dir = common::spec_dir().join("census");

    let by_name = codes_in_spec_dir(
        &dir,
        &CensusOptions {
            name: Some("expected".into()),
            col: None,
        },
    )
    .expect("census");
    assert_eq!(
        by_name,
        vec![
            "named_only",
            "positioned",
            "unexpected",
            "unterminated_string"
        ]
    );

    let by_col = codes_in_spec_dir(
        &dir,
        &CensusOptions {
            col: Some(1),
            name: None,
        },
    )
    .expect("census");
    assert_eq!(by_col, by_name);

    let last_column = codes_in_spec_dir(&dir, &CensusOptions::default()).expect("census");
    assert!(last_column.contains(&"trap_note".to_string()));
    assert!(!last_column.contains(&"named_only".to_string()));
}

#[test]
fn an_unknown_column_name_is_an_error() {
    let error = codes_in_spec_dir(
        common::spec_dir().join("census"),
        &CensusOptions {
            name: Some("nosuch".into()),
            col: None,
        },
    )
    .expect_err("unknown column");
    assert!(error.0.contains("no column named \"nosuch\""), "{error}");
}

#[test]
fn compares_catalogues_byte_for_byte() {
    let a: HashMap<String, String> = [("x", "1"), ("y", "2"), ("z", "3")]
        .into_iter()
        .map(|(k, v)| (k.to_string(), v.to_string()))
        .collect();
    let b: HashMap<String, String> = [("y", "2"), ("z", "three"), ("w", "4")]
        .into_iter()
        .map(|(k, v)| (k.to_string(), v.to_string()))
        .collect();
    let diff = compare_catalogues(&a, &b);
    assert_eq!(diff.missing, vec!["x"]);
    assert_eq!(diff.extra, vec!["w"]);
    assert_eq!(diff.template_mismatch, vec!["z"]);
}

#[test]
fn reports_uncovered_and_orphan_codes() {
    let declared = vec!["a".to_string(), "b".to_string()];
    let exercised = vec!["b".to_string(), "c".to_string()];
    let report = coverage(&declared, &exercised);
    assert_eq!(report.uncovered, vec!["a"]);
    assert_eq!(report.orphan, vec!["c"]);
}

#[test]
fn names_every_named_file_fixture_in_this_runtime() {
    // `util/`, `census/` and `register/` are named-file families: each
    // fixture has its own column shape, so each suite names the files it
    // runs, and this tripwire is what notices one wired into a single
    // runtime. It is static rather than proof: a name in a comment would
    // satisfy it. What it catches is the realistic mistake.
    let tests = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests");
    let sources: String = fs::read_dir(&tests)
        .expect("tests dir")
        .filter_map(|entry| entry.ok())
        .filter(|entry| entry.path().extension().is_some_and(|ext| ext == "rs"))
        .map(|entry| fs::read_to_string(entry.path()).expect("readable"))
        .collect::<Vec<_>>()
        .join("\n");

    for family in ["census", "register", "util"] {
        let family_dir = common::spec_dir().join(family);
        let mut fixtures: Vec<String> = fs::read_dir(&family_dir)
            .expect("family dir")
            .filter_map(|entry| entry.ok())
            .map(|entry| entry.file_name().to_string_lossy().into_owned())
            .filter(|name| name.ends_with(".tsv"))
            .collect();
        fixtures.sort();
        assert!(
            !fixtures.is_empty(),
            "no fixtures in {}",
            family_dir.display()
        );

        let missing: Vec<&String> = fixtures
            .iter()
            .filter(|name| !sources.contains(name.as_str()))
            .collect();
        assert!(
            missing.is_empty(),
            "fixture(s) not named by any test in rs/tests/: {missing:?}; wire them in here AND in ts/ and go/, or the row is agreed by nobody"
        );
    }
}
