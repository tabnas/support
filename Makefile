# Build and test the TypeScript (ts/), Go (go/) and Rust (rs/)
# implementations. ts/ is canonical; go/ and rs/ track it.
#
# go/adder is a SEPARATE module — the support module itself has no
# dependencies, and the grammar that exercises it needs the parser — so
# `go test ./...` in go/ does not reach it and it is run explicitly here.
# rs/adder is a separate crate for the same reason, and is run the same way.
# Both are private test code: go/adder is never tagged and rs/adder is
# never published (see publish-go and version-rs below).

.PHONY: all build test clean build-ts build-go build-rs test-ts test-go test-go-adder \
        test-rs clean-ts clean-go clean-rs publish-ts publish-go tag-ts tags-go reset \
        fmt-go vet version version-rs \
        prose prose-counts

all: build test

build: build-ts build-go build-rs

test: test-ts test-go test-go-adder test-rs

clean: clean-ts clean-go clean-rs

# --- TypeScript (package in ts/) ---
build-ts:
	cd ts && npm run build

test-ts:
	cd ts && npm test

clean-ts:
	rm -rf ts/dist

# Tag the TypeScript release: make tag-ts V=x.y.z
#
# THIS is how the npm package is published. Pushing a `ts/vX.Y.Z` tag
# triggers .github/workflows/release.yml, which publishes via OIDC
# trusted publishing — no token, and the release carries provenance.
#
# The `ts/` prefix is load-bearing: the workflow triggers on `ts/v*`, and
# a plain `vX.Y.Z` tag publishes NOTHING. That is not hypothetical — the
# 0.1.1 release was tagged `v0.1.1`, no workflow ran, and the package was
# then published by hand with no attestations.
tag-ts:
	@test -n "$(V)" || (echo "Usage: make tag-ts V=x.y.z" && exit 1)
	@TS_V=`node -e "console.log(require('./ts/package.json').version)"`; \
	  test "$$TS_V" = "$(V)" || \
	  (echo "ts/package.json is at $$TS_V, not $(V) — run make version V=$(V) first" && exit 1)
	# The tag must point at a commit the remote already has, or it
	# references something nobody else can see.
	@git fetch -q origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || \
	  (echo "HEAD is not origin/main — merge and push the release commit first" && exit 1)
	@T=ts/v$(V); \
	  if git rev-parse -q --verify "refs/tags/$$T" >/dev/null; then \
	    if [ "$$(git rev-parse "refs/tags/$$T^{commit}")" != "$$(git rev-parse HEAD)" ]; then \
	      echo "tag $$T exists on a different commit — bump the version instead"; \
	      exit 1; \
	    fi; \
	    echo "tag $$T already at HEAD — leaving it"; \
	  else \
	    git tag "$$T"; \
	  fi
	git push origin ts/v$(V)
	@echo "pushed ts/v$(V) — watch the release workflow, then confirm provenance:"
	@echo "  npm view @tabnas/support@$(V) dist.attestations"

# Publish the TypeScript package straight from this machine.
#
# The LAST resort. It bypasses OIDC entirely, so the release lands with
# no provenance — which is exactly how 0.1.1 shipped unattested while
# @tabnas/parser's releases carry attestations. Use `make tag-ts` unless
# trusted publishing is genuinely unavailable.
publish-ts: test-ts
	cd ts && npm publish --access public

# Set the release version everywhere it appears: make version V=x.y.z
#
# There are FOUR places, and a release that moves some but not all of them
# leaves the repo failing rather than merely inconsistent — the version
# test in each runtime compares against ts/package.json. That is not
# hypothetical: v0.1.1 shipped with go/support.go still reading 0.1.0,
# which turned `go test ./...` red on main and, because publish-go used to
# run the tests BEFORE bumping, blocked the very target that would have
# fixed it.
version:
	@test -n "$(V)" || (echo "Usage: make version V=x.y.z" && exit 1)
	cd ts && npm version --no-git-tag-version --allow-same-version $(V)
	sed -i.bak "s/^export const VERSION = '.*'/export const VERSION = '$(V)'/" ts/src/support.ts
	sed -i.bak 's/^const VERSION = ".*"/const VERSION = "$(V)"/' go/support.go
	sed -i.bak 's|^\(	github.com/tabnas/support/go \)v.*|\1v$(V)|' go/adder/go.mod
	rm -f ts/src/support.ts.bak go/support.go.bak go/adder/go.mod.bak
	$(MAKE) version-rs V=$(V)
	@echo "version set to $(V) in ts/package.json, ts/src/support.ts, go/support.go, go/adder/go.mod, rs/Cargo.toml, rs/src/lib.rs, rs/adder/Cargo.toml"

# Set the Rust version sites: make version-rs V=x.y.z
#
# Three sites: rs/Cargo.toml, rs/src/lib.rs and rs/adder/Cargo.toml, all
# three held to ts/package.json by rs/tests/version_test.rs. Each crate's
# own entry in its Cargo.lock moves too, which is what the cargo metadata
# runs below are for; that pair is checked by ci/rust/run.sh
# (check_lock_version), not by the version test. Neither commits nor
# tags: neither crate is published. The adder crate cannot be, since it
# takes the engine by path and crates.io refuses a path dependency, and
# it says so with publish = false. The support crate carries no path
# dependency and so is publishable in principle; it has simply never
# been published, and consumers take it as a path dependency on a
# sibling checkout. Only the constants need to stay in step.
version-rs:
	@test -n "$(V)" || (echo "Usage: make version-rs V=x.y.z" && exit 1)
	sed -i.bak 's/^version = ".*"/version = "$(V)"/' rs/Cargo.toml rs/adder/Cargo.toml
	sed -i.bak 's/^pub const VERSION: &str = ".*";/pub const VERSION: \&str = "$(V)";/' rs/src/lib.rs
	rm -f rs/Cargo.toml.bak rs/adder/Cargo.toml.bak rs/src/lib.rs.bak
	cd rs && cargo metadata --format-version 1 --offline >/dev/null
	cd rs/adder && cargo metadata --format-version 1 --offline >/dev/null

# --- Go (module in go/, plus the adder module in go/adder/) ---
build-go:
	cd go && go build ./...
	cd go/adder && go build ./...

test-go:
	cd go && go test ./...

test-go-adder:
	cd go/adder && go test ./...

fmt-go:
	cd go && gofmt -l -w .

vet:
	cd go && go vet ./...
	cd go/adder && go vet ./...

clean-go:
	cd go && go clean
	cd go/adder && go clean

# --- Rust (crate in rs/, plus the adder crate in rs/adder/) ---
build-rs:
	cd rs && cargo build --all-targets
	cd rs/adder && cargo build --all-targets

test-rs:
	cd rs && cargo test --all-targets && cargo test --doc
	cd rs && cargo clippy --all-targets --all-features -- -D warnings
	cd rs/adder && cargo test --all-targets && cargo test --doc
	cd rs/adder && cargo clippy --all-targets --all-features -- -D warnings

clean-rs:
	cd rs && cargo clean
	cd rs/adder && cargo clean

# Publish the Go module: make publish-go V=x.y.z
# Injects V into the Go VERSION const and the go/adder require, commits,
# tags go/vX.Y.Z, and (when gh is available) creates a GitHub release.
#
# Only the support module is tagged. go/adder is a PRIVATE INTERNAL TEST
# MODULE — never tagged, never published, by the maintainer's decision
# (admin#19; tabnas/support#21) — so its require moves with the release,
# keeping it building against this version, and nothing else happens to
# it. Its two old tags, v0.2.0 and v0.3.0, predate the decision and stay:
# a Go tag is immutable once the proxy has served it. Do not add a tag
# for it here or in .github/workflows/release.yml; ts/test/release.test.js
# fails if either describes one.
#
# NOTE: this rewrites the Go side ONLY (go/support.go and the
# go/adder/go.mod require). It does NOT touch
# ts/src/support.ts or ts/package.json — keeping the two runtimes in sync
# is the release orchestrator's job, and the version tests in both
# runtimes fail the build if they ever drift.
#
# Because go/version_test.go checks VERSION against ts/package.json, a Go
# release for a version the TypeScript side has not reached would leave
# the repo failing. The guard below refuses that outright: bump ts/ first
# (or run the orchestrator, which does both), then come back here. The
# check is a hard stop, not a warning — a release that ships a lie about
# its own version is exactly what the version tests exist to prevent.
#
# The tests run AFTER the bump, not as prerequisites. Running them first
# deadlocked this target in practice: v0.1.1 shipped on the TypeScript
# side with go/support.go still reading 0.1.0, so the version test was
# already red — and the prerequisite failed before the sed that would
# have fixed it. Post-bump is also the run that means something, since it
# tests what is about to be tagged.
publish-go:
	@test -n "$(V)" || (echo "Usage: make publish-go V=x.y.z" && exit 1)
	@TS_V=`node -e "console.log(require('./ts/package.json').version)"`; \
	  test "$$TS_V" = "$(V)" || \
	  (echo "ts/package.json is at $$TS_V, not $(V) — bump the TypeScript side first" && exit 1)
	sed -i.bak 's/^const VERSION = ".*"/const VERSION = "$(V)"/' go/support.go
	sed -i.bak 's|^\(	github.com/tabnas/support/go \)v.*|\1v$(V)|' go/adder/go.mod
	rm -f go/support.go.bak go/adder/go.mod.bak
	$(MAKE) test-go test-go-adder
	git add go/support.go go/adder/go.mod
	# Commit only if the bump above actually changed something. When the
	# version is already committed — which is the NORMAL case now that
	# `make version` sets all four sites in one go — both seds are
	# no-ops, `git commit` exits 1 on "nothing to commit", and make would
	# abort before creating the tag. That would fail exactly when the
	# release is otherwise ready.
	git diff --cached --quiet || git commit -m "go: v$(V)"
	# A tag already at HEAD is left alone rather than treated as an
	# error, because a HALF-DONE release is a real state that this target
	# has to be able to finish: a run whose push failed after `git tag`
	# succeeded leaves the tag behind locally, and failing on it would
	# mean the push could never be retried.
	#
	# A tag that exists on a DIFFERENT commit is still a hard stop. That
	# means the version was released and the code has moved since, so the
	# answer is a new version, not a moved tag.
	@T=go/v$(V); \
	  if git rev-parse -q --verify "refs/tags/$$T" >/dev/null; then \
	    if [ "$$(git rev-parse "refs/tags/$$T^{commit}")" != "$$(git rev-parse HEAD)" ]; then \
	      echo "tag $$T exists on a different commit — bump the version instead"; \
	      exit 1; \
	    fi; \
	    echo "tag $$T already at HEAD — leaving it"; \
	  else \
	    git tag "$$T"; \
	  fi
	git push origin main go/v$(V)
	@command -v gh >/dev/null 2>&1 && gh release create go/v$(V) --title "go/v$(V)" --notes "Go module release v$(V)" || true

# List published Go module tags, newest first.
tags-go:
	git tag -l 'go/v*' --sort=-version:refname

reset:
	cd ts && npm run reset
	cd go && go clean -cache && go build ./... && go test ./...
	cd go/adder && go test ./...
	cd rs && cargo clean && cargo test --all-targets
	cd rs/adder && cargo clean && cargo test --all-targets

# The prose gate (see docs/STYLE-GUIDE.md). Vale over the reader-facing
# pages, at the levels set in .vale.ini, on the same file list
# ts/test/docs.test.js reads. Requires `vale` on PATH and one
# `vale sync`. Warnings are advisory, errors fail.
prose:
	vale --minAlertLevel=error $$(node ts/scripts/gated-docs.cjs)
	node ts/scripts/vale-counts.cjs

# Re-measure what .vale.ini and the style guide record, after
# a change to the pages or to the rules moves the numbers.
prose-counts:
	node ts/scripts/vale-counts.cjs --write
