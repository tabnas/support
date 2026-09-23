/* Copyright (c) 2026 tabnas, MIT License */

/* release.test.js — the release tags TWO refs, `ts/v` and `go/v`, and
 * never a `go/adder` one.
 *
 * `go/adder/` is a separate Go module, but it is a PRIVATE INTERNAL TEST
 * MODULE: never published and never tagged, by the maintainer's decision
 * (admin#19, recorded in admin/publish.sh). Only this repository consumes
 * it, and CI builds it from source. For a while this file asserted the
 * opposite: #27 taught the workflow a third tag, `go/adder/v$V`, and this
 * suite held it there. #21 is where that was reversed. The two adder tags
 * that exist, v0.2.0 and v0.3.0, predate the decision and stay, because a
 * Go tag is immutable once the proxy has served it; the rule is only that
 * nothing creates another.
 *
 * A decision that lives only in prose gets "fixed" back by the next person
 * who reads the nested module as a gap, which is how the third tag arrived
 * in the first place. So the suite enforces it: the tag list is exactly
 * the fleet's two, nothing else in the workflow tags or pushes, and the
 * release docs stop describing a per-release adder tag.
 *
 * Both copies of the workflow are checked when both exist. The deployed
 * `.github/workflows/release.yml` is required: it is the one that runs. A
 * candidate staged in `ci/workflows/release.yml` (the ADR-8 route, for
 * credentials that cannot write workflow files) is optional, and held to
 * the same rules, since a candidate that brings the adder tag back is the
 * deployed file's defect the day it is promoted. Promotion deletes the
 * staged copy, which is why neither path may be assumed to exist alone.
 */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const REPO = Path.join(__dirname, '..', '..')
const DEPLOYED = Path.join(REPO, '.github', 'workflows', 'release.yml')
const STAGED = Path.join(REPO, 'ci', 'workflows', 'release.yml')

const TAGS = ['ts/v$V', 'go/v$V']

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

// The workflow with its comments removed: whole-line `#` comments and
// trailing ` # ...` ones (the action pins carry their release that way).
// The header explains, in prose, why there is no adder tag, and it has to
// be able to name the module to do that. What must not name it is
// anything that runs.
function executable(src) {
  return src
    .split('\n')
    .filter((line) => !/^\s*#/.test(line))
    .map((line) => line.replace(/\s+#.*$/, ''))
    .join('\n')
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

  it('tags ts/v and go/v, from one list', () => {
    forEachCopy((src) => {

      // ONE list, not greps. The already-released guard, the anchor
      // choice and the atomic push all read this loop, so a tag named
      // anywhere else would be tagged without being guarded. (The tag
      // step's own `for T in ${{ steps.tags.outputs.missing }}` iterates
      // what this list decided; it names no tag, so it is not a list.)
      const loops = src.match(/^\s*for T in ("[^\n]*?); do$/gm)
      assert.ok(loops, 'no `for T in "..."; do` tag list')
      assert.equal(loops.length, 1, 'more than one literal tag list: ' + loops.join(' / '))

      const list = loops[0].match(/for T in ([^\n]*?); do$/)[1]
      const tags = list.match(/"([^"]+)"/g).map((s) => s.slice(1, -1))
      assert.deepEqual(tags, TAGS)
    })
  })

  it('creates no go/adder tag: the nested module is private', () => {
    forEachCopy((src) => {
      const code = executable(src)

      // Not in the list, not in the input description, not in a second
      // tagging step: nothing that runs names the module at all.
      assert.doesNotMatch(
        code, /adder/i,
        'release.yml names go/adder outside a comment; the module is never tagged (admin#19, #21)')

      // And there is no other way to make a tag. Every `git tag` and
      // `git push` in the file is the loop's, so a nested module cannot
      // be tagged by a step that builds its name some other way.
      assert.deepEqual(
        code.match(/\bgit tag\b[^\n]*/g),
        ['git tag "$T" "${{ steps.tags.outputs.anchor }}"'])
      assert.deepEqual(
        code.match(/\bgit push\b[^\n]*/g),
        ['git push --atomic origin ${{ steps.tags.outputs.missing }}'])
    })
  })

  it('pushes the tag list atomically', () => {
    forEachCopy((src) => {

      // Pushed one at a time, ts/v could land and go/v fail, leaving npm
      // published and the Go module unreleased, and the "every tag
      // exists" guard is then no help: one of the two still does not.
      assert.match(src, /git push --atomic origin \$\{\{ steps\.tags\.outputs\.missing \}\}/)
    })
  })

  it('gates the go tag behind the go input', () => {
    forEachCopy((src) => {

      // A prefix glob, as in the fleet copy, so `go: false` publishes and
      // tags ts/v alone.
      assert.match(src, /go\/\*\)\s*\[ "\$\{\{ inputs\.go \}\}" = "true" \] \|\| continue/)
    })
  })
})


// Markdown pages under `dir`, as repo-relative paths with `/` separators.
// Dependencies, build output and Vale's downloaded styles are not this
// repository's prose. `.git` is skipped by name, since it is a directory
// in a clone and a file in a worktree.
const SKIP = new Set(['.git', 'node_modules', 'dist', 'target', '.vale'])

function markdownPages(dir) {
  const found = []
  for (const entry of Fs.readdirSync(dir, { withFileTypes: true })) {
    if (SKIP.has(entry.name)) continue
    const full = Path.join(dir, entry.name)
    if (entry.isDirectory()) found.push(...markdownPages(full))
    else if (entry.name.endsWith('.md')) {
      found.push(Path.relative(REPO, full).split(Path.sep).join('/'))
    }
  }
  return found
}


describe('release documentation', () => {

  // A per-release adder tag, in any of the spellings these pages use for
  // "the version being released". A concrete version (the historical
  // go/adder/v0.2.0 and v0.3.0) is a fact and may be named; a templated
  // one is an instruction, and there is no such tag to create or check.
  const PER_RELEASE_ADDER_TAG = /go\/adder\/v(?:\$|X\.Y\.Z|<)/

  // Every Markdown page in the repository, found by walking it rather
  // than listed by hand: a hand list is the page someone forgets, and
  // go/README.md was that page in this suite's first draft. Plus the
  // Makefile, whose publish-go is the other way a tag gets made.
  const pages = [...markdownPages(REPO), 'Makefile'].sort()

  it('finds the pages it has to check', () => {
    // A walk that finds nothing would pass every per-page check below.
    for (const page of ['AGENTS.md', 'README.md', 'ci/README.md',
      'go/README.md', 'doc/reference.md', 'Makefile']) {
      assert.ok(pages.includes(page), page + ' was not found by the walk')
    }
  })

  for (const page of pages) {
    it(page + ' describes no per-release go/adder tag', () => {
      const src = Fs.readFileSync(Path.join(REPO, page), 'utf8')
      const hit = src.split('\n').find((line) => PER_RELEASE_ADDER_TAG.test(line))
      assert.equal(hit, undefined, page + ' still describes a go/adder release tag: ' + hit)
    })
  }

  // The confirmation script in the release steps checks the tags the
  // workflow creates. A script that checks a third tag fails every
  // release; one that checks fewer passes a release that is missing one.
  it('AGENTS.md confirms the same tags the workflow creates', () => {
    const src = Fs.readFileSync(Path.join(REPO, 'AGENTS.md'), 'utf8')
    const loops = src.match(/^\s*for T in ([^\n]*?); do$/gm)
    assert.ok(loops, 'AGENTS.md has no `for T in ...; do` confirmation loop')
    for (const loop of loops) {
      const tags = loop.match(/for T in ([^\n]*?); do$/)[1]
        .match(/"([^"]+)"/g).map((s) => s.slice(1, -1))
      assert.deepEqual(tags, TAGS, 'AGENTS.md: ' + loop.trim())
    }
  })
})
