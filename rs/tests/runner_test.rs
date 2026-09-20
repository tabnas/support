// The runner's own failure behaviour.
//
// Its passing path is exercised by every other suite here simply by being
// green. What needs its own test is the other path: that a wrong answer,
// a missing failure, a wrong error code and an empty fixture each FAIL.
// A runner that quietly passes is the one bug that hides every other one.
// `ts/test/runner.test.js` and `go/runner_test.go` cover the same cases.

use tabnas_support::{parse_expect, parse_spec, Failure, Runner, SpecOptions, Value};

fn fixture() -> tabnas_support::SpecFile {
    parse_spec(
        "t.tsv",
        "input\texpected\na\t\"A\"\nb\tERROR:bad_b\nc\tERROR\n",
        &SpecOptions::default(),
    )
    .expect("parses")
}

fn upper(input: &str) -> Result<Value, Failure> {
    Ok(Value::String(input.to_uppercase()))
}

// Run one row directly and return the failure it raised, if any.
fn check(runner: &Runner, index: usize) -> Option<String> {
    let spec = fixture();
    let row = &spec.rows[index];
    runner
        .check_row(row, &row.unesc(0), row.col(1))
        .err()
        .map(|e| e.0)
}

#[test]
fn passes_a_row_whose_value_matches() {
    assert_eq!(check(&Runner::new(upper), 0), None);
}

#[test]
fn fails_a_row_whose_value_does_not_match_quoting_both_sides() {
    let message = check(&Runner::new(|_| Ok(Value::String("WRONG".into()))), 0).expect("fails");
    assert!(message.contains("t.tsv:2"), "{message}");
    assert!(message.contains("got:      \"WRONG\""), "{message}");
    assert!(message.contains("expected: \"A\""), "{message}");
}

#[test]
fn fails_a_value_row_that_failed_rather_than_reporting_a_pass() {
    let message = check(&Runner::new(|_| Err(Failure::message("boom"))), 0).expect("fails");
    assert!(message.contains("t.tsv:2"), "{message}");
    assert!(message.contains("boom"), "{message}");
}

#[test]
fn passes_an_error_row_that_failed_with_the_named_code() {
    assert_eq!(check(&Runner::new(|_| Err(Failure::new("bad_b"))), 1), None);
}

#[test]
fn fails_an_error_row_that_did_not_fail_at_all() {
    let message = check(&Runner::new(|_| Ok(Value::String("fine".into()))), 1).expect("fails");
    assert!(
        message.contains("should fail with ERROR:bad_b"),
        "{message}"
    );
    assert!(message.contains("returned \"fine\""), "{message}");
}

#[test]
fn fails_an_error_row_that_failed_with_a_different_code() {
    // The point of the code being in the fixture: rejecting the input for
    // the wrong reason is not agreement.
    let message = check(&Runner::new(|_| Err(Failure::new("other"))), 1).expect("fails");
    assert!(
        message.contains("code \"other\", expected \"bad_b\""),
        "{message}"
    );
}

#[test]
fn accepts_any_code_for_a_bare_error_row() {
    assert_eq!(check(&Runner::new(|_| Err(Failure::new("any"))), 2), None);
}

#[test]
fn matches_an_error_through_a_match_error_hook() {
    // For a grammar with no stable code to pin: the hook replaces the
    // code comparison, so `ERROR:bad_b` can be matched against the
    // message.
    let runner = Runner::new(|_| Err(Failure::message("something bad_b happened")))
        .match_error(|failure, want, _row| failure.message.contains(want));
    assert_eq!(check(&runner, 1), None);

    let runner = Runner::new(|_| Err(Failure::message("unrelated")))
        .match_error(|failure, want, _row| failure.message.contains(want));
    let message = check(&runner, 1).expect("fails");
    assert!(message.contains("does not match \"bad_b\""), "{message}");
}

#[test]
fn checks_a_pinned_position_independently_of_the_code() {
    let spec = parse_spec(
        "p.tsv",
        "input\texpected\nx\tERROR:unexpected@1:8\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    let row = &spec.rows[0];

    let right = Runner::new(|_| Err(Failure::new("unexpected").at(1, 8)));
    assert!(right.check_row(row, "x", row.col(1)).is_ok());

    let elsewhere = Runner::new(|_| Err(Failure::new("unexpected").at(2, 1)));
    let message = elsewhere
        .check_row(row, "x", row.col(1))
        .expect_err("fails")
        .0;
    assert!(message.contains("failed at 2:1, expected 1:8"), "{message}");

    // An error that names no position is a mismatch, not a pass.
    let nowhere = Runner::new(|_| Err(Failure::new("unexpected")));
    let message = nowhere
        .check_row(row, "x", row.col(1))
        .expect_err("fails")
        .0;
    assert!(
        message.contains("failed at no position, expected 1:8"),
        "{message}"
    );
}

#[test]
fn refuses_a_shared_cell_holding_a_lone_surrogate_escape() {
    let spec = parse_spec(
        "s.tsv",
        "input\texpected\nx\t\"\\ud800\"\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    let row = &spec.rows[0];
    let runner = Runner::new(|_| Ok(Value::String("\u{FFFD}".into())));
    let message = runner
        .check_row(row, "x", row.col(1))
        .expect_err("refused")
        .0;
    assert!(
        message.contains("unpaired surrogate escape at code point 1"),
        "{message}"
    );

    // A parse_expected hook owns its own vocabulary, so the check is not
    // applied on its path.
    let hooked = Runner::new(|_| Ok(Value::String("raw".into())))
        .parse_expected(|_cell, _row| Ok(Value::String("raw".into())));
    assert!(hooked.check_row(row, "x", row.col(1)).is_ok());
}

#[test]
fn reads_wider_vocabularies_through_parse_expected() {
    let spec = parse_spec(
        "v.tsv",
        "input\texpected\nx\tUNDEFINED\ny\t1\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    let runner = Runner::new(|input| {
        Ok(if input == "x" {
            Value::Undefined
        } else {
            Value::Number(1.0)
        })
    })
    .parse_expected(|cell, _row| {
        if cell == "UNDEFINED" {
            Ok(Value::Undefined)
        } else {
            parse_expect(cell)
        }
    });
    assert_eq!(runner.run_spec(&spec).expect("runs"), Vec::<String>::new());
}

#[test]
fn normalizes_both_sides_before_comparing() {
    let spec = parse_spec(
        "n.tsv",
        "input\texpected\nx\t[1]\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    let runner = Runner::new(|_| {
        Ok(Value::Object(vec![(
            "$list".into(),
            Value::Array(vec![Value::Number(1.0)]),
        )]))
    })
    .normalize(|value| match value {
        Value::Object(entries) if entries.len() == 1 && entries[0].0 == "$list" => {
            entries.into_iter().next().expect("one entry").1
        }
        other => other,
    });
    assert_eq!(runner.run_spec(&spec).expect("runs"), Vec::<String>::new());
}

#[test]
fn selects_columns_by_name_and_reads_the_row_in_the_hook() {
    let spec = parse_spec(
        "c.tsv",
        "note\topts\tsrc\twant\nfirst\t{\"upper\":true}\ta\t\"A\"\nsecond\t{}\ta\t\"a\"\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    let runner = Runner::new_with_row(|input, row| {
        let upper = row.named("opts").contains("true");
        Ok(Value::String(if upper {
            input.to_uppercase()
        } else {
            input.to_string()
        }))
    })
    .input("src")
    .expected("want");
    assert_eq!(runner.run_spec(&spec).expect("runs"), Vec::<String>::new());
}

#[test]
fn an_empty_fixture_cannot_be_run() {
    let spec = parse_spec(
        "e.tsv",
        "input\texpected\n# nothing\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    let error = Runner::new(upper).check_spec(&spec).expect_err("no cases");
    assert_eq!(error.0, "e.tsv: no cases");
    assert!(Runner::new(upper).run_spec(&spec).is_err());
}

#[test]
fn a_misnamed_column_fails_the_whole_fixture_once() {
    let error = Runner::new(upper)
        .expected("nosuch")
        .check_spec(&fixture())
        .expect_err("unknown column");
    assert!(error.0.contains("t.tsv: expected column"), "{error}");
}

#[test]
fn run_spec_reports_every_failing_row_at_once() {
    let failures = Runner::new(|_| Ok(Value::Null))
        .run_spec(&fixture())
        .expect("runs");
    assert_eq!(failures.len(), 3);
    assert!(failures[0].contains("t.tsv:2"));
    assert!(failures[2].contains("t.tsv:4"));
}

#[test]
#[should_panic(expected = "3 fixture row(s) failed")]
fn spec_panics_with_the_whole_report() {
    Runner::new(|_| Ok(Value::Null)).spec(&fixture());
}
