# Build and test both the TypeScript (ts/) and Go (go/) implementations.
# ts/ is canonical; go/ tracks it.
#
# go/adder is a SEPARATE module — the support module itself has no
# dependencies, and the grammar that exercises it needs the parser — so
# `go test ./...` in go/ does not reach it and it is run explicitly here.

.PHONY: all build test clean build-ts build-go test-ts test-go test-go-adder \
        clean-ts clean-go publish-ts publish-go tags-go reset fmt-go vet \
        version

all: build test

build: build-ts build-go

test: test-ts test-go test-go-adder

clean: clean-ts clean-go

# --- TypeScript (package in ts/) ---
build-ts:
	cd ts && npm run build

test-ts:
	cd ts && npm test

clean-ts:
	rm -rf ts/dist

# Publish the TypeScript package at its current package.json version.
#
# Normally you do NOT run this: pushing a `ts/vX.Y.Z` tag makes CI publish
# via OIDC trusted publishing (ci/workflows/release.yml), with no token
# anywhere. This target is the manual fallback for when that path is
# unavailable, and it needs credentials this repo deliberately does not
# carry.
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
	@echo "version set to $(V) in ts/package.json, ts/src/support.ts, go/support.go, go/adder/go.mod"

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

# Publish the Go modules: make publish-go V=x.y.z
# Injects V into the Go VERSION const, commits, tags BOTH modules, and
# (when gh is available) creates a GitHub release.
#
# NOTE: this rewrites go/support.go ONLY. It does NOT touch
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
	# abort before creating either tag. That would fail exactly when the
	# release is otherwise ready.
	git diff --cached --quiet || git commit -m "go: v$(V)"
	# Both modules are tagged. go/adder is a nested module, so Go tooling
	# can only discover its releases under go/adder/vX.Y.Z — tagging go/
	# alone would leave the documented adder package unresolvable.
	#
	# A tag already at HEAD is left alone rather than treated as an
	# error, because a HALF-DONE release is a real state that this target
	# has to be able to finish: 0.1.1 shipped with go/v0.1.1 tagged and
	# go/adder/v0.1.1 missing, and `git tag` failing on the first of the
	# two meant the second could never be created.
	#
	# A tag that exists on a DIFFERENT commit is still a hard stop. That
	# means the version was released and the code has moved since, so the
	# answer is a new version, not a moved tag.
	@for T in go/v$(V) go/adder/v$(V); do \
	  if git rev-parse -q --verify "refs/tags/$$T" >/dev/null; then \
	    if [ "$$(git rev-parse "refs/tags/$$T^{commit}")" != "$$(git rev-parse HEAD)" ]; then \
	      echo "tag $$T exists on a different commit — bump the version instead"; \
	      exit 1; \
	    fi; \
	    echo "tag $$T already at HEAD — leaving it"; \
	  else \
	    git tag "$$T"; \
	  fi; \
	done
	git push origin main go/v$(V) go/adder/v$(V)
	@command -v gh >/dev/null 2>&1 && gh release create go/v$(V) --title "go/v$(V)" --notes "Go module release v$(V)" || true

# List published Go module tags, newest first.
tags-go:
	git tag -l 'go/v*' --sort=-version:refname

reset:
	cd ts && npm run reset
	cd go && go clean -cache && go build ./... && go test ./...
	cd go/adder && go test ./...
