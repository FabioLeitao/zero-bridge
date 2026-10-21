# Local gate for zero-bridge. Future CI calls `make check` and does not grow a second list.
# Pinned tools run with `go run pkg@version`. Nothing is installed globally.
# Versions follow the Zero upstream Makefile (read 2026-10-14).
# bridge/go.mod pins toolchain go1.27.3. The first vet, build, test, or
# analyzer run on an older host may download that toolchain (about 250 MB).
# golangci-lint v2.15.0 ships staticcheck v0.8.2 (2026-10-12). v2.12.2
# panicked in the Go 1.27 poll package, so the analyzers used a second toolchain.
DEADCODE_VERSION := v0.46.0
GOLANGCI_LINT_VERSION := v2.15.0
GOVULNCHECK_VERSION := v1.3.0

ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BRIDGE := $(ROOT)/bridge
GO_VERSION := $(shell awk '/^go /{print $$2; exit}' $(BRIDGE)/go.mod)
# go.mod says "go 1.22", a language version, not a toolchain (go1.22.x).
GO_FILES := $(shell git -C $(ROOT) ls-files 'bridge/*.go' 'bridge/**/*.go')

.DEFAULT_GOAL := help
.PHONY: help fmt fmt-check vet build test test-quick lint-static deadcode vulncheck vulncheck-binary tidy-check diff-check hygiene hooks e2e mutate sbom check check-advisory

lint-static deadcode vulncheck: export GOWORK = off

help:
	@echo "Pinned: deadcode $(DEADCODE_VERSION), golangci-lint $(GOLANGCI_LINT_VERSION), govulncheck $(GOVULNCHECK_VERSION); analyzers use the bridge/go.mod toolchain; go.mod language $(GO_VERSION)"
	@echo "Targets: fmt, fmt-check, vet, build, test, test-quick, lint-static, deadcode, vulncheck, vulncheck-binary, tidy-check, diff-check, hygiene, hooks, e2e, mutate, sbom, check, check-advisory"
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
	cd $(BRIDGE)/obs && go vet ./...

build:
	cd $(BRIDGE) && go build ./...

test:
	umask 022 && cd $(BRIDGE) && go test ./... -race -count=1 -timeout 200s
	umask 022 && cd $(BRIDGE)/obs && go test ./... -count=1 -timeout 60s

test-quick:
	cd $(BRIDGE) && go test ./... -count=1

lint-static:
	cd $(BRIDGE) && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --enable-only unused,ineffassign,staticcheck ./...

deadcode:
	cd $(BRIDGE) && go run golang.org/x/tools/cmd/deadcode@$(DEADCODE_VERSION) -test=false ./...

vulncheck: vulncheck-binary
	cd $(BRIDGE) && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	cd $(BRIDGE)/obs && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# Build and scan with the go.mod toolchain. The temp binary is removed
# when the recipe exits.
vulncheck-binary:
	dir=$$(mktemp -d); \
	trap 'rm -rf "$$dir"' EXIT; \
	cd $(BRIDGE) && GOTOOLCHAIN=auto go build -o "$$dir/zero-bridge" ./cmd/zero-bridge && \
	GOTOOLCHAIN=auto go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) -mode=binary "$$dir/zero-bridge"

# Go 1.23+: -diff exits non-zero when tidy would edit, and it does not write.
tidy-check:
	cd $(BRIDGE) && go mod tidy -diff
	cd $(BRIDGE)/obs && go mod tidy -diff

diff-check:
	git -C $(ROOT) diff HEAD --check

hygiene:
	bash $(ROOT)/scripts/check-hygiene-selftest.sh
	bash $(ROOT)/scripts/check-hygiene.sh --root $(ROOT)

# Not part of check or pre-commit. Needs Node >= 22 and /usr/bin/google-chrome,
# or E2E_OPTIONAL=1 to skip when those are missing.
e2e:
	bash $(ROOT)/scripts/e2e.sh

# Not part of check. Writes bridge/zero-bridge.cdx.json.
sbom:
	python3 $(ROOT)/scripts/sbom.py --root $(ROOT) --out $(ROOT)/bridge/zero-bridge.cdx.json

# Slow pre-PR layer. Not part of check or pre-commit.
mutate:
	bash $(ROOT)/scripts/mutate-auth.sh

# Local config only. Does not set user.name or user.email.
hooks:
	git -C $(ROOT) config core.hooksPath .githooks
	@test "$$(git -C $(ROOT) config --get core.hooksPath)" = ".githooks"
	bash $(ROOT)/scripts/hooks-selftest.sh

# fmt-check, vet, and build stay in front so a compile error does not wait
# for the race tests. The next five do not share worktree writes: test, lint,
# and vulncheck use the build cache, tidy-check is go mod tidy -diff, and
# hygiene uses a temp directory. Logs are printed in this fixed order.
# stdbuf keeps line buffering, so a tool's stdout and stderr stay in the
# order the program wrote them when the log is a file.
# diff-check is last, so a tool that wrote the tree fails the gate.
check:
	@$(MAKE) --no-print-directory fmt-check
	@$(MAKE) --no-print-directory vet
	@$(MAKE) --no-print-directory build
	@set -eu; \
	logs=$$(mktemp -d); \
	trap 'rm -rf "$$logs"' EXIT; \
	start() { \
		name=$$1; shift; \
		stdbuf -oL -eL "$$@" >"$$logs/$$name.out" 2>&1 & \
		echo $$! >"$$logs/$$name.pid"; \
	}; \
	start test $(MAKE) --no-print-directory test; \
	start lint $(MAKE) --no-print-directory lint-static; \
	start vuln $(MAKE) --no-print-directory vulncheck; \
	start tidy $(MAKE) --no-print-directory tidy-check; \
	start hygiene $(MAKE) --no-print-directory hygiene; \
	fail=0; \
	for name in test lint vuln tidy hygiene; do \
		if wait "$$(cat "$$logs/$$name.pid")"; then \
			st=0; \
		else \
			st=$$?; \
		fi; \
		cat "$$logs/$$name.out"; \
		if [ "$$st" -ne 0 ]; then fail=$$st; fi; \
	done; \
	$(MAKE) --no-print-directory diff-check; \
	exit "$$fail"

check-advisory: deadcode
