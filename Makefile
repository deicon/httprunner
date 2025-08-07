# Variables
BINARY_NAME=httprunner
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=${VERSION}"

# Default target
.PHONY: all
all: clean test build

# Clean build directory
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	mkdir -p $(BUILD_DIR)

# Run tests
.PHONY: test
test:
	go test ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -cover ./...

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)

# Build for all platforms
.PHONY: build-all
build-all: clean
	# Linux
	GOOS=linux GOARCH=386 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-386
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64
	
	# Windows
	GOOS=windows GOARCH=386 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-386.exe
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe
	
	# macOS
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Run linter (if golangci-lint is available)
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed, skipping..." && exit 0)
	golangci-lint run

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Run all checks (format, lint, test)
.PHONY: check
check: fmt lint test

# Development build (fast)
.PHONY: dev
dev:
	go build -o $(BUILD_DIR)/$(BINARY_NAME)

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all         - Clean, test, and build"
	@echo "  clean       - Remove build directory"
	@echo "  test        - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  build       - Build for current platform"
	@echo "  build-all   - Build for all supported platforms"
	@echo "  deps        - Install dependencies"
	@echo "  lint        - Run linter"
	@echo "  fmt         - Format code"
	@echo "  check       - Run format, lint, and test"
	@echo "  dev         - Quick development build"
	@echo "  help        - Show this help"