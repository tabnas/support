// Copyright (c) 2026 tabnas, MIT License

//! The adder grammar, as a plugin.
//!
//! This is the integer-addition grammar from the `tabnas` engine README
//! (`1+2+3` => 6), packaged so every runtime can run it against the
//! shared `test/spec/adder/*.tsv` fixtures. It is the smallest grammar
//! that is still a real one: two rules, one custom token, a push and a
//! repeat, which makes it the natural end-to-end check that the support
//! crate's loader, escape codec, expectation parsing and runner do the
//! same thing in TypeScript, Go and Rust.
//!
//! `ts/src/adder.ts` and `go/adder/adder.go` are the same grammar,
//! declared the same way.
//!
//! The grammar reads:
//!
//! ```text
//! val = add            -- `val` holds the running total
//! add = NR [ PL add ]  -- each number adds to it; `+` repeats
//! ```
//!
//! `val` opens by pushing `add` with the total at 0. `add` opens on a
//! number and adds it to its parent's node, then closes on `+` by
//! REPLACING itself with another `add`: a repeat at the same stack depth,
//! so `1+2+...` of any length runs in one frame and every iteration's
//! parent is still `val`, where the total lands.
//!
//! ```
//! let parser = tabnas_support_adder::make();
//! assert_eq!(parser.parse("1+2+3").unwrap(), tabnas::Value::Number(6.0));
//! ```

use std::cell::RefCell;
use std::rc::Rc;

use tabnas::{GrammarError, GrammarSpec, Tabnas, Value};

/// The grammar document. Actions are named and referenced as `@` strings,
/// which is what keeps the spec itself a plain data structure, the same
/// shape as its TypeScript and Go twins.
const GRAMMAR: &str = r##"{
  "options": {
    "fixed": { "token": { "#PL": "+" } },
    "rule": { "start": "val" }
  },
  "ruleOrder": ["val", "add"],
  "rule": {
    "val": {
      "open": [ { "p": "add", "a": "@init" } ],
      "close": [ {} ]
    },
    "add": {
      "open": [ { "s": "#NR", "a": "@add" } ],
      "close": [ { "s": "#PL", "r": "add" }, {} ]
    }
  }
}"##;

/// Apply the adder grammar to an instance.
///
/// ```
/// let mut parser = tabnas::Tabnas::new();
/// tabnas_support_adder::adder(&mut parser).unwrap();
/// assert_eq!(parser.parse("10+20").unwrap(), tabnas::Value::Number(30.0));
/// ```
pub fn adder(parser: &mut Tabnas) -> Result<(), GrammarError> {
    // Start the running total at 0. A fresh cell, not a write through the
    // shared one: a pushed rule shares its parent's node cell, and the
    // total has to be the `val` rule's own value.
    parser.action("@init", |rule| {
        rule.node = Rc::new(RefCell::new(Value::Number(0.0)));
    });

    // Add this number to the total. The parent is the `val` rule, and the
    // first open token is the number matched at `#NR`.
    parser.action("@add", |rule| {
        let addend = rule
            .o0()
            .and_then(|token| match &token.val {
                Value::Number(n) => Some(*n),
                _ => None,
            })
            .unwrap_or(0.0);
        if let Some(parent) = &rule.parent_node {
            let mut total = parent.borrow_mut();
            let current = match &*total {
                Value::Number(n) => *n,
                _ => 0.0,
            };
            *total = Value::Number(current + addend);
        }
    });

    let spec = GrammarSpec::from_json(GRAMMAR)?;
    parser.grammar(&spec)?;
    Ok(())
}

/// A new instance with the adder grammar installed.
pub fn make() -> Tabnas {
    let mut parser = Tabnas::new();
    adder(&mut parser).expect("the adder grammar document is fixed and valid");
    parser
}
