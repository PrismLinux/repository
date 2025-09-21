# PrismLinux Package Manager Makefile

.PHONY: help build clean-build install sync-stable sync-testing clean-stable clean-testing status

BINARY_NAME=pkg-mgr
BUILD_DIR=build
INSTALL_DIR=/usr/local/bin

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

help: ## Show this help message
	@echo "$(BLUE)PrismLinux Package Manager$(NC)"
	@echo ""
	@echo "$(YELLOW)Build commands:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | head -5
	@echo ""
	@echo "$(YELLOW)Repository commands:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | tail -n +6

build: ## Build the package manager binary
	@echo "$(BLUE)Building package manager...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "$(GREEN)✓ Built $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

clean-build: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)✓ Cleaned build directory$(NC)"

install: build ## Install the package manager to system
	@echo "$(BLUE)Installing package manager...$(NC)"
	@sudo install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "$(GREEN)✓ Installed to $(INSTALL_DIR)/$(BINARY_NAME)$(NC)"

sync-stable: ## Sync stable repository
	@echo "$(BLUE)Syncing stable repository...$(NC)"
	@if [ ! -f $(BUILD_DIR)/$(BINARY_NAME) ]; then make build; fi
	@./$(BUILD_DIR)/$(BINARY_NAME) --verbose
	@echo "$(GREEN)✓ Stable repository synced$(NC)"

sync-testing: ## Sync testing repository
	@echo "$(BLUE)Syncing testing repository...$(NC)"
	@if [ ! -f $(BUILD_DIR)/$(BINARY_NAME) ]; then make build; fi
	@./$(BUILD_DIR)/$(BINARY_NAME) --testing --verbose
	@echo "$(GREEN)✓ Testing repository synced$(NC)"

clean-stable: ## Clean stable repository
	@echo "$(YELLOW)Cleaning stable repository...$(NC)"
	@if [ ! -f $(BUILD_DIR)/$(BINARY_NAME) ]; then make build; fi
	@./$(BUILD_DIR)/$(BINARY_NAME) clean --verbose
	@echo "$(GREEN)✓ Stable repository cleaned$(NC)"

clean-testing: ## Clean testing repository
	@echo "$(YELLOW)Cleaning testing repository...$(NC)"
	@if [ ! -f $(BUILD_DIR)/$(BINARY_NAME) ]; then make build; fi
	@./$(BUILD_DIR)/$(BINARY_NAME) clean --testing --verbose
	@echo "$(GREEN)✓ Testing repository cleaned$(NC)"

status: ## Show repository status
	@echo "$(BLUE)Showing repository status...$(NC)"
	@if [ ! -f $(BUILD_DIR)/$(BINARY_NAME) ]; then make build; fi
	@echo ""
	@echo "$(YELLOW)Stable Repository Status:$(NC)"
	@./$(BUILD_DIR)/$(BINARY_NAME) status
	@echo ""
	@echo "$(YELLOW)Testing Repository Status:$(NC)"
	@./$(BUILD_DIR)/$(BINARY_NAME) status --testing
