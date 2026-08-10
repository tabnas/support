// Copyright (c) 2026 tabnas, MIT License

package tabnassupport

import (
	"path/filepath"
	"testing"
)

// escape_test.go — the escape codec, against the shared fixture.
// ts/test/codec.test.js runs the same rows.

func TestCodecSpec(t *testing.T) {
	spec := mustLoad(t, filepath.Join(specDir(t), "util", "codec.tsv"))
	if 0 == len(spec.Rows) {
		t.Fatal("no cases")
	}

	for _, row := range spec.Rows {
		source := row.Named("source")

		value, err := ParseExpect(row.Named("value"))
		if err != nil {
			t.Fatalf("%s: %v", row.Where(), err)
		}
		escaped, err := ParseExpect(row.Named("escaped"))
		if err != nil {
			t.Fatalf("%s: %v", row.Where(), err)
		}

		if got := Unescape(source); got != value {
			t.Errorf("%s: Unescape(%q) = %q, want %q",
				row.Where(), source, got, value)
		}

		if got := Escape(value.(string)); got != escaped {
			t.Errorf("%s: Escape(%q) = %q, want %q",
				row.Where(), value, got, escaped)
		}

		// Round trip. Encoding then decoding must be the identity — that
		// is what lets a generator write a fixture the loader reads back.
		if got := Unescape(Escape(value.(string))); got != value {
			t.Errorf("%s: round trip = %q, want %q", row.Where(), got, value)
		}
	}
}

func TestCodecNoBackslash(t *testing.T) {
	for _, s := range []string{"", "plain", "a b c", `{"a":1}`, "ünïcödé", "𝒜𝒷"} {
		if got := Unescape(s); got != s {
			t.Errorf("Unescape(%q) = %q", s, got)
		}
		if got := Escape(s); got != s {
			t.Errorf("Escape(%q) = %q", s, got)
		}
	}
}

func TestCodecLeftToRight(t *testing.T) {
	// `\\` consumes both backslashes, leaving `n` an ordinary letter.
	if got := Unescape(`\\n`); got != `\n` {
		t.Errorf(`Unescape("\\n") = %q`, got)
	}
	// Whereas an unescaped pair really is a newline.
	if got := Unescape(`\n`); got != "\n" {
		t.Errorf(`Unescape("\n") = %q`, got)
	}
}

func TestCodecCarriesTabAndNewline(t *testing.T) {
	// The two characters a TSV cell physically cannot hold.
	if got := Unescape(`a\tb`); got != "a\tb" {
		t.Errorf("got %q", got)
	}
	if got := Unescape(`a\nb`); got != "a\nb" {
		t.Errorf("got %q", got)
	}
	if got := Escape("a\tb"); got != `a\tb` {
		t.Errorf("got %q", got)
	}
	if got := Escape("a\nb"); got != `a\nb` {
		t.Errorf("got %q", got)
	}
}

func TestCodecRoundTrip(t *testing.T) {
	samples := []string{
		"", "a", "\n", "\r\n", "\t\t", `\`, `\\`, `\n`, `a\nb`,
		"line1\nline2\r\nline3", `C:\dir\file`, `/^\d+$/`, "\"\t\"",
		"ünïcödé\t𝒜𝒷\n",
	}
	for _, s := range samples {
		if got := Unescape(Escape(s)); got != s {
			t.Errorf("round trip %q = %q", s, got)
		}
	}
}

// TestCodecUTF8Bytes pins the byte-wise decoder against a rune-wise one:
// a multi-byte rune's continuation bytes are all >= 0x80 and can never be
// mistaken for an ASCII escape, so UTF-8 must pass through untouched.
func TestCodecUTF8Bytes(t *testing.T) {
	for _, s := range []string{
		"héllo\\tworld", "日本語\\n終", "𝒜\\\\𝒷", "emoji 🎉\\tdone",
	} {
		got := Unescape(s)
		if want := Unescape(string([]rune(s))); got != want {
			t.Errorf("Unescape(%q) = %q, want %q", s, got, want)
		}
		if !isValidUTF8(got) {
			t.Errorf("Unescape(%q) produced invalid UTF-8: %q", s, got)
		}
	}
}
