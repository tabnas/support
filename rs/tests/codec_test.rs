// The escape codec, against the shared fixture. `ts/test/codec.test.js`
// and `go/escape_test.go` run the same rows.

mod common;

use tabnas_support::{escape, load_spec, parse_expect, unescape, SpecOptions, Value};

#[test]
fn spec_util_codec() {
    let spec = load_spec(
        common::spec_dir().join("util").join("codec.tsv"),
        &SpecOptions::default(),
    )
    .expect("codec.tsv loads");
    assert!(!spec.rows.is_empty(), "no cases");

    for row in &spec.rows {
        let source = row.named("source");
        let value = parse_expect(row.named("value")).expect("value is JSON");
        let escaped = parse_expect(row.named("escaped")).expect("escaped is JSON");
        let (Value::String(value), Value::String(escaped)) = (&value, &escaped) else {
            panic!("{}: value and escaped must be JSON strings", row.location());
        };

        assert_eq!(
            &unescape(source),
            value,
            "{}: unescape({source:?})",
            row.location()
        );
        assert_eq!(
            &escape(value),
            escaped,
            "{}: escape({value:?})",
            row.location()
        );
        // Round trip. Encoding then decoding must be the identity: that
        // is what lets a generator write a fixture the loader can read.
        assert_eq!(
            &unescape(&escape(value)),
            value,
            "{}: round trip",
            row.location()
        );
    }
}

#[test]
fn leaves_a_cell_with_no_backslash_exactly_as_it_is() {
    for text in ["", "plain", "a b c", "{\"a\":1}", "ünïcödé", "𝒜𝒷"] {
        assert_eq!(unescape(text), text);
        assert_eq!(escape(text), text);
    }
}

#[test]
fn decodes_left_to_right_so_escapes_cannot_be_chained_by_accident() {
    // `\\` consumes both backslashes, leaving `n` as an ordinary letter.
    assert_eq!(unescape("\\\\n"), "\\n");
    // Whereas an unescaped pair really is a newline.
    assert_eq!(unescape("\\n"), "\n");
}

#[test]
fn carries_a_tab_and_a_newline_through_a_cell() {
    assert_eq!(unescape("a\\tb"), "a\tb");
    assert_eq!(unescape("a\\nb"), "a\nb");
    assert_eq!(escape("a\tb"), "a\\tb");
    assert_eq!(escape("a\nb"), "a\\nb");
}

#[test]
fn a_trailing_lone_backslash_stays() {
    assert_eq!(unescape("back\\"), "back\\");
}
