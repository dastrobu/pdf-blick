.PHONY: build clean test fmt vet deps tidy check install-hooks doctoc help

# Binary name
BINARY=pdf-blick

# Build the binary
build:
	go build -o $(BINARY) .

# Clean build artifacts
clean:
	rm -f $(BINARY)
	go clean

# Run Go tests
test:
	go test -v -count=1 ./...

# Format Go code
fmt:
	gofmt -w .

# Run go vet
vet:
	go vet ./...

# Download and verify dependencies
deps:
	go mod download
	go mod verify

# Tidy dependencies
tidy:
	go mod tidy

# Run all checks (format, vet, test)
check: fmt vet test

# Install git hooks
install-hooks:
	@./scripts/install-hooks.sh

# Update README Table of Contents
doctoc:
	npx doctoc --maxlevel 3 README.md --github --title "## Table of Contents"

# Display help
help:
	@echo "Available targets:"
	@echo "  build         - Build the $(BINARY) binary"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run all tests"
	@echo "  fmt           - Format all Go source files"
	@echo "  vet           - Run go vet on all packages"
	@echo "  deps          - Download and verify dependencies"
	@echo "  tidy          - Tidy go.mod and go.sum"
	@echo "  check         - Run format check, vet, and tests"
	@echo "  install-hooks - Install git pre-commit hooks"
	@echo "  doctoc        - Update README table of contents"
	@echo "  help          - Show this help message"
