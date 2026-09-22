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
 * Session credentials cannot write `.github/workflows/*` (ADR-8), so the
 * file this asserts over is the STAGED copy in `ci/workflows/`, which a
 * maintainer promotes. Asserting the staged copy is the point: it is the
 * artifact this repo can change, and a staged fix that silently loses
 * the third tag is the same defect again.
 *
 * The deployed copy is deliberately NOT read here. It is out of this
 * repo's reach until promotion, so comparing against it would turn a
 * pending promotion into a red suite on main.
 */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const REPO = Path.join(__dirname, '..', '..')
const STAGED = Path.join(REPO, 'ci', 'workflows', 'release.yml')

const TAGS = ['ts/v$V', 'go/v$V', 'go/adder/v$V']


describe('release workflow', () => {

  it('stages a release.yml to promote', () => {
    assert.ok(
      Fs.existsSync(STAGED),
      'ci/workflows/release.yml is missing: workflow changes are staged there (ADR-8)')
  })

  it('tags all three refs from one list', () => {
    const src = Fs.readFileSync(STAGED, 'utf8')

    // ONE list, not three greps. The already-released guard, the anchor
    // choice and the atomic push all read this loop, so a tag named
    // anywhere else would be tagged without being guarded.
    const loop = src.match(/^\s*for T in ([^\n]*?); do$/m)
    assert.ok(loop, 'no `for T in ...; do` tag list in the staged workflow')

    const tags = loop[1].match(/"([^"]+)"/g).map((s) => s.slice(1, -1))
    assert.deepEqual(tags, TAGS)
  })

  it('pushes the tag list atomically', () => {
    const src = Fs.readFileSync(STAGED, 'utf8')

    // Pushed one at a time, ts/v could land and go/adder/v fail, leaving
    // npm published and the nested module unreleased.
    assert.match(src, /git push --atomic origin \$\{\{ steps\.tags\.outputs\.missing \}\}/)
  })

  it('gates every go tag behind the go input', () => {
    const src = Fs.readFileSync(STAGED, 'utf8')

    // `go/*` matches go/adder/v0.3.5 as well as go/v0.3.5, so the third
    // tag needs no second case arm — but it does need this one to stay a
    // prefix glob rather than becoming an exact match.
    assert.match(src, /go\/\*\)\s*\[ "\$\{\{ inputs\.go \}\}" = "true" \] \|\| continue/)
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
