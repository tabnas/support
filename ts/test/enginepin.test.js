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
 *   - Sibling source (a symlink, or a copy of one): CI clones the sibling
 *     repos and link_siblings replaces the published copy with a link to
 *     ../parser/ts, so the lockfile is not what the TS side loads at all and
 *     its version legitimately runs ahead of the last publish. The lockfile
 *     is not compared against it — but the lock-vs-go.mod check still
 *     applies, because that is what every LOCAL run uses.
 *     On windows the substitution is a copy even when the log says "linked"
 *     (MSYS `ln -s` copies and exits 0), so lstat alone does not answer
 *     "is this the sibling?" — see isSiblingSource.
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
  if (null != installed && !installed.fromSibling && installed.version !== locked) {
    problems.push(
      `node_modules/${ENGINE} is ${installed.version} but ` +
      `ts/package-lock.json pins ${locked}. The pins can agree with each ` +
      'other and still be a version this test run never loaded — run ' +
      '`npm i` in ts/.')
  }

  return problems
}


// --- reading the real repository ------------------------------------

// A symlink is not the only way a sibling gets into node_modules, and on
// windows it is not even the usual one. link_siblings runs `ln -s` and, when
// that exits 0 leaving a readable package.json, logs "linked" — but under
// MSYS `ln -s` COPIES the tree rather than linking it and still exits 0. So
// the windows log reads "linked @tabnas/parser -> parser/ts" while what
// landed on disk is an ordinary directory. (The explicit `cp -R` fallback
// beside it reaches the same place; neither leaves a symlink.)
//
// That directory carries the SIBLING's version, which lstat cannot tell from
// a registry install — so on windows the exemption never applies and the
// sibling is compared against the lockfile. It passes only while the two
// agree and fails the moment one moves, which is what happened in tabnas/json
// (@tabnas/support 0.3.4 read as a registry install against a pin of 0.3.3).
// Not flakiness; a signal that is simply absent on one platform.
function isSiblingSource({ linked, version, siblingVersion }) {
  if (linked) return true
  // A copy counts only if there IS a sibling beside this repo and what is
  // installed is that version. A registry install on a machine with no
  // sibling checkout has nothing to match, and one that differs from the
  // sibling is still a registry install.
  return null != siblingVersion && null != version && siblingVersion === version
}

// The sibling sits beside this repo at ../<dep>/ts — the layout
// link_siblings uses ($ROOT/$dep/ts), and the one CONTRIBUTING.md asks for.
function siblingVersionOf(name) {
  const dep = name.replace(/^@[^/]+\//, '')
  try {
    return JSON.parse(fs.readFileSync(
      path.join(REPO, '..', dep, 'ts', 'package.json'), 'utf8')).version
  } catch {
    return null
  }
}

function readInstalled(name) {
  const dir = path.join(REPO, 'ts', 'node_modules', name)
  let st
  try {
    // lstat, not stat: a symlink here is conclusive sibling source. Its
    // ABSENCE is not conclusive, which is what isSiblingSource settles.
    st = fs.lstatSync(dir)
  } catch {
    return null
  }
  let version = null
  try {
    version = JSON.parse(
      fs.readFileSync(path.join(dir, 'package.json'), 'utf8')).version
  } catch { /* present but unreadable; reported as a mismatch below */ }
  return {
    version,
    fromSibling: isSiblingSource({
      linked: st.isSymbolicLink(),
      version,
      siblingVersion: siblingVersionOf(name),
    }),
  }
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
      : REAL.installed.fromSibling ? `sibling source (${REAL.installed.version})`
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
        installed: { version: '0.8.1', fromSibling: false },
      })
      assert.equal(p.length, 1)
      assert.match(p[0], /node_modules\/@tabnas\/parser is 0\.8\.1 but/)
    })
  })

  // How the sibling got there is the platform's choice, not a difference the
  // guard should see. On windows there is no symlink to read, so this is the
  // branch that decides whether CI is green there at all.
  describe('sibling source is recognised however it was substituted', () => {

    it('a symlink is sibling source', () => {
      assert.equal(isSiblingSource(
        { linked: true, version: '0.9.0', siblingVersion: null }), true)
    })

    it('a copy matching the sibling checkout is sibling source', () => {
      assert.equal(isSiblingSource(
        { linked: false, version: '0.9.0', siblingVersion: '0.9.0' }), true)
    })

    it('a registry install with no sibling checkout is not', () => {
      assert.equal(isSiblingSource(
        { linked: false, version: '0.8.10', siblingVersion: null }), false)
    })

    it('a registry install beside a different sibling is not', () => {
      assert.equal(isSiblingSource(
        { linked: false, version: '0.8.10', siblingVersion: '0.9.0' }), false)
    })
  })

  // The other half of the same rule: a guard that fires when it SHOULDN'T is
  // just as broken, and this is the case CI runs on every push.
  it('sibling source ahead of the lockfile is not a problem', () => {
    assert.deepEqual(
      enginePinProblems({
        lock: lockAt('0.8.10'),
        goMod: goModAt('0.8.10'),
        installed: { version: '0.9.0-dev', fromSibling: true },
      }),
      [],
    )
  })
})
