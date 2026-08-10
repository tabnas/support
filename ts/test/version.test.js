/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

/* version.test.js — the two runtimes ship one version number.
 *
 * The Go module has no package.json to read, so `go/version_test.go`
 * checks its own constant against `ts/package.json` from disk. This side
 * checks the exported constant against the same file, which leaves the
 * published npm version as the single source of truth for both.
 */

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { VERSION } = require('../dist/support.js')
const Pkg = require('../package.json')


describe('version', () => {

  it('matches package.json', () => {
    assert.equal(VERSION, Pkg.version)
  })

  it('is a plain semver triple', () => {
    assert.match(VERSION, /^\d+\.\d+\.\d+$/)
  })
})
