// Copyright (c) 2026 tabnas, MIT License

// The escape codec shared by every tabnas spec fixture.
//
// A TSV cell cannot hold a raw tab (it would be a column separator) or a
// raw newline (a row separator), so fixtures write those as the
// two-character sequences `\t`, `\n` and `\r`, and a literal backslash as
// `\\`.
//
// Decoding is deliberately minimal: an unrecognised escape passes through
// unchanged (`\q` stays `\q`, `A` stays `A`). That is what lets a fixture
// carry its own backslashes, a regex, a Windows path, a JSON string
// escape, without a second layer of quoting.
//
// `ts/src/escape.ts` and `go/escape.go` implement exactly this, byte for
// byte. The whole point of a shared fixture is that every runtime feeds
// its parser the same source text, so any divergence here is a defect,
// not a preference.

/// Decode the fixture escape set: `\n`, `\r`, `\t` and `\\`. Any other
/// backslash sequence, and a trailing lone backslash, are left as they
/// are.
pub fn unescape(src: &str) -> String {
    // Fast path: the overwhelming majority of fixture cells hold no escape.
    if !src.contains('\\') {
        return src.to_string();
    }

    let mut out = String::with_capacity(src.len());
    let mut chars = src.chars().peekable();
    while let Some(c) = chars.next() {
        if c == '\\' {
            let decoded = match chars.peek() {
                Some('n') => Some('\n'),
                Some('r') => Some('\r'),
                Some('t') => Some('\t'),
                Some('\\') => Some('\\'),
                _ => None,
            };
            if let Some(decoded) = decoded {
                chars.next();
                out.push(decoded);
                continue;
            }
        }
        out.push(c);
    }
    out
}

/// Encode a string into the fixture escape set, the inverse of
/// [`unescape`]. The backslash goes first so an already-escaped sequence
/// is not double-decoded on the way back: `unescape(escape(s)) == s` for
/// every `s`.
pub fn escape(src: &str) -> String {
    let mut out = String::with_capacity(src.len());
    for c in src.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            other => out.push(other),
        }
    }
    out
}
