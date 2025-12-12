# Makefile for promptline

.PHONY: build install clean test help

# Default target
help:
	@echo "promptline Makefile"
	@echo "Usage:"
	@echo "  make build     - Build the application"
	@echo "  make install   - Install the application globally"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make test      - Run tests"
	@echo "  make test-race - Run tests with race detector"
	@echo "  make help      - Show this help message"

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

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...
