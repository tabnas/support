// The register's own failure behaviour.
//
// What needs its own test is not that a regression fails; the runner
// underneath already does that. It is the property a plain fixture does
// NOT have: that a FIXED divergence fails too, and says so in words that
// send the reader to delete the row rather than to restore the old
// behaviour. `ts/test/register.test.js` and `go/register_test.go` cover
// the same cases, and `spec_register_divergent` runs the shared file.

mod common;

use std::cell::Cell;
use std::rc::Rc;

use tabnas_support::{load_spec, parse_spec, Failure, Register, Runner, SpecOptions, Value};

fn fixture() -> tabnas_support::SpecFile {
    parse_spec(
        "d.tsv",
        "input\tts\tgo\na\t\"A\"\t\"a\"\nb\tERROR:bad_b\t\"b\"\n",
        &SpecOptions::default(),
    )
    .expect("parses")
}

// The TS column's recorded behaviour: uppercase, and b fails.
fn ts_port(input: &str) -> Result<Value, Failure> {
    if input == "b" {
        Err(Failure::new("bad_b"))
    } else {
        Ok(Value::String(input.to_uppercase()))
    }
}

fn ts_register(parse: impl Fn(&str) -> Result<Value, Failure> + 'static) -> Register {
    Register::new(Runner::new(parse), "ts", &["ts", "go"])
}

fn check(register: &Register, index: usize) -> Option<String> {
    let spec = fixture();
    let row = &spec.rows[index];
    register.check_row(row, &row.unesc(0)).err().map(|e| e.0)
}

#[test]
fn passes_while_the_divergence_holds() {
    let register = ts_register(ts_port);
    assert_eq!(check(&register, 0), None);
    assert_eq!(check(&register, 1), None);
    assert_eq!(
        register.run_spec(&fixture()).expect("runs"),
        Vec::<String>::new()
    );
}

#[test]
fn reports_a_closed_divergence_and_names_the_row_to_delete() {
    // The TS port now answers what the go column records.
    let register = ts_register(|input| Ok(Value::String(input.to_string())));
    let message = check(&register, 0).expect("fails");
    assert!(message.contains("d.tsv:2"), "{message}");
    assert!(message.contains("CLOSED"), "{message}");
    assert!(message.contains("DELETE this row"), "{message}");
    assert!(
        message.contains("go column(s) record (\"\\\"a\\\"\")"),
        "{message}"
    );
}

#[test]
fn reports_an_ordinary_regression_as_a_mismatch() {
    let register = ts_register(|_| Ok(Value::String("zzz".into())));
    let message = check(&register, 0).expect("fails");
    assert!(!message.contains("CLOSED"), "{message}");
    assert!(message.contains("got:      \"zzz\""), "{message}");
    assert!(message.contains("expected: \"A\""), "{message}");
}

#[test]
fn a_closed_error_divergence_is_reported_too() {
    // Row 1 records that ts fails on b and go accepts it; ts now accepts.
    let register = ts_register(|input| Ok(Value::String(input.to_string())));
    let message = check(&register, 1).expect("fails");
    assert!(message.contains("CLOSED"), "{message}");
}

#[test]
fn rejects_a_row_that_records_no_divergence() {
    let spec = parse_spec(
        "same.tsv",
        "input\tts\tgo\na\t1\t1.0\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    let register = ts_register(|_| Ok(Value::Number(1.0)));
    let message = register
        .check_row(&spec.rows[0], "a")
        .expect_err("no divergence")
        .0;
    assert!(message.contains("records no divergence"), "{message}");
}

#[test]
fn parses_each_row_exactly_once_whatever_it_is_compared_against() {
    let calls = Rc::new(Cell::new(0));
    let counter = Rc::clone(&calls);
    // Answers the go cell, so the register has to compare twice.
    let register = ts_register(move |input| {
        counter.set(counter.get() + 1);
        Ok(Value::String(input.to_string()))
    });
    let message = check(&register, 0).expect("fails");
    assert!(message.contains("CLOSED"), "{message}");
    assert_eq!(calls.get(), 1);
}

#[test]
fn a_partially_closed_divergence_says_which_column_to_update() {
    let spec = parse_spec(
        "three.tsv",
        "input\tts\tgo\trust\na\t\"A\"\t\"a\"\t\"aa\"\n",
        &SpecOptions::default(),
    )
    .expect("parses");
    // The rust port now answers what go records, and still not what ts does.
    let register = Register::new(
        Runner::new(|input| Ok(Value::String(input.to_string()))),
        "rust",
        &["ts", "go", "rust"],
    );
    let message = register.check_row(&spec.rows[0], "a").expect_err("fails").0;
    assert!(message.contains("PARTIALLY closed"), "{message}");
    assert!(
        message.contains("agrees with go, but not with ts"),
        "{message}"
    );
    assert!(message.contains("UPDATE the rust column"), "{message}");
}

#[test]
fn a_misconfigured_register_fails_before_any_row() {
    let error = Register::new(Runner::new(ts_port), "ts", &["ts"])
        .run_spec(&fixture())
        .expect_err("one runtime");
    assert!(error.0.contains("at least two runtimes"), "{error}");

    let error = Register::new(Runner::new(ts_port), "rs", &["ts", "go"])
        .run_spec(&fixture())
        .expect_err("not among runtimes");
    assert!(error.0.contains("must include \"rs\""), "{error}");

    let error = Register::new(Runner::new(ts_port), "ts", &["ts", "py"])
        .run_spec(&fixture())
        .expect_err("missing column");
    assert!(error.0.contains("no column named \"py\""), "{error}");
}

#[test]
fn spec_register_divergent() {
    // The shared stand-in register, read as the ts column: the fake port
    // below answers each row the way that column records.
    let spec = load_spec(
        common::spec_dir().join("register").join("divergent.tsv"),
        &SpecOptions::default(),
    )
    .expect("loads");
    let ts = ts_register(|input| match input {
        "b" => Err(Failure::new("bad_b")),
        "c" => Ok(Value::String("c".into())),
        other => Ok(Value::String(other.to_uppercase())),
    });
    assert_eq!(ts.run_spec(&spec).expect("runs"), Vec::<String>::new());

    // And as the go column, with a port that answers that column.
    let go = Register::new(
        Runner::new(|input| match input {
            "c" => Err(Failure::new("bad_c")),
            other => Ok(Value::String(other.to_string())),
        }),
        "go",
        &["ts", "go"],
    );
    assert_eq!(go.run_spec(&spec).expect("runs"), Vec::<String>::new());
}
