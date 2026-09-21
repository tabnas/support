// Copyright (c) 2026 tabnas, MIT License

// Reading the `expected` column, and comparing a parse result against it.
//
// A fixture's expected cell is one of two things: a JSON value the parse
// must produce, or an error the parse must raise, written `ERROR:<code>`.
// The code is part of the contract: "it threw" is not enough, since two
// runtimes that reject the same input for different reasons have not
// actually agreed on anything.
//
// `ts/src/expect.ts` and `go/expect.go` mirror all of this.

use crate::value::Value;
use crate::{Error, Result};

/// The prefix marking an expected-failure cell.
pub const ERROR_PREFIX: &str = "ERROR";

/// Is this expected cell an error expectation?
///
/// Exactly `ERROR`, or `ERROR:` followed by a code. Note the colon: a bare
/// prefix test would read a legitimate `ERRORS` or `ERROR_LIST` expected
/// value as a failure expectation and then never check the parse result
/// at all, a fixture row that silently tests nothing.
pub fn is_error_expect(expected: &str) -> bool {
    expected == ERROR_PREFIX || expected.starts_with(&format!("{ERROR_PREFIX}:"))
}

/// The code from an error expectation: `ERROR:unexpected` gives
/// `unexpected`, and a bare `ERROR` gives `""` (meaning "any code"). A
/// trailing `@<row>:<col>` is a position expectation and is NOT part of
/// the code; see [`error_expect`]. An error when handed a cell that is
/// not an error expectation at all.
pub fn error_code(expected: &str) -> Result<String> {
    error_expect(expected).map(|expectation| expectation.code)
}

/// An error expectation, split into the parts a runner checks separately.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ErrorExpect {
    /// The code, or `""` for "any code" (a bare `ERROR`, or a cell that
    /// pins only a position).
    pub code: String,
    /// 1-based source position the error must report, when the cell pins
    /// one. Both `None` otherwise.
    pub row: Option<usize>,
    pub col: Option<usize>,
}

/// Read an error expectation.
///
/// ```text
/// ERROR                     any error
/// ERROR:unexpected          that code, position unchecked
/// ERROR:unexpected@1:8      that code, reported at row 1 col 8
/// ERROR:@1:8                any code, reported at row 1 col 8
/// ```
///
/// The position channel exists because a code alone does not pin a
/// diagnostic. Two runtimes can agree on `unexpected` and disagree on
/// where they say it happened, which is exactly what the fleet audit
/// found, in several repos at once, with every code row green. A fixture
/// that pins the position makes that disagreement a failing row instead
/// of a difference nobody is looking at.
///
/// `@` rather than another colon because a code is not always a bare
/// identifier: the fleet's fixtures already carry `ERROR:a:b` and whole
/// diagnostic sentences with embedded colons, so `ERROR:x:1:8` could not
/// be split without guessing. Anchoring at the end and requiring digits
/// keeps a message that merely CONTAINS an `@` intact.
///
/// An error when handed a cell that is not an error expectation at all.
pub fn error_expect(expected: &str) -> Result<ErrorExpect> {
    if !is_error_expect(expected) {
        return Err(Error(format!("not an error expectation: {expected:?}")));
    }

    let code = if expected == ERROR_PREFIX {
        ""
    } else {
        &expected[ERROR_PREFIX.len() + 1..]
    };

    let Some((code, row_text, col_text)) = position_suffix(code) else {
        return Ok(ErrorExpect {
            code: code.to_string(),
            row: None,
            col: None,
        });
    };

    // The digits are already constrained; only an absurdly long run can
    // fail here, and that is a malformed fixture, not a mismatch.
    let row = row_text.parse::<usize>().map_err(|error| {
        Error(format!(
            "invalid row in error expectation {expected:?}: {error}"
        ))
    })?;
    let col = col_text.parse::<usize>().map_err(|error| {
        Error(format!(
            "invalid column in error expectation {expected:?}: {error}"
        ))
    })?;

    // Positions are 1-based, so zero is not a position. It matters more
    // than it looks: an error type that leaves row and col at their zero
    // value when it has no position would MATCH `@0:0`, and the row would
    // pass while pinning no source location at all, the exact silent gap
    // this channel exists to close, reintroduced through its own syntax.
    if row < 1 || col < 1 {
        return Err(Error(format!(
            "position in an error expectation is 1-based, so 0 is not a position: {expected:?}"
        )));
    }

    Ok(ErrorExpect {
        code: code.to_string(),
        row: Some(row),
        col: Some(col),
    })
}

/// Split a trailing `@<digits>:<digits>` off a code. Zero is matched here
/// and REJECTED by the caller rather than excluded by the pattern, so
/// `@0:0` fails as a malformed fixture instead of quietly falling through
/// and being read as part of the code.
fn position_suffix(code: &str) -> Option<(&str, &str, &str)> {
    let digits_end = code.len();
    let col_start = code
        .rfind(|c: char| !c.is_ascii_digit())
        .map_or(0, |i| i + 1);
    if col_start == digits_end || !code[..col_start].ends_with(':') {
        return None;
    }
    let colon = col_start - 1;
    let row_start = code[..colon]
        .rfind(|c: char| !c.is_ascii_digit())
        .map_or(0, |i| i + 1);
    if row_start == colon || !code[..row_start].ends_with('@') {
        return None;
    }
    let at = row_start - 1;
    Some((&code[..at], &code[row_start..colon], &code[col_start..]))
}

/// Parse an expected cell as JSON. An empty cell is [`Value::Undefined`],
/// the fixture convention for "no value", as in a utility whose result is
/// nothing at all.
///
/// The cell is NOT escape-decoded first: it is JSON, and JSON has its own
/// escape rules. Decoding it here would turn the two characters `\n`
/// inside a JSON string into a real newline, which is not valid JSON.
///
/// A number too large for an `f64` (`1e400`) reads as an infinity, which
/// is what the canonical `JSON.parse` answers, so a row that loads in
/// TypeScript loads here too.
pub fn parse_expect(expected: &str) -> Result<Value> {
    if expected.is_empty() {
        return Ok(Value::Undefined);
    }
    Value::parse_json(expected)
        .map_err(|reason| Error(format!("invalid expected JSON: {expected:?}: {reason}")))
}

/// The position of the first UNPAIRED `\uXXXX` surrogate escape in an
/// expected cell, counted in CODE POINTS, or `None`.
///
/// Code points because this number crosses the runtimes. The natural
/// index is a UTF-16 offset in TypeScript, a byte offset in Go and here,
/// and those disagree the moment anything non-ASCII precedes the escape:
/// for `"é\ud800"` they are 2 and 3. A helper whose whole purpose is to
/// keep the ports saying the same thing cannot report a number that
/// depends on which port asked.
///
/// Why this is not a curiosity: `JSON.parse` preserves a lone surrogate
/// (a JavaScript string is UTF-16 and may hold one) while Go's
/// `encoding/json` and this crate's reader replace it with U+FFFD (a
/// UTF-8 string cannot). A SHARED expected cell holding one asks the
/// runtimes different questions and reports agreement either way, so the
/// runner refuses it. A per-runtime register column is a different
/// matter: there each runtime reads its own cell.
///
/// Only the ESCAPE form is detected, because it is the only one that can
/// occur: a fixture file is UTF-8, and a lone surrogate has no UTF-8
/// encoding.
pub fn lone_surrogate_at(cell: &str) -> Option<usize> {
    let bytes = cell.as_bytes();
    let hex4 = |at: usize| -> Option<u32> {
        let text = bytes.get(at..at + 4)?;
        let text = std::str::from_utf8(text).ok()?;
        if !text.chars().all(|c| c.is_ascii_hexdigit()) {
            return None;
        }
        u32::from_str_radix(text, 16).ok()
    };
    let code_points_before = |at: usize| cell[..at].chars().count();

    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] != b'\\' {
            i += 1;
            continue;
        }

        // A run of backslashes escapes itself in pairs; only an ODD run
        // leaves a live escape, whose introducer is the byte after the
        // whole run. `\\ud800` is a literal backslash then `ud800`.
        let mut j = i;
        while j < bytes.len() && bytes[j] == b'\\' {
            j += 1;
        }
        if (j - i) % 2 == 0 {
            i = j;
            continue;
        }

        let start = j - 1;
        if j >= bytes.len() || bytes[j] != b'u' {
            i = j + 1;
            continue;
        }

        let Some(unit) = hex4(j + 1) else {
            i = j + 1;
            continue;
        };

        if (0xD800..=0xDBFF).contains(&unit) {
            // A high surrogate is fine if a low follows IMMEDIATELY.
            let k = j + 5;
            if bytes.get(k) == Some(&b'\\') && bytes.get(k + 1) == Some(&b'u') {
                if let Some(low) = hex4(k + 2) {
                    if (0xDC00..=0xDFFF).contains(&low) {
                        i = k + 6;
                        continue;
                    }
                }
            }
            return Some(code_points_before(start));
        }
        if (0xDC00..=0xDFFF).contains(&unit) {
            // A paired low was consumed above, so reaching one here means
            // it has no high before it. The prefix always ends on a
            // backslash, so counting over it never splits a character.
            return Some(code_points_before(start));
        }

        i = j + 5;
    }

    None
}

/// The message the runner uses when a shared cell holds an unpaired
/// surrogate escape. Exported so every runtime says the same thing, and
/// so a caller building its own runner can reuse it.
pub fn lone_surrogate_message(cell: &str, at: usize) -> String {
    format!(
        "expected cell holds an unpaired surrogate escape at code point {at}: {cell:?}\n  \
         A shared expected column CANNOT express this: JSON.parse preserves a lone surrogate\n  \
         (a JavaScript string is UTF-16) and Go's encoding/json replaces it with U+FFFD\n  \
         (a Go string is UTF-8). The two runtimes would be asked different questions and both\n  \
         would pass. This is a recorded, permanent divergence - see DIVERGENCE.md.\n  \
         Put the case in a per-runtime register column, where each decoding is written out,\n  \
         or in each port's own suite with opposite assertions. A surrogate PAIR is fine here."
    )
}

/// Compare a parse result against an expected value with JSON semantics:
/// structural, key-order independent, `NaN` equal to itself, and `-0`
/// NOT equal to `0`. Those last two are the two halves of ADR-15, and
/// they go opposite ways on purpose. See [`Value`]'s equality.
pub fn equal_value(got: &Value, expected: &Value) -> bool {
    got == expected
}

/// [`equal_value`] with a normalize hook, applied to every node on both
/// sides, outermost first. This is where a runtime-specific container,
/// an insertion-ordered map or a reference wrapper unwrapped into a
/// plain value, is folded into the plain value the fixture's JSON
/// describes.
pub fn equal_value_with(got: &Value, expected: &Value, normalize: &dyn Fn(Value) -> Value) -> bool {
    let got = normalize(got.clone());
    let expected = normalize(expected.clone());
    match (&got, &expected) {
        (Value::Array(a), Value::Array(b)) => {
            a.len() == b.len()
                && a.iter()
                    .zip(b)
                    .all(|(x, y)| equal_value_with(x, y, normalize))
        }
        (Value::Object(a), Value::Object(b)) => {
            a.len() == b.len()
                && a.iter().all(|(key, value)| {
                    b.iter()
                        .find(|(other_key, _)| other_key == key)
                        .is_some_and(|(_, other)| equal_value_with(value, other, normalize))
                })
        }
        _ => got == expected,
    }
}

/// Render a value for a failure message: JSON where possible, so the text
/// lines up with how the fixture wrote it, with `-0` spelt `-0` and the
/// values JSON cannot spell written as JavaScript names them.
pub fn format_value(value: &Value) -> String {
    value.to_string()
}
