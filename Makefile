APP_NAME := typ
BUILD_DIR := bin
BINARY_NAME := $(APP_NAME)
PREFIX := $(HOME)/go/bin
GOLANGCI_LINT_VERSION := v1.64.5

.PHONY: all build clean install uninstall test lint lint-install run help

all: build

## help: Show this help message
help:
	@echo "TYP: Terminal YouTube Player Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## build: Compile the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/typ
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Run 'make lint-install' to install it."; \
		exit 1; \
	fi

## lint-install: Install golangci-lint locally
lint-install:
	@echo "Installing golangci-lint..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)

## test: Run linter and all tests (btfp style)
test: lint
	@echo "Running tests..."
	@go test -v ./...

## install: Build and install binary to $(PREFIX)
install: build
	@echo "Stopping running instances..."
	@pkill -9 $(BINARY_NAME) || true
	@echo "Installing to $(PREFIX)..."
	@mkdir -p $(PREFIX)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(PREFIX)/
	@echo "Setting up configuration directories..."
	@mkdir -p $(HOME)/.config/typ
	@mkdir -p $(XDG_RUNTIME_DIR)/typ
	@echo "Installation complete. Ensure $(PREFIX) is in your PATH."

## uninstall: Remove binary and config
uninstall:
	@echo "Uninstalling from $(PREFIX)..."
	@rm -f $(PREFIX)/$(BINARY_NAME)
	@echo "Uninstall complete."

## run: Build and run the TUI (will auto-start daemon if implemented in code)
run: build
	@./$(BUILD_DIR)/$(BINARY_NAME)

## clean: Remove build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
