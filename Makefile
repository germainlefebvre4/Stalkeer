# Makefile for Stalkeer

.PHONY: all build test clean run lint fmt help

# Variables
BINARY_NAME=stalkeer
BIN_DIR=bin
CMD_DIR=cmd
MAIN_FILE=$(CMD_DIR)/main.go

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

all: test build

## build: Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)/...
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

## coverage: Generate test coverage report
coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

## run: Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BIN_DIR)/$(BINARY_NAME)

## lint: Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run
	@echo "Linting complete"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...
	@which goimports > /dev/null && goimports -w . || echo "goimports not found, skipping import organization"
	@echo "Formatting complete"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies downloaded"

## verify: Verify dependencies
verify:
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	@echo "Dependencies verified"

## docker-up: Start Docker services
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d
	@echo "Docker services started"

## docker-down: Stop Docker services
docker-down:
	@echo "Stopping Docker services..."
	docker-compose down
	@echo "Docker services stopped"

## docker-logs: View Docker logs
docker-logs:
	docker-compose logs -f

## db-migrate: Run database migrations
db-migrate:
	@echo "Running database migrations..."
	@./$(BIN_DIR)/$(BINARY_NAME) migrate || echo "Build the application first with 'make build'"

# Drop and create the database
db-drop-create:
	PGPASSWORD=postgres psql -h localhost -U postgres -c "DROP DATABASE stalkeer;" || true
	PGPASSWORD=postgres psql -h localhost -U postgres -c "CREATE DATABASE stalkeer;"

db-truncate-tables:
	PGPASSWORD=postgres psql -h localhost -U postgres -d stalkeer -c "TRUNCATE channels, movies, tvshows, uncategorized, processed_lines, processing_logs, download_info RESTART IDENTITY CASCADE;"

## help: Display this help message
help:
	@echo "Stalkeer Makefile Commands:"
	@echo ""
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
