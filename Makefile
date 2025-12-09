# Makefile for batchat

.PHONY: build install clean test help

# Default target
help:
	@echo "batchat Makefile"
	@echo "Usage:"
	@echo "  make build     - Build the application"
	@echo "  make install   - Install the application globally"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make test      - Run tests (if any)"
	@echo "  make help      - Show this help message"

# Build the application
build:
	go build -o batchat ./cmd/batchat

# Install the application globally
install:
	go install ./cmd/batchat

# Clean build artifacts
clean:
	rm -f batchat

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...