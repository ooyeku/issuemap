# Makefile for IssueMap

# Variables
BINARY_NAME=issuemap
BUILD_DIR=bin
GO_FILES=$(shell find . -name "*.go" -type f)
VERSION?=0.1.0

# Default target
.PHONY: all build install install-with-path uninstall check-path test test-unit test-coverage test-integration test-stress test-performance test-all test-full lint fmt vet clean deps docs setup dev build-all check help

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" -o $(BUILD_DIR)/$(BINARY_NAME) .

# Install the application
install: build
	@echo "Installing $(BINARY_NAME)..."
	@go install -ldflags="-X 'github.com/ooyeku/$(BINARY_NAME)/internal/app.Version=$(VERSION)'" .
	@echo "$(BINARY_NAME) installed successfully!"
	@echo "Binary location: $$(go env GOPATH)/bin/$(BINARY_NAME)"
	@echo "Make sure $$(go env GOPATH)/bin is in your PATH to run '$(BINARY_NAME)' from anywhere"
	@echo ""
	@echo "Test the installation:"
	@echo "  $(BINARY_NAME) --version"
	@echo "  $(BINARY_NAME) --help"

# Uninstall the application
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@if [ -f "$$(go env GOPATH)/bin/$(BINARY_NAME)" ]; then \
		rm -f "$$(go env GOPATH)/bin/$(BINARY_NAME)"; \
		echo "✅ $(BINARY_NAME) uninstalled successfully!"; \
	else \
		echo "⚠️  $(BINARY_NAME) not found in $$(go env GOPATH)/bin/"; \
	fi

# Check PATH configuration
check-path:
	@echo "Checking PATH configuration..."
	@GOPATH_BIN="$$(go env GOPATH)/bin"; \
	echo "📁 GOPATH/bin location: $$GOPATH_BIN"; \
	if echo "$$PATH" | grep -q "$$GOPATH_BIN"; then \
		echo "✅ $$GOPATH_BIN is in your PATH"; \
		if command -v $(BINARY_NAME) >/dev/null 2>&1; then \
			echo "✅ $(BINARY_NAME) is accessible from terminal"; \
		else \
			echo "⚠️  $(BINARY_NAME) not found in PATH (try 'make install')"; \
		fi; \
	else \
		echo "❌ $$GOPATH_BIN is NOT in your PATH"; \
		echo ""; \
		echo "To add it to your PATH, add this line to your shell profile:"; \
		echo "  export PATH=\"\$$PATH:$$GOPATH_BIN\""; \
		echo ""; \
		echo "For bash: ~/.bashrc or ~/.bash_profile"; \
		echo "For zsh:  ~/.zshrc"; \
		echo "For fish: ~/.config/fish/config.fish"; \
		echo ""; \
		echo "Then reload your shell or run: source ~/.zshrc"; \
	fi

# Install and setup PATH automatically
install-with-path: install
	@echo ""
	@echo "Setting up PATH automatically..."
	@GOPATH_BIN="$$(go env GOPATH)/bin"; \
	SHELL_RC=""; \
	if [ "$$SHELL" = "/bin/zsh" ] || [ "$$SHELL" = "/usr/bin/zsh" ]; then \
		SHELL_RC="$$HOME/.zshrc"; \
	elif [ "$$SHELL" = "/bin/bash" ] || [ "$$SHELL" = "/usr/bin/bash" ]; then \
		if [ -f "$$HOME/.bashrc" ]; then \
			SHELL_RC="$$HOME/.bashrc"; \
		else \
			SHELL_RC="$$HOME/.bash_profile"; \
		fi; \
	fi; \
	if [ -n "$$SHELL_RC" ]; then \
		if ! grep -q "$$GOPATH_BIN" "$$SHELL_RC" 2>/dev/null; then \
			echo "export PATH=\"\$$PATH:$$GOPATH_BIN\"" >> "$$SHELL_RC"; \
			echo "✅ Added $$GOPATH_BIN to $$SHELL_RC"; \
			echo "🔄 Please run: source $$SHELL_RC"; \
			echo "   Or restart your terminal to use 'issuemap' from anywhere"; \
		else \
			echo "✅ $$GOPATH_BIN already in $$SHELL_RC"; \
		fi; \
	else \
		echo "⚠️  Could not detect shell profile. Please manually add:"; \
		echo "   export PATH=\"\$$PATH:$$GOPATH_BIN\""; \
	fi

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
	@echo "  install         - Install the application to GOPATH/bin"
	@echo "  install-with-path - Install and automatically add GOPATH/bin to PATH"
	@echo "  uninstall       - Remove the application from GOPATH/bin"
	@echo "  check-path      - Check if GOPATH/bin is in PATH and provide setup instructions"
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