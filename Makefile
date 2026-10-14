# Local gate for zero-bridge. Future CI calls `make check` and does not grow a second list.
# Pinned tools run with `go run pkg@version`. Nothing is installed globally.
# Versions follow the Zero upstream Makefile (read 2026-10-14).
# GOTOOLCHAIN=go1.26.9 may download that toolchain from the network the first
# time an analyzer target runs. Offline: warm the cache once, with network,
# via `make lint-static`. vet, build, and test stay on the local Go.
DEADCODE_VERSION := v0.46.0
GOLANGCI_LINT_VERSION := v2.12.2
GOVULNCHECK_VERSION := v1.3.0

ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BRIDGE := $(ROOT)/bridge
GO_VERSION := $(shell awk '/^go /{print $$2; exit}' $(BRIDGE)/go.mod)
# go.mod says "go 1.22", a language version, not a toolchain (go1.22.x).
# The pinned staticcheck inside golangci-lint v2.12.2 panics on Go 1.27's
# std package poll. Analyzers use Go 1.26.9, the Zero upstream/main toolchain
# these tool pins were read against.
TOOL_GO_TOOLCHAIN := go1.26.9
GO_FILES := $(shell git -C $(ROOT) ls-files 'bridge/*.go' 'bridge/**/*.go')

.DEFAULT_GOAL := help
.PHONY: help fmt fmt-check vet build test test-quick lint-static deadcode vulncheck tidy-check diff-check hygiene hooks e2e mutate check check-advisory

lint-static deadcode vulncheck: export GOTOOLCHAIN = $(TOOL_GO_TOOLCHAIN)
lint-static deadcode vulncheck: export GOWORK = off

help:
	@echo "Pinned: deadcode $(DEADCODE_VERSION), golangci-lint $(GOLANGCI_LINT_VERSION), govulncheck $(GOVULNCHECK_VERSION); analyzer toolchain $(TOOL_GO_TOOLCHAIN); go.mod language $(GO_VERSION)"
	@echo "Targets: fmt, fmt-check, vet, build, test, test-quick, lint-static, deadcode, vulncheck, tidy-check, diff-check, hygiene, hooks, e2e, mutate, check, check-advisory"
	@echo "hooks installs .githooks for this clone. A fresh clone does not do that. Skipping the hooks is forbidden."
	@echo "e2e starts its own bridge on a free port. mutate is the slow pre-PR layer, not pre-commit."
	@echo "check-advisory is deadcode only. It is not part of check: -test=false reports test-only helpers."

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

# Go 1.23+: -diff exits non-zero when tidy would edit, and it does not write.
tidy-check:
	cd $(BRIDGE) && go mod tidy -diff

diff-check:
	git -C $(ROOT) diff HEAD --check

hygiene:
	bash $(ROOT)/scripts/check-hygiene-selftest.sh
	bash $(ROOT)/scripts/check-hygiene.sh --root $(ROOT)

# Not part of check or pre-commit. Needs Node >= 22 and /usr/bin/google-chrome,
# or E2E_OPTIONAL=1 to skip when those are missing.
e2e:
	bash $(ROOT)/scripts/e2e.sh

# Slow pre-PR layer. Not part of check or pre-commit.
mutate:
	bash $(ROOT)/scripts/mutate-auth.sh

# Local config only. Does not set user.name or user.email.
hooks:
	git -C $(ROOT) config core.hooksPath .githooks
	@test "$$(git -C $(ROOT) config --get core.hooksPath)" = ".githooks"
	bash $(ROOT)/scripts/hooks-selftest.sh

check: fmt-check vet build test lint-static vulncheck tidy-check hygiene diff-check

check-advisory: deadcode
