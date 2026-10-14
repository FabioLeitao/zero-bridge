# Local gate for zero-bridge. Future CI calls `make check` and does not grow a second list.
# Pinned tools run with `go run pkg@version`. Nothing is installed globally.
# Versions follow the Zero upstream Makefile (read 2026-10-14).
DEADCODE_VERSION := v0.46.0
GOLANGCI_LINT_VERSION := v2.12.2
GOVULNCHECK_VERSION := v1.3.0

ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BRIDGE := $(ROOT)/bridge
GO_VERSION := $(shell awk '/^go /{print $$2; exit}' $(BRIDGE)/go.mod)
# go.mod says "go 1.22", a language version, not a toolchain (go1.22.x).
# The pinned staticcheck inside golangci-lint v2.12.2 panics on Go 1.27's
# std package poll. Analyzers use Go 1.26.9, the Zero upstream/main toolchain
# these tool pins were read against. vet, build, and test stay on the local Go.
TOOL_GO_TOOLCHAIN := go1.26.9
GO_FILES := $(shell git -C $(ROOT) ls-files 'bridge/*.go' 'bridge/**/*.go')

.DEFAULT_GOAL := help
.PHONY: help fmt fmt-check vet build test test-quick lint-static deadcode vulncheck tidy-check diff-check check

lint-static deadcode vulncheck: export GOTOOLCHAIN = $(TOOL_GO_TOOLCHAIN)
lint-static deadcode vulncheck: export GOWORK = off

help:
	@echo "Pinned: deadcode $(DEADCODE_VERSION), golangci-lint $(GOLANGCI_LINT_VERSION), govulncheck $(GOVULNCHECK_VERSION); analyzer toolchain $(TOOL_GO_TOOLCHAIN); go.mod language $(GO_VERSION)"
	@echo "Targets: fmt, fmt-check, vet, build, test, test-quick, lint-static, deadcode, vulncheck, tidy-check, diff-check, check"

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@out="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	cd $(BRIDGE) && go vet ./...

build:
	cd $(BRIDGE) && go build ./...

test:
	umask 022 && cd $(BRIDGE) && go test ./... -race -count=1 -timeout 200s

test-quick:
	cd $(BRIDGE) && go test ./... -count=1

lint-static:
	cd $(BRIDGE) && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --enable-only unused,ineffassign,staticcheck ./...

deadcode:
	cd $(BRIDGE) && go run golang.org/x/tools/cmd/deadcode@$(DEADCODE_VERSION) -test=false ./...

vulncheck:
	cd $(BRIDGE) && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

tidy-check:
	@cd $(BRIDGE) && \
	cp go.mod go.mod.tidycheck && \
	if [ -f go.sum ]; then cp go.sum go.sum.tidycheck; else : > go.sum.tidycheck; fi && \
	go mod tidy && \
	status=0; \
	cmp -s go.mod go.mod.tidycheck || status=1; \
	if [ -f go.sum ]; then cmp -s go.sum go.sum.tidycheck || status=1; fi; \
	mv go.mod.tidycheck go.mod; \
	if [ -s go.sum.tidycheck ]; then mv go.sum.tidycheck go.sum; else rm -f go.sum go.sum.tidycheck; fi; \
	if [ $$status -ne 0 ]; then echo "go.mod is not tidy"; exit 1; fi

diff-check:
	git -C $(ROOT) diff HEAD --check

check: fmt-check vet build test lint-static vulncheck diff-check
