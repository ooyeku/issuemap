# Makefile for IssueMap

# Variables
BINARY_NAME=issuemap
BUILD_DIR=bin
GO_FILES=$(shell find . -name "*.go" -type f)
VERSION?=0.1.0

# Default target
.PHONY: all build install test test-unit test-coverage test-integration test-stress test-performance test-all test-full lint fmt vet clean deps docs setup dev build-all check help

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Install the application
install: build
	@echo "Installing $(BINARY_NAME)..."
	@go install -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" .

# Run unit tests (if any exist, otherwise run integration tests)
test:
	@echo "Running tests..."
	@if find ./internal ./cmd -name "*_test.go" 2>/dev/null | grep -q .; then \
		echo "Running unit tests..."; \
		go test -short -v ./...; \
	else \
		echo "No unit tests found in main packages, running integration tests..."; \
		go test -v ./test/integration/...; \
	fi

# Run unit tests only (skip if none exist)
test-unit:
	@echo "Running unit tests..."
	@if find ./internal ./cmd -name "*_test.go" 2>/dev/null | grep -q .; then \
		go test -short -v ./...; \
	else \
		echo "No unit tests found in main packages"; \
	fi

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@if find ./internal ./cmd -name "*_test.go" 2>/dev/null | grep -q .; then \
		go test -short -coverprofile=coverage.out ./...; \
		go tool cover -html=coverage.out -o coverage.html; \
		echo "Coverage report generated: coverage.html"; \
	else \
		echo "Running integration tests for coverage..."; \
		go test -coverprofile=coverage.out ./test/integration/...; \
		go tool cover -html=coverage.out -o coverage.html; \
		echo "Integration test coverage report generated: coverage.html"; \
	fi

# Run integration tests
test-integration: build
	@echo "Running integration tests..."
	@go test -v ./test/integration/...

# Run stress tests  
test-stress: build
	@echo "Running stress tests..."
	@go test -v ./test/integration/... -run "Stress|LongRunning|MemoryLeak"

# Run performance tests
test-performance: build
	@echo "Running performance tests..."
	@go test -v ./test/integration/... -run "Performance|Latency|Concurrent"

# Run all tests (unit + integration + stress + performance)
test-all:
	@echo "Running all tests..."
	@echo "=== Unit Tests ==="
	@$(MAKE) test-unit || true
	@echo "=== Integration Tests ==="
	@$(MAKE) test-integration
	@echo "=== Stress Tests ==="
	@$(MAKE) test-stress
	@echo "=== Performance Tests ==="
	@$(MAKE) test-performance

# Run comprehensive test suite with coverage
test-full: test-coverage test-integration test-stress test-performance
	@echo "Full test suite completed"

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
ci: clean deps check test-full build
	@echo "CI pipeline completed successfully!"

# Benchmark tests
benchmark: build
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./test/integration/...

# Run specific integration test
test-sync: build
	@echo "Running sync integration tests..."
	@go test -v ./test/integration/... -run "TestCLI.*Sync|TestServerSync"

# Run quick integration tests (excluding stress tests)
test-quick: build
	@echo "Running quick integration tests..."
	@go test -v ./test/integration/... -run "^TestIntegrationSuite/(TestBasicServerStartup|TestCLIIssueCreationSync|TestCLIIssueUpdateSync)$$"

# Help target
help:
	@echo "Available targets:"
	@echo "  build           - Build the application"
	@echo "  install         - Install the application"
	@echo ""
	@echo "Testing targets:"
	@echo "  test            - Run tests (unit if available, else integration)"
	@echo "  test-unit       - Run unit tests only"
	@echo "  test-coverage   - Run tests with coverage report"
	@echo "  test-integration- Run integration tests"
	@echo "  test-stress     - Run stress tests"
	@echo "  test-performance- Run performance tests"
	@echo "  test-all        - Run all test types (unit + integration + stress + performance)"
	@echo "  test-full       - Run comprehensive test suite with coverage"
	@echo "  test-quick      - Run quick integration tests"
	@echo "  test-sync       - Run CLI-server sync tests"
	@echo "  benchmark       - Run benchmark tests"
	@echo ""
	@echo "Development targets:"
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
	@echo "  help            - Show this help message" 