/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* enginepin.test.js — the two runtimes test against ONE engine version.
 *
 * support is the library every tabnas repo's parity mechanism runs through,
 * and go/adder is the end-to-end check that its two runtimes agree. That
 * check means nothing if each runtime is wired to a different engine: the
 * committed lockfile pinned @tabnas/parser 0.8.1 while go/adder/go.mod
 * required parser/go v0.8.0 and the fleet shipped 0.8.10, so the "the two
 * agree" result was measured across two different stale engines — and
 * nothing anywhere said so.
 *
 * Neither pin moves on its own. The devDependency range is `*`, so there is
 * no version in package.json for Renovate to bump, and the lockfile only
 * changes when someone runs `npm run reset` (which deletes it). That is why
 * this is a test and not a convention: a drift no mechanism can report is a
 * drift that lasts until someone reads the two files side by side.
 *
 * CI itself builds both runtimes against sibling SOURCE (see the `deps:`
 * input in .github/workflows/ci.yml), so the skew never showed up there.
 * It hits every local `make test` — which is where the parity claim is
 * usually checked.
 *
 * Fixing a failure: `npm update --package-lock-only @tabnas/parser` in ts/,
 * then set the same version in go/adder/go.mod and `go mod tidy`.
 */

const { describe, it } = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO = path.join(__dirname, '..', '..')

// The version npm would install, not the range that asks for it. `*` tells
// you nothing; the lockfile is what a build actually gets.
function lockedParserVersion() {
  const lock = JSON.parse(
    fs.readFileSync(path.join(REPO, 'ts', 'package-lock.json'), 'utf8'))
  const entry = lock.packages && lock.packages['node_modules/@tabnas/parser']
  assert.ok(entry, 'ts/package-lock.json has no @tabnas/parser entry')
  assert.ok(!entry.link,
    'ts/package-lock.json resolves @tabnas/parser to a local link ' +
    `(${entry.resolved}) — a committed lockfile that points outside the repo ` +
    'installs only on the machine it was written on')
  return entry.version
}

// go/adder is a separate module precisely so the support module itself can
// stay dependency-free; its go.mod is the only place the Go side names the
// engine.
function requiredParserVersion() {
  const mod = fs.readFileSync(path.join(REPO, 'go', 'adder', 'go.mod'), 'utf8')
  const m = /^\s*github\.com\/tabnas\/parser\/go\s+v(\S+)/m.exec(mod)
  assert.ok(m, 'go/adder/go.mod does not require github.com/tabnas/parser/go')
  return m[1]
}


describe('engine pin', () => {

  it('both runtimes name the same @tabnas/parser version', () => {
    const ts = lockedParserVersion()
    const go = requiredParserVersion()
    assert.equal(
      ts, go,
      `ts/package-lock.json pins @tabnas/parser ${ts} but go/adder/go.mod ` +
      `requires parser/go v${go}. The adder suite is the end-to-end check ` +
      'that the two runtimes agree; run across two engine versions it ' +
      'compares nothing.',
    )
  })

  it('the pinned version is a plain semver triple', () => {
    assert.match(lockedParserVersion(), /^\d+\.\d+\.\d+$/)
  })
})
