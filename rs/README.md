# tabnas-support (Rust)

Shared test-support utilities for the [tabnas](https://github.com/tabnas)
parser system: the TSV spec-fixture loader, the escape codec, the
expectation helpers, the cross-runtime test runner, the divergence
register and the error-code census. Crate `tabnas-support`, library
`tabnas_support`.

This is the Rust half. The canonical TypeScript half is
[`@tabnas/support`](../ts/), the Go half is
[`github.com/tabnas/support/go`](../go/), and the three are written to
behave identically: same escape codec, same comment and blank-line
handling, same `ERROR:<code>` contract, same value comparison. That is
the point. Every tabnas package proves its runtimes agree by running
**one** set of TSV fixtures in **all** of them, and a loader that
disagreed with its twins would make those fixtures prove nothing.

**This crate has no required dependencies, and never will.** Every tabnas
Rust crate takes it as a dev-dependency, so anything it required would
land in all of them, the engine it is used to test included. The JSON it
reads and writes is handled in the crate. The adder grammar that
exercises it end to end therefore lives in the separate
[`adder`](adder/) crate, which may depend on the engine.

## Use

```rust
use std::path::Path;

use tabnas_support::{find_spec_dir, Failure, Runner, Value};

let dir = find_spec_dir(Some(Path::new(env!("CARGO_MANIFEST_DIR")))).unwrap();

let parser = my_grammar::make();
Runner::new(move |input| {
    parser
        .parse(input)
        .map(|value| Value::from(value.to_json()))
        .map_err(|error| Failure::new(error.code.clone()).with_message(error.to_string()))
})
.dir(dir.join("happy"));
```

`find_spec_dir` walks up from where it is told to start until it finds a
`test/spec` directory. `Runner::dir` runs every `.tsv` in the named
directory and panics with every failing row at once, each line carrying
the fixture's own file name and line number.

The parse hook returns a `Value` (the fixture data model: JSON plus
`Undefined`, and numbers that may be infinite) or a `Failure` carrying
the error code the fixture pins and, when the error reports one, its
position. The `serde_json` feature adds `From<serde_json::Value>`, which
is what the engine's `Value::to_json` produces.

## Install

The crate is not on crates.io. Take it as a path dependency on a sibling
checkout, the standard tabnas development model:

```toml
[dev-dependencies]
tabnas-support = { path = "../../support/rs", features = ["serde_json"] }
```

A dev-dependency never reaches a release artifact, which is the same
guarantee `devDependencies` gives the TypeScript half.

## What is in it

- **The escape codec.** `unescape` and `escape`: `\n`, `\r`, `\t` and
  `\\` decoded; every other backslash sequence passed through untouched.
- **The fixture loader.** `parse_spec`, `load_spec`, `load_spec_dir`,
  `find_spec_dir`, `SpecFile`, `Row`, `SpecOptions`, `Column`. Header
  row, blank lines, `#` comment lines, CRLF, BOM, raw columns with
  explicit per-column decoding, physical line numbers, directory
  discovery.
- **The expectation helpers.** `is_error_expect`, `error_code`,
  `error_expect`, `parse_expect`, `equal_value`, `equal_value_with`,
  `format_value`, `lone_surrogate_at`, `lone_surrogate_message`, and the
  `Value` type they work on.
- **The runner.** `Runner`, `Failure`, `report`.
- **The divergence register.** `Register` and `no_divergences`.
- **The census.** `codes_in_spec_dir`, `compare_catalogues`, `coverage`.

## Differences from the TypeScript and Go halves

The behaviour is the same; the shape differs where the language does.

- **Failures are one report, not subtests.** Rust's test harness has no
  subtests, so a fixture's failing rows are collected and reported at
  once, each prefixed `<file>:<line>`. The `run_spec`, `run_file` and
  `run_dir` methods return the failures for a suite that reports its own
  way; `spec`, `file` and `dir` panic with them.
- **The parse hook returns a `Failure`.** This crate cannot name the
  engine's error type, so the suite converts at the boundary. That
  struct already carries the code and the position, so the `errorCode`
  and `errorPos` hooks of the other halves do not exist here.
- **`Undefined` is a `Value` variant.** An empty expected cell reads as
  `Value::Undefined`, distinct from `Value::Null` as in TypeScript. Go
  cannot tell the two apart.
- **`resolve` takes a `Column`.** A position or a name, as an enum, where
  TypeScript takes `number | string`.
- **A lone `\uXXXX` surrogate in a cell decodes to U+FFFD**, as in Go. The
  runner refuses such a cell in a shared expected column before it is
  compared, in every runtime.
- **Errors are values.** Every fallible function returns
  `Result<_, tabnas_support::Error>`, the Rust convention, with the same
  message text the other halves raise.

## Build and test

```bash
cargo test --all-targets
cargo test --doc
cargo clippy --all-targets --all-features -- -D warnings
```

The suite runs the shared `../test/spec/util`, `census` and `register`
fixtures, the same files the TypeScript and Go suites run, plus the
crate's own failure-behaviour tests. `register/divergent.tsv` carries an
`rs` column alongside `ts` and `go`, and this suite reads it, so the
crate is a runtime of that register rather than a reader of someone
else's. The [`adder`](adder/) crate runs `../test/spec/adder` through a
real grammar and needs a sibling checkout of `tabnas/parser`. From the
repository root, `make test-rs` runs both, and `ci/rust/run.sh` is the
full gate.

Hosted CI does not run any of this yet. The Rust workflow is staged at
`../ci/workflows/rust.yml` and has not been promoted, so `ci/rust/run.sh`
locally is what stands in for it.

## License

MIT.
