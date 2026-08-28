# Makefile for promptline

.PHONY: build install clean test test-unit test-protocol test-integration test-race test-race-integration test-stress test-fuzz-smoke coverage check-coverage help release fmt vet security-gates benchmarks build-linux build-darwin build-windows test-all check-go-policy vulncheck check-workflow-policy release-dry-run

# Use the Go executable supplied by the environment (including GitHub Actions).
# Local contributors may override this, for example GO=/usr/local/go/bin/go.
GO ?= go
GOCACHE ?= $(CURDIR)/.gocache
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
GOEXE := $(shell GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) env GOEXE)
BINARY := promptline$(GOEXE)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
# Keep the vulnerability scan to this module's tracked package roots. This
# deliberately excludes user-owned untracked trees such as plugins/upstream/.
GOVULNCHECK_PACKAGES := ./cmd/promptline ./internal/... ./plugins/promptline/skills
GOVULNCHECK ?= $(CURDIR)/.bin/govulncheck
GOSEC ?= $(CURDIR)/.bin/gosec
# Do not let untracked third-party material under plugins/upstream influence
# local verification of this module. CI starts from a clean checkout, while
# contributor worktrees may intentionally retain upstream reference trees.
GO_PACKAGES := ./cmd/promptline ./internal/... ./plugins/promptline/skills

# Default target
help:
	@echo "promptline Makefile"
	@echo "Usage:"
	@echo "  make build     - Build the application (version: $(VERSION))"
	@echo "  make release   - Build a release binary with version $(VERSION)"
	@echo "  make install   - Install the application globally"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make test-unit             - Run the fast unit suite"
	@echo "  make test-protocol         - Run app-server and MCP protocol contracts"
	@echo "  make test-integration      - Run mock app-server and toolbox integration tests"
	@echo "  make test-race-integration - Run integration tests with the race detector"
	@echo "  make test-stress           - Repeat concurrency-sensitive package tests"
	@echo "  make test-fuzz-smoke       - Run bounded parser and path fuzz smoke tests"
	@echo "  make check-coverage        - Enforce behavior-package coverage floors"
	@echo "  make vet / benchmarks      - Run static checks or toolbox benchmark smoke"
	@echo "  make build-linux|darwin|windows - Cross-compile the command"
	@echo "  make test-all              - Run the complete local release gate"
	@echo "  make help      - Show this help message"
	@echo ""
	@echo "Version can be set via VERSION variable: make VERSION=v1.0.0 release"

# Build the application
build:
	$(GO) build -o $(BINARY) ./cmd/promptline

release:
	$(GO) build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BINARY) ./cmd/promptline

check-go-policy:
	./scripts/check-go-policy.sh

vulncheck:
	@mkdir -p "$(dir $(GOVULNCHECK))"
	GOBIN="$(dir $(GOVULNCHECK))" $(GO) install golang.org/x/vuln/cmd/govulncheck@v1.7.0
	PATH="$(dir $(GO)):$$PATH" $(GOVULNCHECK) $(GOVULNCHECK_PACKAGES)

check-workflow-policy:
	python3 ./scripts/check-workflow-policy.py .github/workflows/ci.yml .releaserc

security-gates:
	python3 ./scripts/security-gates.py
	python3 ./scripts/security-gates.py --self-test
	python3 ./scripts/check-gosec-policy.py
	python3 ./scripts/check-gosec-policy.py --self-test
	GOCACHE="$(GOCACHE)" $(GO) test ./plugins/promptline/skills -run '^TestEmbeddedCatalog'
	GOBIN="$(dir $(GOSEC))" $(GO) install github.com/securego/gosec/v2/cmd/gosec@v2.22.4
	PATH="$(dir $(GO)):$$PATH" python3 ./scripts/run-gosec.py $(GOSEC) ./cmd/promptline ./internal/... ./plugins/promptline/skills

release-dry-run: check-go-policy
	GO="$(GO)" ./scripts/release-dry-run.sh

# Install the application globally
install:
	$(GO) install ./cmd/promptline

# Clean build artifacts
clean:
	rm -f promptline promptline.exe

# Run tests
test:
	$(MAKE) test-unit

test-unit:
	GOCACHE="$(GOCACHE)" $(GO) test $(GO_PACKAGES)

test-protocol:
	GOCACHE="$(GOCACHE)" $(GO) test ./internal/appserver ./internal/mcp

test-integration:
	GOCACHE="$(GOCACHE)" $(GO) test -tags=integration $(GO_PACKAGES)

# Run tests with race detector
test-race:
	GOCACHE="$(GOCACHE)" $(GO) test -race $(GO_PACKAGES)

test-race-integration:
	GOCACHE="$(GOCACHE)" $(GO) test -race -tags=integration $(GO_PACKAGES)

test-stress:
	GOCACHE="$(GOCACHE)" $(GO) test -race -count=20 ./internal/appserver ./internal/governance ./internal/instance ./internal/mcp ./internal/paths ./internal/runtime ./internal/tools

test-fuzz-smoke:
	GOCACHE="$(GOCACHE)" $(GO) test -run '^$$' -fuzz=FuzzClientDispatch -fuzztime=5s ./internal/appserver
	GOCACHE="$(GOCACHE)" $(GO) test -run '^$$' -fuzz=FuzzPathConfinement -fuzztime=5s ./internal/paths
	GOCACHE="$(GOCACHE)" $(GO) test -run '^$$' -fuzz=FuzzServerRejectsMalformedRequests -fuzztime=5s ./internal/mcp

# Run tests with coverage
coverage:
	$(MAKE) check-coverage

check-coverage:
	GO="$(GO)" GOCACHE="$(GOCACHE)" ./scripts/check-coverage.sh

benchmarks:
	$(info Running tool benchmarks...)
	GOCACHE="$(GOCACHE)" $(GO) test -run '^$$' -bench BenchmarkURoot -benchmem ./internal/tools

# Format code
fmt:
	GOCACHE="$(GOCACHE)" $(GO) fmt $(GO_PACKAGES)

# Vet code
vet:
	GOCACHE="$(GOCACHE)" $(GO) vet $(GO_PACKAGES)

build-linux:
	GOCACHE="$(GOCACHE)" GOOS=linux GOARCH=amd64 $(GO) build -o /tmp/promptline-linux-amd64 ./cmd/promptline

build-darwin:
	GOCACHE="$(GOCACHE)" GOOS=darwin GOARCH=amd64 $(GO) build -o /tmp/promptline-darwin-amd64 ./cmd/promptline

build-windows:
	GOCACHE="$(GOCACHE)" GOOS=windows GOARCH=amd64 $(GO) build -o /tmp/promptline-windows-amd64.exe ./cmd/promptline

test-all: check-go-policy vulncheck check-workflow-policy security-gates test-unit test-protocol test-integration test-race-integration test-stress test-fuzz-smoke check-coverage vet benchmarks build-linux build-darwin build-windows release-dry-run
