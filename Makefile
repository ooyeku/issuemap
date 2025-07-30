# Makefile for IssueMap

# Variables
BINARY_NAME=issuemap
BUILD_DIR=bin
GO_FILES=$(shell find . -name "*.go" -type f)

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) .

# Install the binary to GOPATH/bin
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install .

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linting
.PHONY: lint
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@golangci-lint run

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	@go vet ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Generate documentation
.PHONY: docs
docs:
	@echo "Generating documentation..."
	@which godoc > /dev/null || go install golang.org/x/tools/cmd/godoc@latest
	@echo "Run 'godoc -http=:6060' and visit http://localhost:6060/pkg/github.com/ooyeku/issuemap/"

# Development setup
.PHONY: setup
setup: deps
	@echo "Setting up development environment..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@which godoc > /dev/null || go install golang.org/x/tools/cmd/godoc@latest

# Run development version
.PHONY: dev
dev: build
	@echo "Running development version..."
	@./$(BUILD_DIR)/$(BINARY_NAME)

# Build for multiple platforms
.PHONY: build-all
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Built binaries:"
	@ls -la $(BUILD_DIR)/

# Check everything before commit
.PHONY: check
check: fmt vet lint test
	@echo "All checks passed!"

# Quick integration test
.PHONY: integration
integration: build
	@echo "Running integration test..."
	@cd /tmp && \
		git init test-issuemap && \
		cd test-issuemap && \
		git config user.name "Test User" && \
		git config user.email "test@example.com" && \
		echo "# Test Repo" > README.md && \
		git add README.md && \
		git commit -m "Initial commit" && \
		$(PWD)/$(BUILD_DIR)/$(BINARY_NAME) init --name "Test Project" && \
		$(PWD)/$(BUILD_DIR)/$(BINARY_NAME) create "Test issue" --type bug --priority high && \
		$(PWD)/$(BUILD_DIR)/$(BINARY_NAME) list && \
		$(PWD)/$(BUILD_DIR)/$(BINARY_NAME) show ISSUE-001 && \
		echo "Integration test passed!" && \
		cd .. && rm -rf test-issuemap

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  install      - Install the binary to GOPATH/bin"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  vet          - Vet code"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  docs         - Generate documentation"
	@echo "  setup        - Set up development environment"
	@echo "  dev          - Build and run development version"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  check        - Run all checks (fmt, vet, lint, test)"
	@echo "  integration  - Run integration test"
	@echo "  help         - Show this help" 