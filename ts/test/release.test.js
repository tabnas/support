/* Copyright (c) 2026 tabnas, MIT License */

/* release.test.js — the release tags THREE refs, and the staged workflow
 * is where that is true.
 *
 * `go/adder/` is a separate module, and Go finds a nested module only
 * under its own path prefix, so `go/adder/vX.Y.Z` is what makes
 * `github.com/tabnas/support/go/adder` resolvable at all. The deployed
 * workflow created `ts/v` and `go/v` and stopped there, so four
 * consecutive dispatch-driven releases (0.3.1 to 0.3.4) left the nested
 * module unresolvable while every release-time check passed. Nothing
 * fails at release time; the breakage surfaces later, in a consumer.
 *
 * Session credentials cannot WRITE `.github/workflows/*` (ADR-8) -- but
 * they can read it, so BOTH copies are checked, and neither stands in
 * for the other:
 *
 *   - before promotion, only `ci/workflows/release.yml` exists, and a
 *     staged fix that silently loses the third tag is the same defect
 *     again;
 *   - after promotion the staged copy is DELETED by the rollout, and the
 *     deployed file is both the thing that runs and the only one left.
 *
 * It used to read the staged copy alone, with a note that the deployed
 * one was out of reach. Promotion made that note false and the suite red
 * on main: `ci/` here now holds only README.md and rust/, so the path
 * this asserted over had simply gone.
 */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const REPO = Path.join(__dirname, '..', '..')
const DEPLOYED = Path.join(REPO, '.github', 'workflows', 'release.yml')
const STAGED = Path.join(REPO, 'ci', 'workflows', 'release.yml')

const TAGS = ['ts/v$V', 'go/v$V', 'go/adder/v$V']

// Every copy that exists, deployed first. The deployed one is required;
// a staged candidate is optional and held to the same rules.
function releaseWorkflows() {
  const found = []
  for (const [label, path] of [['deployed', DEPLOYED], ['staged', STAGED]]) {
    if (Fs.existsSync(path)) found.push({ label, path, src: Fs.readFileSync(path, 'utf8') })
  }
  return found
}

// Runs `check` over each copy, naming which one failed. A rule that holds
// for the deployed file and not for the candidate is a rule that stops
// holding the day the candidate is promoted.
function forEachCopy(check) {
  const copies = releaseWorkflows()
  assert.ok(copies.length, 'no release.yml at all: neither deployed nor staged')
  for (const copy of copies) {
    try {
      check(copy.src)
    } catch (e) {
      e.message = `${copy.label} (${Path.relative(REPO, copy.path)}): ${e.message}`
      throw e
    }
  }
}


describe('release workflow', () => {

  it('has a DEPLOYED release.yml', () => {
    // Not "one of the two". A staged candidate cannot receive a dispatch
    // or publish a release, so its presence says nothing about whether
    // this repository can still cut one.
    assert.ok(
      Fs.existsSync(DEPLOYED),
      '.github/workflows/release.yml is missing: a staged candidate cannot release')
  })

  it('tags all three refs from one list', () => {
    forEachCopy((src) => {

      // ONE list, not three greps. The already-released guard, the anchor
      // choice and the atomic push all read this loop, so a tag named
      // anywhere else would be tagged without being guarded.
      const loop = src.match(/^\s*for T in ([^\n]*?); do$/m)
      assert.ok(loop, 'no `for T in ...; do` tag list')

      const tags = loop[1].match(/"([^"]+)"/g).map((s) => s.slice(1, -1))
      assert.deepEqual(tags, TAGS)
    })
  })

  it('pushes the tag list atomically', () => {
    forEachCopy((src) => {

      // Pushed one at a time, ts/v could land and go/adder/v fail, leaving
      // npm published and the nested module unreleased.
      assert.match(src, /git push --atomic origin \$\{\{ steps\.tags\.outputs\.missing \}\}/)
    })
  })

  it('gates every go tag behind the go input', () => {
    forEachCopy((src) => {

      // `go/*` matches go/adder/v0.3.5 as well as go/v0.3.5, so the third
      // tag needs no second case arm — but it does need this one to stay a
      // prefix glob rather than becoming an exact match.
      assert.match(src, /go\/\*\)\s*\[ "\$\{\{ inputs\.go \}\}" = "true" \] \|\| continue/)
    })
  })
})


describe('release documentation', () => {

  // A guide that describes a two-tag release is the process defect this
  // issue found, written down.
  for (const page of ['AGENTS.md', 'README.md']) {
    it(page + ' names go/adder/v in the confirmation', () => {
      const src = Fs.readFileSync(Path.join(REPO, page), 'utf8')
      if (!/confirm/i.test(src)) return
      assert.ok(
        src.includes('go/adder/v$V') || src.includes('go/adder/vX.Y.Z'),
        page + ' describes the release without the nested module tag')
    })
  }
})
