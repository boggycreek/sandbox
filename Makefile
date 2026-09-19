# Agent Sandbox — Root Makefile

SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

# Go build settings
GO ?= go
GOFLAGS ?=
COVERAGE_THRESHOLD := 90.0

BIN_DIR := bin
DIST_DIR := dist
COVERAGE_DIR := coverage
COVERAGE_PROFILE := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML := $(COVERAGE_DIR)/coverage.html

.PHONY: all help dev-setup check test test-coverage test-install lint lint-go lint-shell sca vulncheck gosec deadcode deadcode-diff deadcode-all format clean clean-test-env clean-all build build-cli build-libbp build-images build-image-base build-image-opencode build-image-claude build-image-agy build-image-egress

all: check build

# --- Help Target ---

help: ## Show available Makefile targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# --- Development & Setup ---

dev-setup: ## Run developer workstation configuration and dependency checks
	@./dev-setup.sh

# --- Quality Gates: Linting & Static Analysis ---

lint: lint-go lint-shell ## Run all Go and shell linter checks

lint-go: ## Run golangci-lint on Go code
	@echo "==> Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "Warning: golangci-lint not installed. Run ./dev-setup.sh to install."; \
	fi

lint-shell: ## Run shellcheck on bash scripts
	@echo "==> Running shellcheck..."
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck install.sh dev-setup.sh scripts/clean-test-env.sh scripts/deadcode-check.sh; \
	else \
		echo "Warning: shellcheck not installed. Run ./dev-setup.sh to install."; \
	fi

format: ## Auto-format Go code and scripts
	@echo "==> Formatting code..."
	@if command -v gofmt >/dev/null 2>&1; then \
		gofmt -s -w .; \
	fi

# --- Quality Gates: Testing & Coverage ---

test: ## Run unit tests with race detection
	@echo "==> Running Go unit tests..."
	@if [ -f go.mod ]; then \
		$(GO) test -race -v ./...; \
	else \
		echo "Notice: go.mod not yet initialized. Skipping test run."; \
	fi

test-coverage: ## Run tests and enforce >90% code coverage threshold
	@echo "==> Running unit tests with coverage analysis..."
	@mkdir -p $(COVERAGE_DIR)
	@if [ -f go.mod ]; then \
		$(GO) test -race -covermode=atomic -coverprofile=$(COVERAGE_PROFILE) ./pkg/... ./cmd/...; \
		$(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML); \
		TOTAL_COV=$$($(GO) tool cover -func=$(COVERAGE_PROFILE) | grep total: | awk '{print substr($$3, 1, length($$3)-1)}'); \
		echo "==> Total Test Coverage: $${TOTAL_COV}% (Required: >= $(COVERAGE_THRESHOLD)%)"; \
		COVERAGE_PASS=$$(echo "$${TOTAL_COV} >= $(COVERAGE_THRESHOLD)" | bc -l 2>/dev/null || awk -v t="$${TOTAL_COV}" -v req="$(COVERAGE_THRESHOLD)" 'BEGIN {print (t >= req) ? 1 : 0}'); \
		if [ "$${COVERAGE_PASS}" -ne 1 ]; then \
			echo "ERROR: Test coverage $${TOTAL_COV}% is below the required threshold of $(COVERAGE_THRESHOLD)%!" >&2; \
			exit 1; \
		fi; \
		echo "==> Coverage check PASSED."; \
	else \
		echo "Notice: go.mod not yet initialized. Skipping coverage check."; \
	fi

test-install: ## Run containerized installation and bootstrap smoke test in isolated Podman container
	@echo "==> Running containerized installation smoke test..."
	@./test/container-install/test-install.sh

# --- Security & Software Composition Analysis (SCA) ---

sca: vulncheck gosec ## Run all SCA and security vulnerability scanners

vulncheck: ## Run govulncheck on Go dependencies
	@echo "==> Running govulncheck (Software Composition Analysis)..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		if [ -f go.mod ]; then \
			govulncheck ./...; \
		fi; \
	else \
		echo "Notice: govulncheck not installed. Install via: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

gosec: ## Run gosec static security analysis
	@echo "==> Running gosec security analyzer..."
	@if command -v gosec >/dev/null 2>&1; then \
		if [ -f go.mod ]; then \
			gosec -quiet ./...; \
		fi; \
	else \
		echo "Notice: gosec not installed. Install via: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

deadcode: deadcode-diff ## Run deadcode analysis on staged/local changes (alias to deadcode-diff)

deadcode-diff: ## Check for dead code introduced or orphaned by staged/local changes
	@./scripts/deadcode-check.sh --diff

deadcode-all: ## Run comprehensive whole-codebase dead code audit (accumulated debt)
	@./scripts/deadcode-check.sh --all

check: lint test-coverage sca ## Complete quality gate: lint + coverage (>90%) + SCA security

# --- Build Targets ---

build: build-cli build-libbp ## Build all CLI binaries and libraries

build-cli: ## Build native Go CLI binaries (sndbx, bp, bpd, retention-sweep)
	@echo "==> Building CLI binaries..."
	@mkdir -p $(BIN_DIR)
	@if [ -f go.mod ]; then \
		$(GO) build $(GOFLAGS) -o $(BIN_DIR)/sndbx ./cmd/sndbx; \
		$(GO) build $(GOFLAGS) -o $(BIN_DIR)/bp ./cmd/bp; \
		$(GO) build $(GOFLAGS) -o $(BIN_DIR)/bpd ./cmd/bpd; \
		if [ -d ./cmd/retention-sweep ]; then \
			$(GO) build $(GOFLAGS) -o $(BIN_DIR)/retention-sweep ./cmd/retention-sweep; \
		fi; \
		echo "Binaries built in $(BIN_DIR)/"; \
	else \
		echo "Notice: go.mod not yet initialized. Skipping build."; \
	fi

build-libbp: ## Build C-shared library (libbp.dylib / libbp.so)
	@echo "==> Building C-shared libbp library..."
	@mkdir -p $(DIST_DIR)/lib $(DIST_DIR)/include
	@if [ -f go.mod ] && [ -d cmd/libbp-c ]; then \
		$(GO) build -buildmode=c-shared -o $(DIST_DIR)/lib/libbp.so ./cmd/libbp-c; \
		mv $(DIST_DIR)/lib/libbp.h $(DIST_DIR)/include/ 2>/dev/null || true; \
		echo "libbp shared library built in $(DIST_DIR)/"; \
	fi

# --- OCI Image Build Targets (Podman) ---

build-images: build-image-base build-image-opencode build-image-claude build-image-agy build-image-egress ## Build all OCI images (base + derivatives + egress filter)

build-image-base: ## Build neutral agent-sandbox-base OCI image with Podman
	@echo "==> Building agent-sandbox-base OCI image..."
	podman build -t agent-sandbox-base:latest -f images/base/Dockerfile .

build-image-opencode: build-image-base ## Build OpenCode derivative agent OCI image with Podman
	@echo "==> Building agent-sandbox-opencode OCI image..."
	podman build -t agent-sandbox-opencode:latest -f images/agents/opencode/Dockerfile .

build-image-claude: build-image-base ## Build Claude Code derivative agent OCI image with Podman
	@echo "==> Building agent-sandbox-claude OCI image..."
	podman build -t agent-sandbox-claude:latest -f images/agents/claude/Dockerfile .

build-image-agy: build-image-base ## Build Antigravity (agy) derivative agent OCI image with Podman
	@echo "==> Building agent-sandbox-agy OCI image..."
	podman build -t agent-sandbox-agy:latest -f images/agents/agy/Dockerfile .

build-image-egress: ## Build agent-sandbox-egress OCI image with Podman
	@echo "==> Building agent-sandbox-egress OCI image..."
	podman build -t agent-sandbox-egress:latest -f images/egress-filter/Dockerfile .

clean: ## Clean build and test coverage artifacts
	@rm -rf $(BIN_DIR) $(DIST_DIR) $(COVERAGE_DIR)

clean-test-env: ## Clean stale test containers, networks, and orphaned test processes
	@./scripts/clean-test-env.sh

clean-all: clean clean-test-env ## Clean all artifacts and stale test runtime state
