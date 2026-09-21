// Copyright (c) 2026 tabnas, MIT License

// The value a fixture's expected column describes, and the value a parser
// under test hands the runner.
//
// It is the JSON data model plus the two things a parity harness has to
// be able to say that JSON cannot: `Undefined` (an empty expected cell,
// or a parse that yielded no value at all, which TypeScript tells apart
// from `null` and Go cannot) and a number that is infinite or not a
// number (`1e400` in a fixture reads as Infinity, as `JSON.parse` reads
// it; a JSON5 grammar produces `NaN`).
//
// The reader and writer are written out here rather than borrowed from a
// JSON crate, because this crate is a dev-dependency of every tabnas
// Rust crate and takes no dependencies of its own. They handle exactly
// RFC 8259 JSON, with two deliberate readings recorded in
// `doc/reference.md`: an out-of-range number widens to an infinity
// instead of failing to load, and an unpaired `\uXXXX` surrogate decodes
// to U+FFFD, which is what a UTF-8 string can hold (the runner refuses
// such a cell before it is ever compared, so this is a courtesy for a
// suite reading one deliberately).

use std::fmt;

/// A parsed value in the fixture data model. See the module notes.
#[derive(Debug, Clone)]
pub enum Value {
    /// No value at all: an empty expected cell, or a parse that produced
    /// nothing. Distinct from [`Value::Null`], as in TypeScript.
    Undefined,
    Null,
    Bool(bool),
    /// Every number is an `f64`, as in the canonical runtime. An integer
    /// beyond 2^53 is therefore inexact here exactly as it is in
    /// `JSON.parse`, and a fixture must not pin one.
    Number(f64),
    String(String),
    Array(Vec<Value>),
    /// Entries in document order. Order never takes part in equality
    /// (ADR-15 keeps key order out of the value contract); it is kept so
    /// a failure message renders the value the way the fixture wrote it.
    Object(Vec<(String, Value)>),
}

impl Value {
    /// Read a JSON text. The error is a short reason without the text
    /// itself, so [`crate::parse_expect`] can quote the cell once.
    pub fn parse_json(text: &str) -> Result<Value, String> {
        let mut reader = Reader {
            bytes: text.as_bytes(),
            at: 0,
        };
        reader.skip_space();
        let value = reader.value()?;
        reader.skip_space();
        if reader.at < reader.bytes.len() {
            return Err(format!("unexpected trailing content at byte {}", reader.at));
        }
        Ok(value)
    }

    /// The entry at `key` of an object, or `None` for anything else.
    pub fn get(&self, key: &str) -> Option<&Value> {
        match self {
            Value::Object(entries) => entries
                .iter()
                .find(|(name, _)| name == key)
                .map(|(_, value)| value),
            _ => None,
        }
    }

    /// Insert or replace an object entry, keeping the position of an
    /// existing key, which is what a JavaScript object does.
    pub fn insert(&mut self, key: impl Into<String>, value: Value) {
        let key = key.into();
        if let Value::Object(entries) = self {
            if let Some(entry) = entries.iter_mut().find(|(name, _)| *name == key) {
                entry.1 = value;
            } else {
                entries.push((key, value));
            }
        }
    }

    /// Whether this is [`Value::Undefined`].
    pub fn is_undefined(&self) -> bool {
        matches!(self, Value::Undefined)
    }

    /// Whether this is [`Value::Null`].
    pub fn is_null(&self) -> bool {
        matches!(self, Value::Null)
    }

    /// The number, when this is one.
    pub fn as_f64(&self) -> Option<f64> {
        match self {
            Value::Number(n) => Some(*n),
            _ => None,
        }
    }

    /// The string, when this is one.
    pub fn as_str(&self) -> Option<&str> {
        match self {
            Value::String(s) => Some(s),
            _ => None,
        }
    }

    /// The boolean, when this is one.
    pub fn as_bool(&self) -> Option<bool> {
        match self {
            Value::Bool(b) => Some(*b),
            _ => None,
        }
    }

    /// The elements, when this is an array.
    pub fn as_array(&self) -> Option<&[Value]> {
        match self {
            Value::Array(items) => Some(items),
            _ => None,
        }
    }

    /// The entries, when this is an object.
    pub fn as_object(&self) -> Option<&[(String, Value)]> {
        match self {
            Value::Object(entries) => Some(entries),
            _ => None,
        }
    }
}

// Equality with JSON semantics: structural, key-order independent, an
// integer equal to the float of the same magnitude (there is only one
// number type here, so that is automatic), NaN equal to itself, and -0
// NOT equal to 0. The last two are the two halves of ADR-15 and go
// opposite ways on purpose: a parser that reports 0 for the input -0 has
// lost information the source carried, while NaN is what a fixture cannot
// spell but an in-language case can.
impl PartialEq for Value {
    fn eq(&self, other: &Value) -> bool {
        match (self, other) {
            (Value::Undefined, Value::Undefined) => true,
            (Value::Null, Value::Null) => true,
            (Value::Bool(a), Value::Bool(b)) => a == b,
            (Value::Number(a), Value::Number(b)) => same_number(*a, *b),
            (Value::String(a), Value::String(b)) => a == b,
            (Value::Array(a), Value::Array(b)) => {
                a.len() == b.len() && a.iter().zip(b).all(|(x, y)| x == y)
            }
            (Value::Object(a), Value::Object(b)) => {
                a.len() == b.len()
                    && a.iter().all(|(key, value)| {
                        b.iter()
                            .find(|(other_key, _)| other_key == key)
                            .is_some_and(|(_, other)| other == value)
                    })
            }
            _ => false,
        }
    }
}

/// `to_bits` rather than `==`, so -0.0 and 0.0 stay distinct: the two
/// comparisons differ only on signed zero and NaN, and NaN is handled
/// first so its payload does not matter.
fn same_number(a: f64, b: f64) -> bool {
    if a.is_nan() && b.is_nan() {
        return true;
    }
    a.to_bits() == b.to_bits()
}

// Renders JSON where possible, so the text lines up with how the fixture
// wrote it. `-0` is spelt `-0` (JSON.stringify would say `0`, and a
// signed-zero mismatch would then read "got 0, expected 0"), and the
// values JSON cannot spell are written the way JavaScript names them:
// `undefined`, `NaN`, `Infinity`, `-Infinity`.
impl fmt::Display for Value {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Value::Undefined => formatter.write_str("undefined"),
            Value::Null => formatter.write_str("null"),
            Value::Bool(b) => write!(formatter, "{b}"),
            Value::Number(n) => formatter.write_str(&format_number(*n)),
            Value::String(s) => write_json_string(formatter, s),
            Value::Array(items) => {
                formatter.write_str("[")?;
                for (index, item) in items.iter().enumerate() {
                    if index > 0 {
                        formatter.write_str(",")?;
                    }
                    write!(formatter, "{item}")?;
                }
                formatter.write_str("]")
            }
            Value::Object(entries) => {
                formatter.write_str("{")?;
                for (index, (key, value)) in entries.iter().enumerate() {
                    if index > 0 {
                        formatter.write_str(",")?;
                    }
                    write_json_string(formatter, key)?;
                    write!(formatter, ":{value}")?;
                }
                formatter.write_str("}")
            }
        }
    }
}

fn format_number(n: f64) -> String {
    if n.is_nan() {
        return "NaN".to_string();
    }
    if n.is_infinite() {
        return if n > 0.0 { "Infinity" } else { "-Infinity" }.to_string();
    }
    if n == 0.0 {
        return if n.is_sign_negative() { "-0" } else { "0" }.to_string();
    }
    let magnitude = n.abs();
    if !(1e-6..1e21).contains(&magnitude) {
        // JavaScript writes the exponent with an explicit sign (`1e+21`);
        // Rust's `{:e}` leaves a positive one bare.
        let exp = format!("{n:e}");
        return match exp.find('e') {
            Some(at) if !exp[at + 1..].starts_with('-') => {
                format!("{}e+{}", &exp[..at], &exp[at + 1..])
            }
            _ => exp,
        };
    }
    // `Display` already writes an integral f64 without a fraction, at every
    // magnitude below 1e21. A cast through i64 would saturate above 2^63.
    format!("{n}")
}

fn write_json_string(formatter: &mut fmt::Formatter<'_>, text: &str) -> fmt::Result {
    formatter.write_str("\"")?;
    for c in text.chars() {
        match c {
            '"' => formatter.write_str("\\\"")?,
            '\\' => formatter.write_str("\\\\")?,
            '\n' => formatter.write_str("\\n")?,
            '\r' => formatter.write_str("\\r")?,
            '\t' => formatter.write_str("\\t")?,
            '\u{8}' => formatter.write_str("\\b")?,
            '\u{c}' => formatter.write_str("\\f")?,
            c if (c as u32) < 0x20 => write!(formatter, "\\u{:04x}", c as u32)?,
            c => write!(formatter, "{c}")?,
        }
    }
    formatter.write_str("\"")
}

// A small, strict JSON reader over the bytes of a cell.
struct Reader<'a> {
    bytes: &'a [u8],
    at: usize,
}

impl Reader<'_> {
    fn skip_space(&mut self) {
        while self.at < self.bytes.len()
            && matches!(self.bytes[self.at], b' ' | b'\t' | b'\n' | b'\r')
        {
            self.at += 1;
        }
    }

    fn peek(&self) -> Option<u8> {
        self.bytes.get(self.at).copied()
    }

    fn value(&mut self) -> Result<Value, String> {
        match self.peek() {
            None => Err("unexpected end of text".to_string()),
            Some(b'{') => self.object(),
            Some(b'[') => self.array(),
            Some(b'"') => self.string().map(Value::String),
            Some(b't') => self.literal("true", Value::Bool(true)),
            Some(b'f') => self.literal("false", Value::Bool(false)),
            Some(b'n') => self.literal("null", Value::Null),
            Some(b'-' | b'0'..=b'9') => self.number(),
            Some(other) => Err(format!(
                "unexpected character {:?} at byte {}",
                other as char, self.at
            )),
        }
    }

    fn literal(&mut self, word: &str, value: Value) -> Result<Value, String> {
        if self.bytes[self.at..].starts_with(word.as_bytes()) {
            self.at += word.len();
            Ok(value)
        } else {
            Err(format!("unexpected token at byte {}", self.at))
        }
    }

    fn number(&mut self) -> Result<Value, String> {
        let start = self.at;
        if self.peek() == Some(b'-') {
            self.at += 1;
        }
        match self.peek() {
            Some(b'0') => self.at += 1,
            Some(b'1'..=b'9') => self.digits(),
            _ => return Err(format!("invalid number at byte {start}")),
        }
        if self.peek() == Some(b'.') {
            self.at += 1;
            if !matches!(self.peek(), Some(b'0'..=b'9')) {
                return Err(format!("invalid number at byte {start}"));
            }
            self.digits();
        }
        if matches!(self.peek(), Some(b'e' | b'E')) {
            self.at += 1;
            if matches!(self.peek(), Some(b'+' | b'-')) {
                self.at += 1;
            }
            if !matches!(self.peek(), Some(b'0'..=b'9')) {
                return Err(format!("invalid number at byte {start}"));
            }
            self.digits();
        }
        let text = std::str::from_utf8(&self.bytes[start..self.at]).expect("ASCII digits");
        // `str::parse::<f64>` answers an infinity for an out-of-range
        // literal rather than failing, which is what JSON.parse does and
        // what keeps a row that loads in TypeScript loading here.
        text.parse::<f64>()
            .map(Value::Number)
            .map_err(|error| format!("invalid number {text:?}: {error}"))
    }

    fn digits(&mut self) {
        while matches!(self.peek(), Some(b'0'..=b'9')) {
            self.at += 1;
        }
    }

    fn string(&mut self) -> Result<String, String> {
        self.at += 1; // the opening quote
        let mut out = String::new();
        loop {
            let Some(byte) = self.peek() else {
                return Err("unterminated string".to_string());
            };
            match byte {
                b'"' => {
                    self.at += 1;
                    return Ok(out);
                }
                b'\\' => {
                    self.at += 1;
                    let Some(escaped) = self.peek() else {
                        return Err("unterminated string escape".to_string());
                    };
                    self.at += 1;
                    match escaped {
                        b'"' => out.push('"'),
                        b'\\' => out.push('\\'),
                        b'/' => out.push('/'),
                        b'b' => out.push('\u{8}'),
                        b'f' => out.push('\u{c}'),
                        b'n' => out.push('\n'),
                        b'r' => out.push('\r'),
                        b't' => out.push('\t'),
                        b'u' => {
                            let unit = self.hex4()?;
                            if (0xD800..=0xDBFF).contains(&unit) {
                                // A high surrogate pairs with an immediately
                                // following low one; anything else is a lone
                                // surrogate, which a UTF-8 string spells as
                                // U+FFFD, as Go's encoding/json does.
                                let low = if self.bytes[self.at..].starts_with(b"\\u") {
                                    let mark = self.at;
                                    self.at += 2;
                                    match self.hex4() {
                                        Ok(low) if (0xDC00..=0xDFFF).contains(&low) => Some(low),
                                        _ => {
                                            self.at = mark;
                                            None
                                        }
                                    }
                                } else {
                                    None
                                };
                                match low {
                                    Some(low) => {
                                        let scalar =
                                            0x10000 + ((unit - 0xD800) << 10) + (low - 0xDC00);
                                        out.push(char::from_u32(scalar).unwrap_or('\u{FFFD}'));
                                    }
                                    None => out.push('\u{FFFD}'),
                                }
                            } else {
                                out.push(char::from_u32(unit).unwrap_or('\u{FFFD}'));
                            }
                        }
                        other => {
                            return Err(format!("invalid escape \\{}", other as char));
                        }
                    }
                }
                byte if byte < 0x20 => {
                    return Err("control character in string".to_string());
                }
                _ => {
                    // Copy one whole UTF-8 sequence. The input is a `&str`,
                    // so the bytes are valid UTF-8 and the sequence length
                    // follows from its first byte.
                    let width = match byte {
                        0x00..=0x7F => 1,
                        0xC0..=0xDF => 2,
                        0xE0..=0xEF => 3,
                        _ => 4,
                    };
                    let end = (self.at + width).min(self.bytes.len());
                    out.push_str(
                        std::str::from_utf8(&self.bytes[self.at..end]).unwrap_or("\u{FFFD}"),
                    );
                    self.at = end;
                }
            }
        }
    }

    fn hex4(&mut self) -> Result<u32, String> {
        if self.at + 4 > self.bytes.len() {
            return Err("truncated \\u escape".to_string());
        }
        let text = std::str::from_utf8(&self.bytes[self.at..self.at + 4])
            .map_err(|_| "invalid \\u escape".to_string())?;
        let unit = u32::from_str_radix(text, 16).map_err(|_| "invalid \\u escape".to_string())?;
        self.at += 4;
        Ok(unit)
    }

    fn array(&mut self) -> Result<Value, String> {
        self.at += 1; // [
        let mut items = Vec::new();
        self.skip_space();
        if self.peek() == Some(b']') {
            self.at += 1;
            return Ok(Value::Array(items));
        }
        loop {
            self.skip_space();
            items.push(self.value()?);
            self.skip_space();
            match self.peek() {
                Some(b',') => self.at += 1,
                Some(b']') => {
                    self.at += 1;
                    return Ok(Value::Array(items));
                }
                _ => return Err(format!("expected , or ] at byte {}", self.at)),
            }
        }
    }

    fn object(&mut self) -> Result<Value, String> {
        self.at += 1; // {
        let mut object = Value::Object(Vec::new());
        self.skip_space();
        if self.peek() == Some(b'}') {
            self.at += 1;
            return Ok(object);
        }
        loop {
            self.skip_space();
            if self.peek() != Some(b'"') {
                return Err(format!("expected a string key at byte {}", self.at));
            }
            let key = self.string()?;
            self.skip_space();
            if self.peek() != Some(b':') {
                return Err(format!("expected : at byte {}", self.at));
            }
            self.at += 1;
            self.skip_space();
            let value = self.value()?;
            object.insert(key, value);
            self.skip_space();
            match self.peek() {
                Some(b',') => self.at += 1,
                Some(b'}') => {
                    self.at += 1;
                    return Ok(object);
                }
                _ => return Err(format!("expected , or }} at byte {}", self.at)),
            }
        }
    }
}

#[cfg(feature = "serde_json")]
impl From<&serde_json::Value> for Value {
    /// A serde_json value as a fixture value. Numbers widen to `f64`, as
    /// every number here is one.
    fn from(value: &serde_json::Value) -> Self {
        match value {
            serde_json::Value::Null => Value::Null,
            serde_json::Value::Bool(b) => Value::Bool(*b),
            serde_json::Value::Number(n) => Value::Number(n.as_f64().unwrap_or(f64::NAN)),
            serde_json::Value::String(s) => Value::String(s.clone()),
            serde_json::Value::Array(items) => {
                Value::Array(items.iter().map(Value::from).collect())
            }
            serde_json::Value::Object(entries) => Value::Object(
                entries
                    .iter()
                    .map(|(key, value)| (key.clone(), Value::from(value)))
                    .collect(),
            ),
        }
    }
}

#[cfg(feature = "serde_json")]
impl From<serde_json::Value> for Value {
    fn from(value: serde_json::Value) -> Self {
        Value::from(&value)
    }
}

#[cfg(feature = "serde_json")]
impl From<&Value> for serde_json::Value {
    /// A fixture value as a serde_json one. `Undefined` becomes `null`,
    /// and a number serde_json cannot hold (an infinity, NaN) becomes
    /// `null` too, since that type has nowhere to put them.
    fn from(value: &Value) -> Self {
        match value {
            Value::Undefined | Value::Null => serde_json::Value::Null,
            Value::Bool(b) => serde_json::Value::Bool(*b),
            Value::Number(n) => serde_json::Number::from_f64(*n)
                .map(serde_json::Value::Number)
                .unwrap_or(serde_json::Value::Null),
            Value::String(s) => serde_json::Value::String(s.clone()),
            Value::Array(items) => {
                serde_json::Value::Array(items.iter().map(serde_json::Value::from).collect())
            }
            Value::Object(entries) => serde_json::Value::Object(
                entries
                    .iter()
                    .map(|(key, value)| (key.clone(), serde_json::Value::from(value)))
                    .collect(),
            ),
        }
    }
}
