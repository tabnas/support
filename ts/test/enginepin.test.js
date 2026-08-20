/* Copyright (c) 2026 tabnas, MIT License */

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
 * TWO INSTALL MODES, BOTH CHECKED. A declared pin is not the engine a test
 * run loads — that gap is the same mistake this file exists to catch, one
 * level down. So the INSTALLED copy is checked too, and how depends on where
 * it came from:
 *
 *   - Registry install (node_modules/@tabnas/parser is a real directory):
 *     its version must equal the lockfile's. A `npm update
 *     --package-lock-only` with no reinstall leaves the files agreeing while
 *     `npm test` still loads the old engine.
 *   - Sibling source (a symlink): CI clones the sibling repos and
 *     link_siblings replaces the published copy with a link to
 *     ../parser/ts, so the lockfile is not what the TS side loads at all and
 *     its version legitimately runs ahead of the last publish. The lockfile
 *     is not compared against it — but the lock-vs-go.mod check still
 *     applies, because that is what every LOCAL run uses.
 *
 * The mode is detected, never assumed, and reported in the assertion message
 * so a run can never quietly narrow what it checked.
 *
 * Fixing a failure: `npm update --package-lock-only @tabnas/parser` in ts/,
 * then set the same version in go/adder/go.mod, `go mod tidy`, and `npm i`.
 */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO = path.join(__dirname, '..', '..')
const ENGINE = '@tabnas/parser'
const ENGINE_MOD = 'github.com/tabnas/parser/go'


// --- the guard ------------------------------------------------------
//
// AGENTS.md: "A guard that can only fail a test cannot be tested." So this
// is an ordinary function over DATA — no disk, no assertions — returning the
// problems it found. The failure cases below are then ordinary assertions
// against fixtures, not a check nobody can point at.
//
// `installed` is null when nothing is installed (a lockfile-only checkout),
// which is not itself a problem: the file pins are still worth checking.
function enginePinProblems({ lock, goMod, installed }) {
  const problems = []

  const entry = lock && lock.packages && lock.packages['node_modules/' + ENGINE]
  if (null == entry) {
    problems.push(`ts/package-lock.json has no ${ENGINE} entry`)
    return problems
  }
  if (entry.link) {
    problems.push(
      `ts/package-lock.json resolves ${ENGINE} to a local link ` +
      `(${entry.resolved}) — a committed lockfile that points outside the ` +
      'repo installs only on the machine it was written on')
    return problems
  }

  const locked = entry.version
  if (!/^\d+\.\d+\.\d+$/.test(String(locked))) {
    problems.push(
      `ts/package-lock.json pins ${ENGINE} ${locked}, which is not a plain ` +
      'semver triple')
  }

  const m = new RegExp('^\\s*' + ENGINE_MOD.replace(/[.\\/]/g, '\\$&') +
    '\\s+v(\\S+)', 'm').exec(String(goMod))
  if (null == m) {
    problems.push(`go/adder/go.mod does not require ${ENGINE_MOD}`)
  } else if (m[1] !== locked) {
    problems.push(
      `ts/package-lock.json pins ${ENGINE} ${locked} but go/adder/go.mod ` +
      `requires ${ENGINE_MOD} v${m[1]}. The adder suite is the end-to-end ` +
      'check that the two runtimes agree; run across two engine versions it ' +
      'compares nothing.')
  }

  // A pin is a claim about what will be loaded; this is what IS loaded.
  if (null != installed && !installed.linked && installed.version !== locked) {
    problems.push(
      `node_modules/${ENGINE} is ${installed.version} but ` +
      `ts/package-lock.json pins ${locked}. The pins can agree with each ` +
      'other and still be a version this test run never loaded — run ' +
      '`npm i` in ts/.')
  }

  return problems
}


// --- reading the real repository ------------------------------------

function readInstalled(name) {
  const dir = path.join(REPO, 'ts', 'node_modules', name)
  let st
  try {
    // lstat, not stat: the symlink IS the signal that this came from a
    // sibling checkout rather than the registry.
    st = fs.lstatSync(dir)
  } catch {
    return null
  }
  let version = null
  try {
    version = JSON.parse(
      fs.readFileSync(path.join(dir, 'package.json'), 'utf8')).version
  } catch { /* present but unreadable; reported as a mismatch below */ }
  return { version, linked: st.isSymbolicLink() }
}

const REAL = {
  lock: JSON.parse(
    fs.readFileSync(path.join(REPO, 'ts', 'package-lock.json'), 'utf8')),
  goMod: fs.readFileSync(path.join(REPO, 'go', 'adder', 'go.mod'), 'utf8'),
  installed: readInstalled(ENGINE),
}


// --- fixtures for the failure cases ---------------------------------

const lockAt = (version, extra) => ({
  packages: { ['node_modules/' + ENGINE]: { version, ...extra } },
})
const goModAt = (version) => `require (\n\t${ENGINE_MOD} v${version}\n)\n`


describe('engine pin', () => {

  it('this repository pins one engine version everywhere', () => {
    const mode = null == REAL.installed ? 'not installed'
      : REAL.installed.linked ? `sibling source (${REAL.installed.version})`
        : `registry install (${REAL.installed.version})`
    assert.deepEqual(
      enginePinProblems(REAL), [],
      `engine pin problems (install mode: ${mode})`,
    )
  })

  // Every branch of the guard, asserted to fail when it should. Without
  // these the suite contains only the already-passing case and could not
  // tell a working guard from one that returns [] unconditionally.
  describe('fails when it should', () => {

    it('lockfile and go.mod name different versions', () => {
      const p = enginePinProblems(
        { lock: lockAt('0.8.1'), goMod: goModAt('0.8.10'), installed: null })
      assert.equal(p.length, 1)
      assert.match(p[0], /pins @tabnas\/parser 0\.8\.1 but .*v0\.8\.10/)
    })

    it('the lockfile has no entry for the engine', () => {
      const p = enginePinProblems(
        { lock: { packages: {} }, goMod: goModAt('0.8.10'), installed: null })
      assert.deepEqual(p, ['ts/package-lock.json has no @tabnas/parser entry'])
    })

    it('the lockfile entry is a local link', () => {
      const p = enginePinProblems({
        lock: lockAt(undefined, { link: true, resolved: '../../parser/ts' }),
        goMod: goModAt('0.8.10'),
        installed: null,
      })
      assert.equal(p.length, 1)
      assert.match(p[0], /local link \(\.\.\/\.\.\/parser\/ts\)/)
    })

    it('go.mod does not require the engine at all', () => {
      const p = enginePinProblems(
        { lock: lockAt('0.8.10'), goMod: 'module x\n', installed: null })
      assert.deepEqual(p, [
        'go/adder/go.mod does not require github.com/tabnas/parser/go'])
    })

    it('the pinned version is not a semver triple', () => {
      const p = enginePinProblems(
        { lock: lockAt('0.8'), goMod: goModAt('0.8'), installed: null })
      assert.equal(p.length, 1)
      assert.match(p[0], /not a plain semver triple/)
    })

    it('node_modules is stale against a lockfile-only update', () => {
      const p = enginePinProblems({
        lock: lockAt('0.8.10'),
        goMod: goModAt('0.8.10'),
        installed: { version: '0.8.1', linked: false },
      })
      assert.equal(p.length, 1)
      assert.match(p[0], /node_modules\/@tabnas\/parser is 0\.8\.1 but/)
    })
  })

  // The other half of the same rule: a guard that fires when it SHOULDN'T is
  // just as broken, and this is the case CI runs on every push.
  it('a sibling-source symlink ahead of the lockfile is not a problem', () => {
    assert.deepEqual(
      enginePinProblems({
        lock: lockAt('0.8.10'),
        goMod: goModAt('0.8.10'),
        installed: { version: '0.9.0-dev', linked: true },
      }),
      [],
    )
  })
})
