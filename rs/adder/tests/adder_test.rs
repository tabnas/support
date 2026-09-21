// The end-to-end check on the support crate: a real grammar, driven
// entirely by the shared fixtures, through the public API.
//
// `ts/test/adder.test.js` and `go/adder/adder_test.go` run the SAME rows
// against the SAME grammar, so a divergence anywhere in the chain (the
// escape codec, the comment rule, the `ERROR:<code>` contract, the value
// comparison) turns one of the runtimes red.

use std::path::Path;

use tabnas::Value as EngineValue;
use tabnas_support::{find_spec_dir, Failure, Runner, Value};
use tabnas_support_adder::{adder, make};

#[test]
fn spec_adder() {
    let dir = find_spec_dir(Some(Path::new(env!("CARGO_MANIFEST_DIR")))).expect("test/spec");
    let parser = make();
    Runner::new(move |input| {
        parser
            .parse(input)
            .map(|value| Value::from(value.to_json()))
            .map_err(|error| Failure::new(error.code.clone()).with_message(error.to_string()))
    })
    .dir(dir.join("adder"));
}

#[test]
fn parses_the_readme_examples() {
    let parser = make();
    assert_eq!(parser.parse("1+2+3").unwrap(), EngineValue::Number(6.0));
    assert_eq!(parser.parse("10+20").unwrap(), EngineValue::Number(30.0));
    assert_eq!(parser.parse("12+3+45").unwrap(), EngineValue::Number(60.0));
}

#[test]
fn is_a_plugin_so_it_applies_to_any_bare_instance() {
    let mut other = tabnas::Tabnas::new();
    adder(&mut other).unwrap();
    assert_eq!(other.parse("1+2").unwrap(), EngineValue::Number(3.0));
}

#[test]
fn holds_no_state_between_parses() {
    let parser = make();
    assert_eq!(parser.parse("1+2").unwrap(), EngineValue::Number(3.0));
    assert_eq!(parser.parse("1+2").unwrap(), EngineValue::Number(3.0));
    assert_eq!(parser.parse("4").unwrap(), EngineValue::Number(4.0));
}
