// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import "strings"

// escape.go — the escape codec shared by every tabnas spec fixture.
//
// A TSV cell cannot hold a raw tab (it would be a column separator) or a
// raw newline (a row separator), so fixtures write those as the
// two-character sequences \t, \n and \r, and a literal backslash as \\.
//
// Decoding is deliberately minimal: an unrecognised escape passes through
// unchanged (\q stays \q, \A stays \A). That is what lets a fixture carry
// its own backslashes — a regex, a Windows path, a JSON string escape —
// without a second layer of quoting.
//
// ts/src/escape.ts implements exactly this, byte for byte. The whole point
// of a shared fixture is that both runtimes feed their parser the same
// source text, so any divergence here is a defect, not a preference.

// Unescape decodes the fixture escape set: \n, \r, \t and \\. Any other
// backslash sequence, and a trailing lone backslash, are left as they are.
func Unescape(src string) string {
	// Fast path: the overwhelming majority of fixture cells hold no escape.
	if !strings.Contains(src, `\`) {
		return src
	}

	var b strings.Builder
	b.Grow(len(src))

	// Byte-wise, not rune-wise: every byte this rewrites is ASCII, and a
	// multi-byte rune's continuation bytes are all >= 0x80, so they can
	// never be mistaken for one. UTF-8 passes through intact.
	for i := 0; i < len(src); i++ {
		c := src[i]
		if '\\' == c && i+1 < len(src) {
			switch src[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(c)
	}

	return b.String()
}

// Escape encodes a string into the fixture escape set, the inverse of
// Unescape. The backslash goes first so an already-escaped sequence is not
// double-decoded on the way back: Unescape(Escape(s)) == s for every s.
func Escape(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(src[i])
		}
	}

	return b.String()
}
