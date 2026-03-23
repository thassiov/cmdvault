.PHONY: all build test lint coverage check clean install uninstall fmt tidy vet
.PHONY: release-dry tools help

# Build configuration
BINARY_NAME := cmdvault
BINARY_DIR := bin
BUILD_DIR := ./cmd/cmdvault
INSTALL_DIR := $(HOME)/.local/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
LDFLAGS_RELEASE := -ldflags "-s -w -X main.Version=$(VERSION)"

# Ensure GOPATH/bin is on PATH for tools installed via `go install`
export PATH := $(shell go env GOPATH)/bin:$(PATH)

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m

# Default target
all: check build

# =============================================================================
# Build
# =============================================================================

build:
	@mkdir -p $(BINARY_DIR)
	go build $(LDFLAGS) -o $(BINARY_DIR)/$(BINARY_NAME) $(BUILD_DIR)
	@echo "$(GREEN)Built: $(BINARY_DIR)/$(BINARY_NAME) ($(VERSION))$(NC)"

# Build with release flags (stripped, smaller binary)
build-release:
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS_RELEASE) -o $(BINARY_DIR)/$(BINARY_NAME) $(BUILD_DIR)
	@echo "$(GREEN)Built (release): $(BINARY_DIR)/$(BINARY_NAME) ($(VERSION))$(NC)"
	@ls -lh $(BINARY_DIR)/$(BINARY_NAME) | awk '{print "  Size: " $$5}'

# =============================================================================
# Test
# =============================================================================

test:
	@echo "$(GREEN)Running tests...$(NC)"
	go test -race -timeout 60s ./...

# Run tests with verbose output
test-v:
	@echo "$(GREEN)Running tests (verbose)...$(NC)"
	go test -race -timeout 60s -v ./...

# =============================================================================
# Code quality
# =============================================================================

fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	go fmt ./...
	@which goimports > /dev/null 2>&1 && goimports -w . || echo "$(YELLOW)goimports not installed, skipping (run make tools)$(NC)"

tidy:
	go mod tidy

vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	go vet ./...

lint:
	@echo "$(GREEN)Running golangci-lint...$(NC)"
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || (echo "$(YELLOW)golangci-lint not installed (run make tools)$(NC)" && exit 1)

# Check for known vulnerabilities in dependencies
vuln:
	@echo "$(GREEN)Checking for vulnerabilities...$(NC)"
	@which govulncheck > /dev/null 2>&1 && govulncheck ./... || (echo "$(YELLOW)govulncheck not installed (run make tools)$(NC)" && exit 1)

# Generate coverage report
coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	go test -coverprofile=coverage.out -covermode=atomic -timeout 60s ./...
	@echo ""
	@echo "$(GREEN)Coverage summary:$(NC)"
	@go tool cover -func=coverage.out | grep -E "^total:|internal/|cmd/"
	@echo ""
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Full report: coverage.html$(NC)"

# =============================================================================
# CI pipeline (run locally)
# =============================================================================

# Full CI pipeline
ci: fmt tidy vet lint vuln test coverage build
	@echo ""
	@echo "$(GREEN)========================================$(NC)"
	@echo "$(GREEN)  CI pipeline completed successfully!  $(NC)"
	@echo "$(GREEN)========================================$(NC)"

# Quick check (no coverage, no vuln scan)
check: fmt tidy vet test build
	@echo "$(GREEN)Quick check passed!$(NC)"

# =============================================================================
# Release
# =============================================================================

# Dry-run goreleaser to verify config
release-dry:
	@echo "$(GREEN)Running goreleaser (dry run)...$(NC)"
	@which goreleaser > /dev/null 2>&1 && goreleaser release --snapshot --clean || (echo "$(YELLOW)goreleaser not installed (run make tools)$(NC)" && exit 1)

# =============================================================================
# Install / Uninstall
# =============================================================================

install: build
	@mkdir -p $(INSTALL_DIR)
	@if [ -x $(INSTALL_DIR)/$(BINARY_NAME) ]; then \
		echo "$(YELLOW)Current:$(NC) $$($(INSTALL_DIR)/$(BINARY_NAME) --version 2>/dev/null || echo 'unknown')"; \
		echo "$(GREEN)New:$(NC)     $(VERSION)"; \
	fi
	cp $(BINARY_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "$(GREEN)Installed to $(INSTALL_DIR)/$(BINARY_NAME)$(NC)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "$(GREEN)Removed $(INSTALL_DIR)/$(BINARY_NAME)$(NC)"

# =============================================================================
# Utility
# =============================================================================

clean:
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# Install development tools
tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/goreleaser/goreleaser/v2@latest
	@echo "$(GREEN)Tools installed!$(NC)"

help:
	@echo "Available targets:"
	@echo ""
	@echo "  $(GREEN)Build:$(NC)"
	@echo "    build          - Build binary"
	@echo "    build-release  - Build with release flags (stripped)"
	@echo "    install        - Build and install to ~/.local/bin"
	@echo "    uninstall      - Remove from ~/.local/bin"
	@echo ""
	@echo "  $(GREEN)Test:$(NC)"
	@echo "    test           - Run tests with race detector"
	@echo "    test-v         - Run tests verbose"
	@echo "    coverage       - Generate coverage report"
	@echo ""
	@echo "  $(GREEN)Quality:$(NC)"
	@echo "    fmt            - Format code (go fmt + goimports)"
	@echo "    vet            - Run go vet"
	@echo "    lint           - Run golangci-lint"
	@echo "    vuln           - Check dependency vulnerabilities"
	@echo "    check          - Quick check (fmt, vet, test, build)"
	@echo ""
	@echo "  $(GREEN)Pipeline:$(NC)"
	@echo "    ci             - Full CI pipeline (local)"
	@echo "    release-dry    - Test goreleaser config"
	@echo ""
	@echo "  $(GREEN)Utility:$(NC)"
	@echo "    tools          - Install dev tools (golangci-lint, govulncheck, etc)"
	@echo "    tidy           - go mod tidy"
	@echo "    clean          - Remove build artifacts"
	@echo "    help           - Show this help"
