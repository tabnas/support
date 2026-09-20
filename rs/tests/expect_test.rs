// Reading the expected column, and comparing against it.
// `ts/test/expect.test.js` and `go/expect_test.go` run the same rows.

mod common;

use tabnas_support::{
    equal_value, equal_value_with, error_code, error_expect, format_value, is_error_expect,
    load_spec, lone_surrogate_at, parse_expect, SpecOptions, Value,
};

fn util(name: &str) -> tabnas_support::SpecFile {
    load_spec(
        common::spec_dir().join("util").join(name),
        &SpecOptions::default(),
    )
    .expect("fixture loads")
}

#[test]
fn spec_util_expect_error() {
    let spec = util("expect-error.tsv");
    assert!(!spec.rows.is_empty(), "no cases");

    for row in &spec.rows {
        let cell = row.named("expected");
        let at = row.location();
        let want_is_error = parse_expect(row.named("iserror"))
            .expect("iserror is JSON")
            .as_bool()
            .expect("iserror is a boolean");

        assert_eq!(
            is_error_expect(cell),
            want_is_error,
            "{at}: is_error_expect({cell:?})"
        );

        // A cell that IS an error expectation but a malformed one:
        // reading it must fail rather than yield a position nobody meant.
        if parse_expect(row.named("bad"))
            .expect("bad is JSON")
            .as_bool()
            == Some(true)
        {
            let error = error_expect(cell).expect_err("malformed position");
            assert!(error.0.contains("is 1-based"), "{at}: {error}");
            let error = error_code(cell).expect_err("malformed position");
            assert!(error.0.contains("is 1-based"), "{at}: {error}");
            continue;
        }

        if want_is_error {
            let code = parse_expect(row.named("code")).expect("code is JSON");
            assert_eq!(
                Value::String(error_code(cell).expect("reads")),
                code,
                "{at}: error_code({cell:?})"
            );

            // The position channel. `row`/`col` are empty for a cell that
            // pins no position, and parse_expect reads an empty cell as
            // Undefined, so the same assertion covers both kinds of row.
            let expectation = error_expect(cell).expect("reads");
            let want_row = parse_expect(row.named("row")).expect("row is JSON");
            let want_col = parse_expect(row.named("col")).expect("col is JSON");
            let as_value = |position: Option<usize>| {
                position.map_or(Value::Undefined, |n| Value::Number(n as f64))
            };
            assert_eq!(as_value(expectation.row), want_row, "{at}: row of {cell:?}");
            assert_eq!(as_value(expectation.col), want_col, "{at}: col of {cell:?}");
        } else {
            let error = error_code(cell).expect_err("not an error expectation");
            assert!(
                error.0.contains("not an error expectation"),
                "{at}: {error}"
            );
            let error = error_expect(cell).expect_err("not an error expectation");
            assert!(
                error.0.contains("not an error expectation"),
                "{at}: {error}"
            );
        }
    }
}

#[test]
fn spec_util_value_equal() {
    let spec = util("value-equal.tsv");
    assert!(!spec.rows.is_empty(), "no cases");

    for row in &spec.rows {
        let at = row.location();
        let a = parse_expect(row.named("a")).unwrap_or_else(|e| panic!("{at}: {e}"));
        let b = parse_expect(row.named("b")).unwrap_or_else(|e| panic!("{at}: {e}"));
        let want = parse_expect(row.named("equal"))
            .expect("equal is JSON")
            .as_bool()
            .expect("equal is a boolean");

        assert_eq!(
            equal_value(&a, &b),
            want,
            "{at}: equal_value({}, {})",
            row.named("a"),
            row.named("b")
        );
        // Symmetric, whichever side the fixture put the value on.
        assert_eq!(equal_value(&b, &a), want, "{at}: reversed");
        // The normalizing form with an identity hook agrees.
        assert_eq!(
            equal_value_with(&a, &b, &|v| v),
            want,
            "{at}: with identity"
        );
    }
}

#[test]
fn spec_util_lone_surrogate() {
    let spec = util("lone-surrogate.tsv");
    assert!(!spec.rows.is_empty(), "no cases");

    for row in &spec.rows {
        let cell = row.named("cell");
        let want = parse_expect(row.named("at"))
            .expect("at is JSON")
            .as_f64()
            .expect("at is a number");
        let got = lone_surrogate_at(cell).map_or(-1.0, |index| index as f64);
        assert_eq!(got, want, "{}: lone_surrogate_at({cell:?})", row.location());
    }
}

#[test]
fn reads_an_empty_cell_as_no_value() {
    assert_eq!(parse_expect("").expect("reads"), Value::Undefined);
    assert_ne!(Value::Undefined, Value::Null);
}

#[test]
fn reads_a_cell_as_json_not_as_escaped_text() {
    // The two characters `\n` inside a JSON string are a newline by
    // JSON's own rules. Escape-decoding the cell first would produce a
    // real newline inside the quotes, which is not valid JSON at all.
    assert_eq!(
        parse_expect("\"a\\nb\"").expect("reads"),
        Value::String("a\nb".to_string())
    );
    let value = parse_expect("{\"a\":[1,2]}").expect("reads");
    assert_eq!(
        value.get("a").and_then(Value::as_array).map(<[Value]>::len),
        Some(2)
    );
}

#[test]
fn rejects_a_cell_that_is_not_json_with_the_cell_quoted() {
    let error = parse_expect("{a:1}").expect_err("not JSON");
    assert!(
        error.0.starts_with("invalid expected JSON: \"{a:1}\""),
        "{error}"
    );
    let error = parse_expect("1 2").expect_err("trailing content");
    assert!(error.0.contains("trailing"), "{error}");
}

#[test]
fn reads_an_out_of_range_number_as_infinity_like_json_parse() {
    assert_eq!(
        parse_expect("1e400").expect("reads"),
        Value::Number(f64::INFINITY)
    );
    assert_eq!(
        parse_expect("-1e400").expect("reads"),
        Value::Number(f64::NEG_INFINITY)
    );
}

#[test]
fn decodes_json_string_escapes_including_surrogate_pairs() {
    assert_eq!(
        parse_expect("\"\\ud83d\\ude00 \\u00e9 \\/\\b\\f\\t\"").expect("reads"),
        Value::String("😀 é /\u{8}\u{c}\t".to_string())
    );
    // A lone surrogate is what a UTF-8 string cannot hold; it reads as
    // U+FFFD, which is what Go's encoding/json does too.
    assert_eq!(
        parse_expect("\"\\ud800\"").expect("reads"),
        Value::String("\u{FFFD}".to_string())
    );
}

#[test]
fn a_repeated_key_keeps_the_first_position_and_the_last_value() {
    let value = parse_expect("{\"a\":1,\"b\":2,\"a\":3}").expect("reads");
    assert_eq!(format_value(&value), "{\"a\":3,\"b\":2}");
}

#[test]
fn formats_signed_zero_and_the_values_json_cannot_spell() {
    assert_eq!(format_value(&Value::Number(-0.0)), "-0");
    assert_eq!(format_value(&Value::Number(0.0)), "0");
    assert_eq!(format_value(&Value::Number(1.0)), "1");
    assert_eq!(format_value(&Value::Number(2.5)), "2.5");
    // Integral values above 2^63 must not go through an i64 cast.
    assert_eq!(format_value(&Value::Number(1e20)), "100000000000000000000");
    assert_eq!(
        format_value(&Value::Number(-1e20)),
        "-100000000000000000000"
    );
    assert_eq!(format_value(&Value::Number(9.3e18)), "9300000000000000000");
    // The exponent form is JavaScript's, sign included.
    assert_eq!(format_value(&Value::Number(1e21)), "1e+21");
    assert_eq!(format_value(&Value::Number(-1.5e25)), "-1.5e+25");
    assert_eq!(format_value(&Value::Number(1.5e-7)), "1.5e-7");
    assert_eq!(format_value(&Value::Number(0.000001)), "0.000001");
    assert_eq!(format_value(&Value::Number(f64::INFINITY)), "Infinity");
    assert_eq!(format_value(&Value::Number(f64::NAN)), "NaN");
    assert_eq!(format_value(&Value::Undefined), "undefined");
    let nested = parse_expect("[{\"a\":\"q\\\"\\n\"},null,true]").expect("reads");
    assert_eq!(format_value(&nested), "[{\"a\":\"q\\\"\\n\"},null,true]");
}

#[test]
fn nan_is_equal_to_itself_and_key_order_does_not_matter() {
    assert!(equal_value(
        &Value::Number(f64::NAN),
        &Value::Number(f64::NAN)
    ));
    let a = parse_expect("{\"a\":1,\"b\":2}").expect("reads");
    let b = parse_expect("{\"b\":2,\"a\":1}").expect("reads");
    assert!(equal_value(&a, &b));
}

#[test]
fn normalize_is_applied_to_every_node_outermost_first() {
    // A wrapper the parser produces, folded away before comparison.
    let unwrap = |value: Value| match value {
        Value::Object(ref entries) if entries.len() == 1 && entries[0].0 == "$wrapped" => {
            entries[0].1.clone()
        }
        other => other,
    };
    let got = parse_expect("{\"$wrapped\":[{\"$wrapped\":1},2]}").expect("reads");
    let want = parse_expect("[1,2]").expect("reads");
    assert!(equal_value_with(&got, &want, &unwrap));
    assert!(!equal_value(&got, &want));
}
