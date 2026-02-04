.PHONY: help build test lint clean install build-all test-integration test-coverage

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the main binary
	go build -ldflags "-X main.version=dev" -o bin/agent cmd/sync/*.go

build-all: ## Build for all platforms
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=dev" -o bin/agent-linux-amd64 cmd/sync/*.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=dev" -o bin/agent-darwin-amd64 cmd/sync/*.go
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=dev" -o bin/agent-darwin-arm64 cmd/sync/*.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=dev" -o bin/agent-windows-amd64.exe cmd/sync/*.go

install: build ## Install the binary to /usr/local/bin
	sudo cp bin/agent /usr/local/bin/

# Test targets
test: ## Run all tests
	go test -v -race -coverprofile=coverage.out ./...

test-integration: ## Run integration tests
	go test -v ./test/integration/...

test-coverage: test ## Run tests with coverage and generate HTML report
	go tool cover -html=coverage.out -o coverage.html

# Quality targets
lint: ## Lint code
	golangci-lint run

fmt: ## Format code
	go fmt ./...
	goimports -w .

tidy: ## Tidy dependencies
	go mod tidy
	go mod verify

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

# Development targets
dev: ## Build and run in development mode
	go run -ldflags "-X main.version=dev" cmd/sync/*.go

deps: ## Download dependencies
	go mod download

# Release targets
release: clean test lint build-all ## Full release pipeline

# CI targets
ci: deps test lint ## CI pipeline