# Makefile for promptline

.PHONY: build install clean test coverage help

# Default target
help:
	@echo "promptline Makefile"
	@echo "Usage:"
	@echo "  make build     - Build the application"
	@echo "  make install   - Install the application globally"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make test      - Run tests"
	@echo "  make test-race - Run tests with race detector"
	@echo "  make coverage  - Run tests with coverage and display report"
	@echo "  make help      - Show this help message"
	@echo "  make prompt    - Show a reusable system prompt"

# Build the application
build:
	go build -o promptline ./cmd/promptline

# Install the application globally
install:
	go install ./cmd/promptline

# Clean build artifacts
clean:
	rm -f promptline

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "For detailed HTML report, run: go tool cover -html=coverage.out"

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

prompt:
	printf "%s\n" \
		system_prompt/0[1-9]*.txt \
	 	| sort -n | xargs cat

# system_prompt/[1-4][0-9]*.txt \
