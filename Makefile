# Build and test both the TypeScript (ts/) and Go (go/) implementations.
# ts/ is canonical; go/ tracks it.
#
# go/adder is a SEPARATE module — the support module itself has no
# dependencies, and the grammar that exercises it needs the parser — so
# `go test ./...` in go/ does not reach it and it is run explicitly here.

.PHONY: all build test clean build-ts build-go test-ts test-go test-go-adder \
        clean-ts clean-go publish-ts publish-go tags-go reset fmt-go vet

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
publish-ts: test-ts
	cd ts && npm publish --access public

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
publish-go: test-go test-go-adder
	@test -n "$(V)" || (echo "Usage: make publish-go V=x.y.z" && exit 1)
	@TS_V=`node -e "console.log(require('./ts/package.json').version)"`; \
	  test "$$TS_V" = "$(V)" || \
	  (echo "ts/package.json is at $$TS_V, not $(V) — bump the TypeScript side first" && exit 1)
	sed -i.bak 's/^const VERSION = ".*"/const VERSION = "$(V)"/' go/support.go
	sed -i.bak 's|^\(	github.com/tabnas/support/go \)v.*|\1v$(V)|' go/adder/go.mod
	rm -f go/support.go.bak go/adder/go.mod.bak
	$(MAKE) test-go test-go-adder
	git add go/support.go go/adder/go.mod
	git commit -m "go: v$(V)"
	# Both modules are tagged. go/adder is a nested module, so Go tooling
	# can only discover its releases under go/adder/vX.Y.Z — tagging go/
	# alone would leave the documented adder package unresolvable.
	git tag go/v$(V)
	git tag go/adder/v$(V)
	git push origin main go/v$(V) go/adder/v$(V)
	@command -v gh >/dev/null 2>&1 && gh release create go/v$(V) --title "go/v$(V)" --notes "Go module release v$(V)" || true

# List published Go module tags, newest first.
tags-go:
	git tag -l 'go/v*' --sort=-version:refname

reset:
	cd ts && npm run reset
	cd go && go clean -cache && go build ./... && go test ./...
	cd go/adder && go test ./...
