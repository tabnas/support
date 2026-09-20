#!/usr/bin/env bash
# Rust gate. Kept in one script so local and hosted validation cannot
# quietly drift apart: ci/workflows/rust.yml runs this file, and so can
# you. `make test-rs` is the fast inner loop; this is the full gate.
#
# Two crates: rs/ (tabnas-support, no dependencies) and rs/adder/ (the
# adder grammar, which takes the engine as a PATH DEPENDENCY on the
# sibling checkout of https://github.com/tabnas/parser). Clone the engine
# next to this repo before running the adder half.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
ENGINE="$ROOT/../parser/rs"

if [[ ! -f "$ENGINE/Cargo.toml" ]]; then
  echo "no engine checkout at $ENGINE" >&2
  echo "clone https://github.com/tabnas/parser as a sibling of $(basename "$ROOT")" >&2
  exit 1
fi

# Run through the MSRV toolchain when one is available. A newer toolchain
# accepts code and formatting that the MSRV rejects, so the "local and
# hosted cannot drift" claim this script exists for would otherwise hold
# everywhere except the compiler version. Loud when the toolchain is
# absent, because a quiet fallback is the drift.
MSRV=$(awk -F'"' '/^rust-version = /{print $2; exit}' "$ROOT/rs/Cargo.toml")
CARGO=(cargo)
if [[ -n "$MSRV" ]]; then
  if command -v rustup >/dev/null 2>&1 && rustup toolchain list | grep -q "^$MSRV"; then
    CARGO=(cargo "+$MSRV")
  else
    echo "warning: MSRV $MSRV is not installed; running on $(rustc --version 2>/dev/null)" >&2
    echo "         install it with: rustup toolchain install $MSRV" >&2
  fi
fi

# The lock's entry for a crate must match its manifest BEFORE anything
# runs cargo: a cargo command silently rewrites Cargo.lock, so a version
# bump that updates Cargo.toml and forgets Cargo.lock would pass every
# check and ship a stale lock. The engine's own entry legitimately moves
# whenever the sibling checkout does, so the whole-file comparison below
# exempts exactly it.
check_lock_version() {
  local dir=$1
  local crate want have
  crate=$(awk -F'"' '/^name = /{print $2; exit}' "$dir/Cargo.toml")
  want=$(awk -F'"' '/^version = /{print $2; exit}' "$dir/Cargo.toml")
  have=$(awk -v c="$crate" -F'"' '
    $0 == "name = \"" c "\"" { f = 1; next }
    f && /^version = / { print $2; exit }
  ' "$dir/Cargo.lock")
  if [[ "$want" != "$have" ]]; then
    echo "$dir/Cargo.lock records $crate ${have:-<missing>}, but Cargo.toml says $want" >&2
    echo "run a cargo command and commit the updated Cargo.lock" >&2
    exit 1
  fi
}

lock_without_sibling_versions() {
  awk '
    /^\[\[package\]\]$/  { sib = 0 }
    /^name = "tabnas"$/  { sib = 1 }
    sib && /^version = / { print "version = \"<sibling>\""; next }
                         { print }
  ' "$1"
}

gate() {
  local dir=$1
  local no_features=${2:-}
  cd "$dir"
  check_lock_version "$dir"

  local before
  before=$(mktemp)
  cp Cargo.lock "$before"

  # NOT `--all` for fmt: the adder crate's path dependency is the engine,
  # and `--all` would format the sibling checkout too.
  "${CARGO[@]}" fmt --check
  "${CARGO[@]}" build --all-targets
  "${CARGO[@]}" test --all-targets
  # `--all-targets` does NOT include doctests.
  "${CARGO[@]}" test --doc
  "${CARGO[@]}" clippy --all-targets --all-features -- -D warnings
  # AFTER the snapshot above, like every other cargo command here: run
  # before it, this pass would rewrite a stale lock and the comparison
  # below would then bless the rewritten file.
  if [[ "$no_features" == "--no-features" ]]; then
    "${CARGO[@]}" clippy --all-targets -- -D warnings
  fi

  if ! diff -q <(lock_without_sibling_versions "$before") \
               <(lock_without_sibling_versions Cargo.lock) >/dev/null; then
    echo "$dir/Cargo.lock does not match Cargo.toml: cargo rewrote it:" >&2
    diff <(lock_without_sibling_versions "$before") \
         <(lock_without_sibling_versions Cargo.lock) >&2 || true
    cp "$before" Cargo.lock
    rm -f "$before"
    exit 1
  fi
  # The exempted sibling version may still have moved, and cargo wrote it
  # into the lock. Put the lock back so a green gate leaves the tree
  # exactly as it found it: updating the committed lock is a deliberate
  # cargo run and commit, never a side effect of running the gate.
  if ! cmp -s "$before" Cargo.lock; then
    cp "$before" Cargo.lock
  fi
  rm -f "$before"
}

# The support crate must also build with no features at all: that is the
# configuration every consumer that does not opt into serde_json gets.
gate "$ROOT/rs" --no-features
gate "$ROOT/rs/adder"
