# This Makefile provides targets for building, linting and testing.

# Directories
BUILD_DIR := build

# Build informations
BUILD_USER ?= $(shell whoami)@$(shell hostname)
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)
PROJECT_NAME := claude-statusline
VERSION := $(shell git describe --always --tags)

# Default target
.DEFAULT_GOAL := build

# Supported architectures and OSes
ARCHS = amd64 arm64
OSES = linux darwin windows
EXT = $(if $(filter $(GOOS),windows),.exe,)

# Colors for output
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

##@ Building

GO_BIN := $(BUILD_DIR)/$(PROJECT_NAME).$(GOOS)-$(GOARCH)$(EXT)

.PHONY: build
build: ## Build the Go binary
	@printf "$(CYAN)Building Go binary...$(RESET)\n"
	mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -v -o ./$(GO_BIN) -ldflags=" \
	-s -w \
	-X github.com/prometheus/common/version.Version=$(VERSION) \
	-X github.com/prometheus/common/version.Revision=$(shell git rev-parse HEAD) \
	-X github.com/prometheus/common/version.Branch=$(shell git rev-parse --abbrev-ref HEAD) \
	-X github.com/prometheus/common/version.BuildUser=$(BUILD_USER) \
	-X github.com/prometheus/common/version.BuildDate=$(shell date --utc +%FT%T)" \
	.
	@printf "$(GREEN)Build completed. Output is in $(GO_BIN)\n"

.PHONY: build-all
build-all: ## Build the Go binary for all supported OSes and architectures
	$(foreach GOOS,$(OSES),$(foreach GOARCH,$(ARCHS), \
		$(MAKE) GOOS=$(GOOS) GOARCH=$(GOARCH) build; \
	))

.PHONY: install
install: build ## Install the binary to ~/.local/bin
	@printf "$(CYAN)Installing binary to ~/.local/bin...$(RESET)\n"
	@mkdir -p ~/.local/bin
	cp $(GO_BIN) ~/.local/bin/$(PROJECT_NAME)
	chmod +x ~/.local/bin/$(PROJECT_NAME)
	@printf "$(GREEN)Binary installed to ~/.local/bin/$(PROJECT_NAME)$(RESET)\n"

.PHONY: clean
clean: ## Clean build artifacts
	@printf "$(CYAN)Cleaning build artifacts...$(RESET)\n"
	rm -rf $(BUILD_DIR)
	@printf "$(GREEN)Cleanup completed$(RESET)\n"

##@ Testing

.PHONY: test
test: ## Run the complete test suite, optionally filtered by run_pattern or bench_pattern
	@printf "$(CYAN)Running tests...$(RESET)\n"
	go test -v -race -run="$(run_pattern)" -bench="$(bench_pattern)" -benchmem ./...
	@printf "$(GREEN)Tests completed successfully$(RESET)\n"

.PHONY: bench
bench: ## Run all benchmarks
	@printf "$(CYAN)Running benchmarks...$(RESET)\n"
	go test -run=^$$ -bench=. -benchmem ./...
	@printf "$(GREEN)Benchmarks completed$(RESET)\n"

##@ Code Quality

.PHONY: lint
lint: ## Run golangci-lint for comprehensive code analysis (requires CGO environment)
	@printf "$(CYAN)Running golangci-lint...$(RESET)\n"
	golangci-lint run -E gosec -E goconst --timeout 10m --max-same-issues 0 --max-issues-per-linter 0 ./...
	@printf "$(GREEN)Linting completed$(RESET)\n"

.PHONY: vet
vet: ## Run go vet for static analysis
	@printf "$(CYAN)Running go vet...$(RESET)\n"
	go vet ./...
	@printf "$(GREEN)Static analysis completed$(RESET)\n"

.PHONY: fmt
fmt: ## Check code formatting
	@printf "$(CYAN)Checking code formatting...$(RESET)\n"
	gofmt -d .

.PHONY: lint-all
lint-all: fmt vet lint ## Run all linting checks

##@ Security

nancy: ## Run Nancy vulnerability scan
	@printf "$(CYAN)Running nancy vulnerability scan...$(RESET)\n"
	sh -c "go list -json -m all | nancy sleuth"
	@printf "$(GREEN)Nancy scan completed$(RESET)\n"

.PHONY: security
security: nancy ## Run all security scans

##@ Help

.PHONY: help
help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(CYAN)Usage:$(RESET)\n  make $(YELLOW)<target>$(RESET)\n"} /^[a-zA-Z_0-9-]+.*?##/ { printf "  $(YELLOW)%-20s$(RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(CYAN)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@printf "\n"
	@printf "$(CYAN)Examples:$(RESET)\n"
	@printf "  make install                        # Install to ~/.local/bin\n"
	@printf "  make build                          # Build the binary\n"
	@printf "  make test                           # Run all tests\n"
	@printf "  make test run_pattern=Parse         # Run tests matching 'Parse'\n"
	@printf "  make bench                          # Run all benchmarks\n"
	@printf "  make lint-all                       # Run all code quality checks\n"
	@printf "  make security                       # Run all security scans\n"
	@printf "  make clean                          # Clean all artifacts\n"
