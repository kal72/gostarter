# ==============================================================================
# GoStarter — The Production Go Initializr
# Makefile for building, testing, running, and managing the application
# ==============================================================================

APP_NAME     := gostarter
BIN_DIR      := bin
BINARY       := $(BIN_DIR)/$(APP_NAME)
MAIN_SRC     := ./cmd/server
GO           := go
COVERAGE_OUT := coverage.out

# ANSI Colors for terminal output
CYAN   := \033[36m
GREEN  := \033[32m
YELLOW := \033[33m
RED    := \033[31m
RESET  := \033[0m

.PHONY: all build run dev test test-race test-cover fmt vet tidy clean db-up db-down help

## Default target: build the binary
all: build

## Compile the GoStarter standalone binary
build: ui
	@echo "$(CYAN)Building $(APP_NAME)...$(RESET)"
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="-s -w" -o $(BINARY) $(MAIN_SRC)
	@echo "$(GREEN)✓ Build complete: $(BINARY)$(RESET)"

## Build and execute the GoStarter server
run: build
	@echo "$(CYAN)Starting $(APP_NAME)...$(RESET)"
	@./$(BINARY)

## Build and run the application (web/API and embedded validation worker)
.PHONY: start
start: run

.PHONY: migrate
## Apply embedded database migrations and seed defaults without starting the server
migrate:
	$(GO) run $(MAIN_SRC) -migrate-only

## Run web/API server directly in development mode
dev: ui
	@echo "$(CYAN)Starting $(APP_NAME) in dev mode...$(RESET)"
	$(GO) run $(MAIN_SRC)

## Run all automated unit tests across internal packages
test:
	@echo "$(CYAN)Running tests...$(RESET)"
	$(GO) test -v ./internal/...

## Run all unit tests with race condition detector
test-race:
	@echo "$(CYAN)Running tests with race detector...$(RESET)"
	$(GO) test -v -race ./internal/...

## Run unit tests with code coverage analysis
test-cover:
	@echo "$(CYAN)Calculating test coverage...$(RESET)"
	$(GO) test -coverprofile=$(COVERAGE_OUT) ./internal/...
	$(GO) tool cover -func=$(COVERAGE_OUT)
	@echo "$(GREEN)✓ Coverage report generated: $(COVERAGE_OUT)$(RESET)"

## Format all Go source files according to Go standards
fmt:
	@echo "$(CYAN)Formatting Go source files...$(RESET)"
	$(GO) fmt ./...
	@echo "$(GREEN)✓ Code formatted$(RESET)"

## Run go vet static code analysis
vet:
	@echo "$(CYAN)Running go vet static analysis...$(RESET)"
	$(GO) vet ./...
	@echo "$(GREEN)✓ Vet passed$(RESET)"

## Clean and sync Go module dependencies (go mod tidy)
tidy:
	@echo "$(CYAN)Tidying go.mod dependencies...$(RESET)"
	$(GO) mod tidy
	@echo "$(GREEN)✓ go.mod dependencies tidied$(RESET)"

## Clean up compiled binaries and test coverage artifacts
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf $(BIN_DIR)
	@rm -f $(COVERAGE_OUT) coverage.html
	@echo "$(GREEN)✓ Clean complete$(RESET)"

## Spin up a local PostgreSQL 16 Docker container for development
db-up:
	@echo "$(CYAN)Starting PostgreSQL Docker container (gostarter-postgres)...$(RESET)"
	@docker run -d \
		--name gostarter-postgres \
		-p 5432:5432 \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=gostarter \
		postgres:16-alpine || docker start gostarter-postgres
	@echo "$(GREEN)✓ PostgreSQL running on 127.0.0.1:5432 (database: gostarter)$(RESET)"

## Stop and remove the local PostgreSQL container
db-down:
	@echo "$(YELLOW)Stopping PostgreSQL Docker container...$(RESET)"
	@docker stop gostarter-postgres && docker rm gostarter-postgres || true
	@echo "$(GREEN)✓ PostgreSQL container stopped and removed$(RESET)"

## Display available Makefile commands and descriptions
help:
	@echo ""
	@echo "$(CYAN)=================================================================$(RESET)"
	@echo "$(CYAN)   GoStarter — Available Makefile Targets$(RESET)"
	@echo "$(CYAN)=================================================================$(RESET)"
	@echo "  $(GREEN)make start$(RESET)       - Run the application (web/API + embedded worker)"
	@echo "  $(GREEN)make build$(RESET)       - Compile the binary to ./bin/gostarter"
	@echo "  $(GREEN)make run$(RESET)         - Build and run the server using ./bin/gostarter"
	@echo "  $(GREEN)make dev$(RESET)         - Run web/API server via 'go run ./cmd/server'"
	@echo "  $(GREEN)make test$(RESET)        - Run all automated unit tests"
	@echo "  $(GREEN)make test-race$(RESET)   - Run tests with Go race condition detector"
	@echo "  $(GREEN)make test-cover$(RESET)  - Run tests and display coverage metrics"
	@echo "  $(GREEN)make ui$(RESET)          - Generate templ components and build local frontend assets"
	@echo "  $(GREEN)make fmt$(RESET)         - Format all Go source code"
	@echo "  $(GREEN)make vet$(RESET)         - Run go vet static analysis"
	@echo "  $(GREEN)make tidy$(RESET)        - Clean and verify dependencies (go mod tidy)"
	@echo "  $(GREEN)make clean$(RESET)       - Remove build artifacts and coverage files"
	@echo "  $(GREEN)make db-up$(RESET)       - Launch local PostgreSQL container in Docker"
	@echo "  $(GREEN)make migrate$(RESET)     - Apply database migrations without starting the server"
	@echo "  $(GREEN)make db-down$(RESET)     - Stop & remove local PostgreSQL container"
	@echo "  $(GREEN)make help$(RESET)        - Show this target overview menu"
	@echo ""

.PHONY: test-ui
## Test asynchronous UI state (Node.js, no browser installation required)
test-ui:
	node --test tests/unit/*.test.cjs

# Regenerate checked-in Go components and compiled local browser assets.
.PHONY: ui generate assets
ui: generate assets

generate:
	$(GO) tool templ generate

assets:
	npm ci --ignore-scripts --no-audit --no-fund
	npm run build
