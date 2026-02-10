# LocalFiles Index - Makefile

# Auto-detect commands from cmd/ directory
COMMANDS := $(shell ls -d cmd/*/  2>/dev/null | xargs -I{} basename {})
DEFAULT_BINARY_NAME := $(firstword $(COMMANDS))
MODULE_NAME := $(DEFAULT_BINARY_NAME)
BUILD_DIR := bin

# Platform detection
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
BINARY_SUFFIX := -$(GOOS)-$(GOARCH)

# Install directory
ifeq ($(shell id -u),0)
    TARGET ?= /usr/local/bin
else
    TARGET ?= $(HOME)/.local/bin
endif

.PHONY: build build-all rebuild rebuild-all run install uninstall \
        test test-unit test-all e2e-test \
        fmt vet lint check \
        clean clean-all init-mod init-deps \
        list-commands info help db-setup

# Build targets
build:
	@for cmd in $(COMMANDS); do \
		echo "Building $$cmd..."; \
		go build -o $(BUILD_DIR)/$$cmd$(BINARY_SUFFIX) ./cmd/$$cmd/; \
	done
	@if [ "$(GOOS)" = "darwin" ]; then \
		for f in $(BUILD_DIR)/*-darwin-*; do \
			[ -f "$$f" ] && codesign -s - "$$f" 2>/dev/null || true; \
		done; \
	fi
	@echo "Build complete."

build-all:
	@for cmd in $(COMMANDS); do \
		for os_arch in linux-amd64 darwin-amd64 darwin-arm64 windows-amd64; do \
			os=$$(echo $$os_arch | cut -d- -f1); \
			arch=$$(echo $$os_arch | cut -d- -f2); \
			ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
			echo "Building $$cmd for $$os/$$arch..."; \
			GOOS=$$os GOARCH=$$arch go build -o $(BUILD_DIR)/$$cmd-$$os-$$arch$$ext ./cmd/$$cmd/; \
		done; \
	done
	@echo "Build complete (all platforms)."

rebuild: clean build
rebuild-all: clean build-all

# Run target
run:
ifndef CMD
	$(error CMD is required. Usage: make run CMD=<command> ARGS='<arguments>')
endif
	@go run ./cmd/$(CMD)/ $(ARGS)

# Install targets
install: build
	@mkdir -p $(TARGET)
	@for cmd in $(COMMANDS); do \
		cp $(BUILD_DIR)/$$cmd$(BINARY_SUFFIX) $(TARGET)/$$cmd; \
		echo "Installed $$cmd to $(TARGET)/$$cmd"; \
	done

uninstall:
	@for cmd in $(COMMANDS); do \
		rm -f $(TARGET)/$$cmd; \
		echo "Removed $(TARGET)/$$cmd"; \
	done

# Test targets
test:
	@if [ -f tests/run_tests.sh ]; then \
		chmod +x tests/run_tests.sh; \
		./tests/run_tests.sh; \
	else \
		echo "No functional tests found."; \
	fi

test-unit:
	@go test ./...

test-all: test-unit test

e2e-test: build test

# Code quality
fmt:
	@go fmt ./...

vet:
	@go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi

check: fmt vet lint test-all

# Clean targets
clean:
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete."

clean-all: clean
	@rm -f go.mod go.sum
	@echo "Full clean complete."

# Module management
init-mod:
	@go mod init $(MODULE_NAME)

init-deps: init-mod
	@go mod tidy

# Database setup
db-setup:
	@echo "Creating PostgreSQL database..."
	@psql -U postgres -c "CREATE USER localfiles WITH PASSWORD 'localfiles';" 2>/dev/null || true
	@psql -U postgres -c "CREATE DATABASE localfiles OWNER localfiles;" 2>/dev/null || true
	@psql -U localfiles -d localfiles -c "CREATE EXTENSION IF NOT EXISTS vector;" 2>/dev/null || true
	@echo "Database setup complete."

# Info targets
list-commands:
	@echo "Available commands:"
	@for cmd in $(COMMANDS); do echo "  $$cmd"; done

info:
	@echo "Platform: $(GOOS)/$(GOARCH)"
	@echo "Module:   $(MODULE_NAME)"
	@echo "Commands: $(COMMANDS)"
	@echo "Build:    $(BUILD_DIR)"

help:
	@echo "Usage:"
	@echo "  make build        Build for current platform"
	@echo "  make build-all    Build for all platforms"
	@echo "  make run CMD=x    Run command x"
	@echo "  make install      Install to $(TARGET)"
	@echo "  make test         Run functional tests"
	@echo "  make test-unit    Run unit tests"
	@echo "  make e2e-test     Build + run functional tests"
	@echo "  make check        Run all checks"
	@echo "  make db-setup     Create PostgreSQL database"
	@echo "  make clean        Remove build artifacts"
	@echo "  make help         Show this help"
