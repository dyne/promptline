# Makefile for promptline

.PHONY: build install clean test coverage help release test-race fmt vet benchmarks prompt

GO ?= go
GOOS ?= $(shell $(GO) env GOOS)
GOARCH ?= $(shell $(GO) env GOARCH)
GOEXE := $(shell GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) env GOEXE)
BINARY := promptline$(GOEXE)

# Default target
help:
	@echo "promptline Makefile"
	@echo "Usage:"
	@echo "  make build     - Build the application"
	@echo "  make release   - Build a release binary"
	@echo "  make install   - Install the application globally"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make test      - Run tests"
	@echo "  make test-race - Run tests with race detector"
	@echo "  make coverage  - Run tests with coverage and display report"
	@echo "  make help      - Show this help message"
	@echo "  make prompt    - Show a reusable system prompt"

# Build the application
build:
	$(GO) build -o $(BINARY) ./cmd/promptline

release:
	$(GO) build -trimpath -ldflags "-s -w" -o $(BINARY) ./cmd/promptline

# Install the application globally
install:
	$(GO) install ./cmd/promptline

# Clean build artifacts
clean:
	rm -f promptline promptline.exe

# Run tests
test:
	$(GO) test ./...

# Run tests with race detector
test-race:
	$(GO) test -race ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GO) test -coverprofile=coverage.out ./...
	@echo ""
	@echo "=== Coverage Summary ==="
	@$(GO) tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "For detailed HTML report, run: go tool cover -html=coverage.out"

benchmarks:
	$(info Running tool benchmarks...)
	$(GO) test -run '^$$' -bench BenchmarkURoot -benchmem ./internal/tools

# Format code
fmt:
	$(GO) fmt ./...

# Vet code
vet:
	$(GO) vet ./...

# All system prompts are in sorting order, from 01 to 49 are reusable on
# any LLM prompt, from 50 up they are specific to promptline.
# To obtain a reusable prompts just do 'make prompt' in parent dir.
prompt:
	@printf "%s\n" \
		system_prompt/0[1-9]*.txt \
	 	| sort -n | xargs cat

# system_prompt/[1-4][0-9]*.txt \
