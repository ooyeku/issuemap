# Makefile for IssueMap

# Variables
BINARY_NAME=issuemap
BUILD_DIR=bin
GO_FILES=$(shell find . -name "*.go" -type f)
VERSION?=0.1.0

# Default target
.PHONY: all build install test test-coverage test-integration test-stress test-performance lint fmt vet clean deps docs setup dev build-all check help

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Install the application
install: build
	@echo "Installing $(BINARY_NAME)..."
	@go install -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" .

# Run unit tests
test:
	@echo "Running unit tests..."
	@go test -short -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -short -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@go test -v ./test/integration/...

# Run stress tests  
test-stress:
	@echo "Running stress tests..."
	@go test -v ./test/integration/... -run "Stress|LongRunning|MemoryLeak"

# Run performance tests
test-performance:
	@echo "Running performance tests..."
	@go test -v ./test/integration/... -run "Performance|Latency|Concurrent"

# Run all tests (unit + integration)
test-all: test test-integration

# Lint the code
lint:
	@echo "Linting code..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	@golangci-lint run

# Format the code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@go clean

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Generate documentation
docs:
	@echo "Generating documentation..."
	@command -v godoc >/dev/null 2>&1 || { echo "Installing godoc..."; go install golang.org/x/tools/cmd/godoc@latest; }
	@echo "Documentation server will be available at http://localhost:6060"
	@godoc -http=:6060

# Setup development environment
setup: deps
	@echo "Setting up development environment..."
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@command -v godoc >/dev/null 2>&1 || go install golang.org/x/tools/cmd/godoc@latest
	@echo "Development environment ready!"

# Development workflow (format, lint, test, build)
dev: fmt vet lint test build
	@echo "Development build complete!"

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

# Quality check (lint + vet + test)
check: lint vet test

# CI pipeline simulation
ci: clean deps check test-integration build
	@echo "CI pipeline completed successfully!"

# Benchmark tests
benchmark:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./test/integration/...

# Run specific integration test
test-sync:
	@echo "Running sync integration tests..."
	@go test -v ./test/integration/... -run "TestCLI.*Sync|TestServerSync"

# Run quick integration tests (excluding stress tests)
test-quick:
	@echo "Running quick integration tests..."
	@go test -short -v ./test/integration/... -run -v "^((?!Stress|LongRunning|MemoryLeak).)*$$"

# Help target
help:
	@echo "Available targets:"
	@echo "  build           - Build the application"
	@echo "  install         - Install the application"
	@echo "  test            - Run unit tests"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  test-integration- Run integration tests"
	@echo "  test-stress     - Run stress tests"
	@echo "  test-performance- Run performance tests"
	@echo "  test-all        - Run all tests"
	@echo "  test-quick      - Run quick integration tests"
	@echo "  test-sync       - Run CLI-server sync tests"
	@echo "  lint            - Lint the code"
	@echo "  fmt             - Format the code"
	@echo "  vet             - Run go vet"
	@echo "  clean           - Clean build artifacts"
	@echo "  deps            - Download dependencies"
	@echo "  docs            - Generate documentation"
	@echo "  setup           - Setup development environment"
	@echo "  dev             - Development workflow"
	@echo "  build-all       - Build for all platforms"
	@echo "  check           - Quality check (lint + vet + test)"
	@echo "  ci              - CI pipeline simulation"
	@echo "  benchmark       - Run benchmark tests"
	@echo "  help            - Show this help message" 