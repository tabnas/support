/* Copyright (c) 2026 tabnas, MIT License */

/* escape.ts
 * The escape codec shared by every tabnas spec fixture.
 *
 * A TSV cell cannot hold a raw tab (it would be a column separator) or a
 * raw newline (a row separator), so fixtures write those as the
 * two-character sequences `\t`, `\n` and `\r`, and a literal backslash as
 * `\\`.
 *
 * Decoding is deliberately minimal: an unrecognised escape passes through
 * unchanged (`\q` stays `\q`, `A` stays `A`). That is what lets a
 * fixture carry its own backslashes — a regex, a Windows path, a JSON
 * string escape — without a second layer of quoting.
 *
 * `go/escape.go` implements exactly this, byte for byte. The whole point of
 * a shared fixture is that both runtimes feed their parser the same source
 * text, so any divergence here is a defect, not a preference.
 */

// Decode the fixture escape set: \n, \r, \t and \\. Any other backslash
// sequence, and a trailing lone backslash, are left as they are.
export function unescape(src: string): string {
  // Fast path: the overwhelming majority of fixture cells hold no escape.
  if (!src.includes('\\')) {
    return src
  }

  let out = ''
  for (let i = 0; i < src.length; i++) {
    const c = src[i]
    if ('\\' === c && i + 1 < src.length) {
      const n = src[i + 1]
      if ('n' === n) { out += '\n'; i++; continue }
      if ('r' === n) { out += '\r'; i++; continue }
      if ('t' === n) { out += '\t'; i++; continue }
      if ('\\' === n) { out += '\\'; i++; continue }
    }
    out += c
  }
  return out
}


// Encode a string into the fixture escape set, the inverse of `unescape`.
// The backslash goes first so an already-escaped sequence is not
// double-decoded on the way back: `unescape(escape(s)) === s` for every s.
export function escape(src: string): string {
  let out = ''
  for (const c of src) {
    if ('\\' === c) { out += '\\\\'; continue }
    if ('\n' === c) { out += '\\n'; continue }
    if ('\r' === c) { out += '\\r'; continue }
    if ('\t' === c) { out += '\\t'; continue }
    out += c
  }
  return out
}
