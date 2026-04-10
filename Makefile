# Use bash as the shell, with environment lookup
SHELL := /usr/bin/env bash

.DEFAULT_GOAL := all

MAKEFLAGS += --no-print-directory --silent

VERSION ?= 0.0.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILT_BY ?= $(shell whoami)
PROJECT_ROOT_DIR := $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Tool binaries
AIR := ./tools/air.sh
BUN := ./tools/bun.sh
GOLANGCI-LINT := ./tools/golangci-lint.sh
NO-DIFF := tools/ensure-no-diff.sh
PREFLIGHT-CHECKS := ./tools/preflight-checks.sh

.PHONY: all # Build everything (default target).
all: build web

.PHONY: help # Print this help message.
help:
	@grep -E '^\.PHONY: [a-zA-Z_-]+ .*?# .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = "(: |#)"}; {printf "%-30s %s\n", $$2, $$3}'

.PHONY: dev # Run the application in development mode.
dev:
	$(MAKE) -j2 server-dev web-dev

.PHONY: lint # Lint all of the code.
lint:
	-$(MAKE) server-lint
	-$(MAKE) web-lint

.PHONY: lint-fix # Lint and fix all of the code.
lint-fix:
	-$(MAKE) server-lint-fix
	-$(MAKE) web-lint-fix

.PHONY: test # Unit test all of the code.
test:
	-$(MAKE) server-test
	-$(MAKE) web-test

.PHONY: verify # Verify all of the code.
verify:
	-$(MAKE) server-verify
	-$(MAKE) web-verify

.PHONY: clean # Remove build and cache artifacts.
clean:
	rm -rf build cmd/assets/generated_assets.go dist node_modules web/build web/node_modules

.PHONY: pigeon # Generate PEG parser
pigeon:
	 pigeon -o internal/querybuilder/parser.go internal/querybuilder/parser.peg

.PHONY: server # Build the unified admiral-server binary.
server: preflight-checks-go
	go build -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -X main.builtBy=$(BUILT_BY)" \
		-o ./build/admiral-server .

.PHONY: server-with-assets # Build the unified binary with web assets.
server-with-assets: preflight-checks-go web
	go run cmd/assets/generate.go ./web/build && go build -tags withAssets \
		-ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -X main.builtBy=$(BUILT_BY)" \
		-o ./build/admiral-server .

.PHONY: server-dev # Start the server in development mode.
server-dev: preflight-checks-go
	$(AIR) server

.PHONY: server-lint # Lint the server code.
server-lint: preflight-checks-go
	$(GOLANGCI-LINT) run --timeout 2m30s

.PHONY: server-lint-fix # Lint and fix the server code.
server-lint-fix: preflight-checks-go
	$(GOLANGCI-LINT) run --fix
	go mod tidy

.PHONY: server-test # Run unit tests for the server code.
server-test: preflight-checks-go
	go test -race -covermode=atomic ./...

.PHONY: server-verify # Verify go modules' requirements files are clean.
server-verify: preflight-checks-go
	go mod tidy
	$(NO-DIFF) server

.PHONY: web # Build production web assets.
web: bun-install
	$(BUN) run --cwd web build

.PHONY: web-dev-build # Build development web assets.
web-dev-build: bun-install
	$(BUN) run --cwd web preview

.PHONY: web-dev # Start the web in development mode.
web-dev: bun-install
	$(BUN) run --cwd web dev

.PHONY: web-lint # Lint the web code.
web-lint: bun-install
	$(BUN) run --cwd web lint

.PHONY: web-lint-fix # Lint and fix the web code.
web-lint-fix: bun-install
	$(BUN) run --cwd web lint:fix

.PHONY: web-test # Run unit tests for the web code.
web-test: bun-install
	$(BUN) run --cwd web test:run

.PHONY: web-verify # Verify web packages are sorted.
web-verify: bun-install
	$(BUN) run --cwd web lint:packages

.PHONY: bun-install # Install web dependencies.
bun-install:
	$(BUN) install --cwd web --frozen-lockfile

.PHONY: preflight-checks-go
preflight-checks-go:
	$(PREFLIGHT-CHECKS) go

.PHONY: preflight-checks
preflight-checks:
	$(PREFLIGHT-CHECKS)

.PHONY: release # Tag and push the next version (auto-detected from commits).
release:
	@VERSION=$$(./tools/svu.sh next) && \
	echo "Current version: $$(./tools/svu.sh current)" && \
	echo "Next version:    $$VERSION" && \
	echo "" && \
	read -p "Proceed? [y/N] " confirm && [ "$$confirm" = "y" ] && \
	git tag -a $$VERSION -m "Release $$VERSION" && \
	git push origin $$VERSION

.PHONY: release-patch # Tag and push a patch release.
release-patch:
	@VERSION=$$(./tools/svu.sh patch) && \
	echo "Current version: $$(./tools/svu.sh current)" && \
	echo "Next version:    $$VERSION" && \
	git tag -a $$VERSION -m "Release $$VERSION" && \
	git push origin $$VERSION

.PHONY: release-minor # Tag and push a minor release.
release-minor:
	@VERSION=$$(./tools/svu.sh minor) && \
	echo "Current version: $$(./tools/svu.sh current)" && \
	echo "Next version:    $$VERSION" && \
	git tag -a $$VERSION -m "Release $$VERSION" && \
	git push origin $$VERSION

.PHONY: release-major # Tag and push a major release.
release-major:
	@VERSION=$$(./tools/svu.sh major) && \
	echo "Current version: $$(./tools/svu.sh current)" && \
	echo "Next version:    $$VERSION" && \
	git tag -a $$VERSION -m "Release $$VERSION" && \
	git push origin $$VERSION

.PHONY: version # Show current and next version.
version:
	@echo "Current: $$(./tools/svu.sh current)"
	@echo "Next:    $$(./tools/svu.sh next)"
