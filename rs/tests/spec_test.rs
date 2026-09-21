// The fixture loader. `go/spec_test.go` and `ts/test/spec.test.js` assert
// the same row count and the same line numbers against the same file.
// Those two numbers are the whole agreement: if the runtimes disagree
// about what a row IS, nothing a row says can be trusted.

mod common;

use std::fs;

use tabnas_support::{find_spec_dir, load_spec, load_spec_dir, parse_spec, Column, SpecOptions};

fn loader_rows() -> tabnas_support::SpecFile {
    load_spec(
        common::spec_dir().join("util").join("loader-rows.tsv"),
        &SpecOptions::default(),
    )
    .expect("loader-rows.tsv loads")
}

#[test]
fn skips_the_header_blank_lines_and_tab_free_comments() {
    let spec = loader_rows();
    assert_eq!(
        *spec.header,
        vec!["input".to_string(), "expected".to_string()]
    );
    assert_eq!(spec.file, "loader-rows.tsv");

    // The file's own layout, asserted by line number: header on 1,
    // comments on 2 to 4, then a row, a blank, and the rest.
    let lines: Vec<usize> = spec.rows.iter().map(|row| row.line).collect();
    assert_eq!(lines, vec![5, 7, 8, 9, 10]);
    let indexes: Vec<usize> = spec.rows.iter().map(|row| row.index).collect();
    assert_eq!(indexes, vec![0, 1, 2, 3, 4]);
}

#[test]
fn treats_a_hash_leading_line_with_a_tab_as_data() {
    let spec = loader_rows();
    let row = spec.rows.iter().find(|row| row.line == 7).expect("line 7");
    assert_eq!(row.col(0), "#hash");
    assert_eq!(row.col(1), "2");
}

#[test]
fn keeps_an_empty_leading_column() {
    let spec = loader_rows();
    let row = spec.rows.iter().find(|row| row.line == 8).expect("line 8");
    assert_eq!(row.col(0), "");
    assert_eq!(row.col(1), "3");
}

#[test]
fn returns_columns_raw_and_decodes_only_on_request() {
    let spec = loader_rows();
    let row = spec.rows.iter().find(|row| row.line == 9).expect("line 9");
    // Raw: the two characters backslash and t.
    assert_eq!(row.col(0), "b\\tc");
    // Decoded: a real tab.
    assert_eq!(row.unesc(0), "b\tc");
}

#[test]
fn allows_a_row_with_more_columns_than_the_header_names() {
    let spec = loader_rows();
    let row = spec
        .rows
        .iter()
        .find(|row| row.line == 10)
        .expect("line 10");
    assert_eq!(row.cols.len(), 3);
    assert_eq!(row.col(2), "6");
}

#[test]
fn reads_a_column_out_of_range_as_empty_not_as_a_crash() {
    let spec = loader_rows();
    let row = &spec.rows[0];
    assert_eq!(row.col(9), "");
    assert_eq!(row.unesc(9), "");
    assert_eq!(row.named("nosuch"), "");
    assert_eq!(row.index_of("nosuch"), None);
}

#[test]
fn reads_columns_by_header_name() {
    let spec = loader_rows();
    let row = &spec.rows[0];
    assert_eq!(row.named("input"), "a");
    assert_eq!(row.named("expected"), "1");
    assert_eq!(row.index_of("expected"), Some(1));
    assert_eq!(row.resolve(&Column::from("expected")).expect("resolves"), 1);
    assert_eq!(row.resolve(&Column::from(3usize)).expect("resolves"), 3);
    assert_eq!(row.location(), "loader-rows.tsv:5");
}

#[test]
fn an_unknown_column_name_is_an_error_not_a_silent_read() {
    let spec = loader_rows();
    let error = spec.rows[0]
        .resolve(&Column::from("nosuch"))
        .expect_err("unknown name");
    assert!(
        error.0.contains("no column named 'nosuch'"),
        "unexpected message: {error}"
    );
}

#[test]
fn strips_a_bom_and_crlf_endings() {
    let spec = parse_spec(
        "x.tsv",
        "\u{FEFF}input\texpected\r\na\t1\r\nb\t2\r",
        &SpecOptions::default(),
    )
    .expect("parses");
    assert_eq!(
        *spec.header,
        vec!["input".to_string(), "expected".to_string()]
    );
    assert_eq!(spec.rows.len(), 2);
    assert_eq!(spec.rows[1].col(1), "2");
}

#[test]
fn rejects_a_row_narrower_than_min_cols() {
    let error = parse_spec(
        "n.tsv",
        "input\texpected\nonly\n",
        &SpecOptions {
            min_cols: 2,
            ..SpecOptions::default()
        },
    )
    .expect_err("too narrow");
    assert!(error.0.contains("n.tsv:2"), "{error}");
    assert!(error.0.contains("expected at least 2"), "{error}");
}

#[test]
fn header_and_comment_handling_can_be_turned_off() {
    let spec = parse_spec(
        "raw.tsv",
        "#a\tb\nc\td\n",
        &SpecOptions {
            header: false,
            comment: false,
            min_cols: 1,
        },
    )
    .expect("parses");
    assert!(spec.header.is_empty());
    assert_eq!(spec.rows.len(), 2);
    assert_eq!(spec.rows[0].col(0), "#a");
}

#[test]
fn loads_a_directory_sorted_by_name_and_finds_the_spec_dir() {
    let dir = common::spec_dir();
    let specs = load_spec_dir(dir.join("util"), &SpecOptions::default()).expect("loads");
    let names: Vec<&str> = specs.iter().map(|spec| spec.file.as_str()).collect();
    let mut sorted = names.clone();
    sorted.sort();
    assert_eq!(names, sorted);
    assert!(names.contains(&"codec.tsv"));

    assert!(find_spec_dir(Some(&dir.join("util")))
        .expect("found")
        .ends_with("test/spec"));
}

#[test]
fn an_empty_or_missing_directory_is_an_error() {
    let empty = std::env::temp_dir().join(format!("tabnas-support-empty-{}", std::process::id()));
    fs::create_dir_all(&empty).expect("temp dir");
    let error = load_spec_dir(&empty, &SpecOptions::default()).expect_err("empty");
    assert!(error.0.contains("no .tsv fixtures"), "{error}");
    fs::remove_dir_all(&empty).ok();

    let error = load_spec_dir(empty.join("nowhere"), &SpecOptions::default()).expect_err("missing");
    assert!(error.0.contains("spec directory not found"), "{error}");

    let error = load_spec(empty.join("nowhere.tsv"), &SpecOptions::default()).expect_err("missing");
    assert!(error.0.contains("spec file not found"), "{error}");
}

#[cfg(unix)]
#[test]
fn a_symlinked_fixture_is_loaded_and_a_dangling_one_is_an_error() {
    use std::os::unix::fs::symlink;

    let dir = std::env::temp_dir().join(format!("tabnas-support-links-{}", std::process::id()));
    fs::create_dir_all(&dir).expect("temp dir");
    fs::write(dir.join("plain.tsv"), "input\texpected\na\t1\n").expect("write");
    symlink(dir.join("plain.tsv"), dir.join("linked.tsv")).expect("link");

    let specs = load_spec_dir(&dir, &SpecOptions::default()).expect("loads");
    let names: Vec<&str> = specs.iter().map(|spec| spec.file.as_str()).collect();
    assert_eq!(names, vec!["linked.tsv", "plain.tsv"]);

    symlink(dir.join("gone.tsv"), dir.join("dangling.tsv")).expect("link");
    let error = load_spec_dir(&dir, &SpecOptions::default()).expect_err("dangling");
    assert!(error.0.contains("dangling.tsv"), "{error}");

    fs::remove_dir_all(&dir).ok();
}
