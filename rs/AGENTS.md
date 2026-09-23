# Agents Guide, rs/

The Rust half of `@tabnas/support`. Read [`../AGENTS.md`](../AGENTS.md)
first: it holds the cross-runtime rules, and this file only covers what
is specific to these two crates.

## Layout

| Path | |
|---|---|
| `Cargo.toml`, `src/` | the `tabnas-support` crate (library `tabnas_support`): `escape.rs`, `spec.rs`, `expect.rs`, `runner.rs`, `register.rs`, `census.rs`, and `value.rs` (the fixture data model with its own JSON reader and writer) |
| `tests/` | the shared `../test/spec/{util,census,register}` fixtures, the runner's and register's failure behaviour, the census tripwire, the version sites |
| | `register/divergent.tsv` has an `rs` column beside `ts` and `go`, and `register_test.rs` reads it, so this crate is a runtime OF that register. It also runs the file as the other two columns, which is how one suite covers a mechanism three suites share. The runtimes list is held to the file's header, because an unnamed column is not a runtime and dropping one would pass. |
| `adder/` | a SEPARATE crate, `tabnas-support-adder`, holding the adder grammar; needs the engine as a sibling checkout (`../../../parser/rs`) and runs `../test/spec/adder` through the runner |
| `README.md` | the crate front page, prose-gated |

## Rule 2 in Rust: no dependencies

The support crate takes **no required dependencies**. `value.rs` carries
a small strict JSON reader and writer rather than depending on a JSON
crate, because every tabnas Rust crate takes this one as a
dev-dependency. The optional `serde_json` feature adds conversions to and
from `serde_json::Value` (which the engine's `Value::to_json` produces)
and nothing else; it is off by default. Do not add a required dependency.

The adder crate is separate for the same reason `go/adder` is a separate
module: it needs the engine. `cargo test` in `rs/` does not reach it; the
Makefile runs both.

## The value model

`Value` is JSON plus `Undefined` and numbers that may be infinite or
NaN. Equality is the ADR-15 contract: key-order independent, `-0 != 0`,
`NaN == NaN`. `parse_expect("")` is `Undefined`; `parse_expect("1e400")`
is `Infinity`, as `JSON.parse` reads it, so a fixture that loads in
TypeScript loads here. A lone surrogate escape decodes to U+FFFD, the Go
reading; the runner refuses such a shared cell before comparing it.

## What differs, and why

Every difference is listed in `README.md` and `../doc/reference.md`
(the Rust section). The two that shape a consuming suite: the runner
returns a `Failure` struct from the parse hook rather than reading a
thrown error by shape, and failures are one report per fixture rather
than subtests. Add a seventh difference only with the sentence that says
why, in both places.

## Running it

```bash
cd rs && cargo test --all-targets && cargo test --doc
cargo clippy --all-targets --all-features -- -D warnings
cd adder && cargo test --all-targets
```

`make test-rs` from the repository root runs both crates; `ci/rust/run.sh`
is the full gate (formatting, both lockfiles, clippy) and needs
`tabnas/parser` checked out beside this repository.

Run that script before calling Rust work done. The `ci.yml` matrix
covers `ts/`, `go/` and `go/adder/` and stops; `.github/workflows/rust.yml`
runs this script, but only when a change touches `rs/**`, `test/spec/**`,
`ts/package.json`, `ci/rust/**` or the workflow itself, and always
against the engine's `main` rather than a release.
