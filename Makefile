# dhcp-go build targets. The web UI is embedded, so the binary is
# self-contained.

BINARY := dhcp-go
PKG    := ./cmd/dhcp-go
GO     ?= go

.PHONY: build test deps check-deps clean

build:
	CGO_ENABLED=0 $(GO) build -trimpath -o dist/$(BINARY) $(PKG)

test:
	$(GO) vet ./...
	$(GO) test -race ./...

# The web UI's browser dependencies come from npm (web/ui/package.json).
# node_modules there is committed, exactly as npm ci leaves it, so the Go
# module embeds it and `go get` delivers it. After changing package.json,
# run `make deps` and commit the result; check-deps fails when the
# committed files differ from the lockfile.
deps:
	npm install --prefix web/ui --no-audit --no-fund

check-deps:
	npm ci --prefix web/ui --no-audit --no-fund
	git diff --exit-code --stat -- web/ui/node_modules
	@test -z "$$(git status --porcelain -- web/ui/node_modules)" || (git status --short -- web/ui/node_modules; exit 1)

clean:
	rm -rf dist
