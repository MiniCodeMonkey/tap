# Tap - Markdown Presentation Tool
# Build and development commands

BINARY_NAME=tap
BUILD_DIR=bin
VERSION ?= 0.1.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/tap

# Run tests
.PHONY: test
test:
	$(GOTEST) ./...

# Run benchmarks
.PHONY: bench
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Run benchmarks with specific packages
.PHONY: bench-parser
bench-parser:
	$(GOTEST) -bench=. -benchmem ./internal/parser/...

.PHONY: bench-builder
bench-builder:
	$(GOTEST) -bench=. -benchmem ./internal/builder/...

.PHONY: bench-server
bench-server:
	$(GOTEST) -bench=. -benchmem ./internal/server/...

# Run performance target tests
.PHONY: bench-targets
bench-targets:
	$(GOTEST) -v -run "PerformanceTarget" ./...

# Run linter (golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Run development server (placeholder for now)
.PHONY: dev
dev: build
	./$(BUILD_DIR)/$(BINARY_NAME) dev

# Build release binaries for all platforms
.PHONY: release
release: clean release-darwin-amd64 release-darwin-arm64 release-linux-amd64 release-linux-arm64 release-windows-amd64

.PHONY: release-darwin-amd64
release-darwin-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/tap

.PHONY: release-darwin-arm64
release-darwin-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/tap

.PHONY: release-linux-amd64
release-linux-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/tap

.PHONY: release-linux-arm64
release-linux-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/tap

.PHONY: release-windows-amd64
release-windows-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/tap

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Verify code compiles (typecheck)
.PHONY: typecheck
typecheck:
	$(GOBUILD) ./...

.PHONY: help
help:
	@echo "Tap Makefile targets:"
	@echo "  build          - Build the tap binary to bin/tap"
	@echo "  test           - Run all tests"
	@echo "  lint           - Run golangci-lint"
	@echo "  dev            - Build and run development server"
	@echo "  release        - Build release binaries for all platforms"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  typecheck      - Verify code compiles"
	@echo "  bench          - Run all benchmarks"
	@echo "  bench-parser   - Run parser benchmarks only"
	@echo "  bench-builder  - Run builder benchmarks only"
	@echo "  bench-server   - Run server benchmarks only"
	@echo "  bench-targets  - Run performance target tests"
	@echo "  help           - Show this help message"
