# Enterprise Scheduler Makefile

.PHONY: help build test test-race test-cgo-disabled lint coverage clean install-tools

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the scheduler binary
	@echo "Building scheduler..."
	go build -v -o bin/scheduler ./cmd/scheduler

build-cgo-disabled: ## Build with CGO disabled
	@echo "Building scheduler with CGO disabled..."
	CGO_ENABLED=0 go build -v -o bin/scheduler-static ./cmd/scheduler

build-all: ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/scheduler-linux-amd64 ./cmd/scheduler
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o bin/scheduler-darwin-amd64 ./cmd/scheduler
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o bin/scheduler-windows-amd64.exe ./cmd/scheduler
	@echo "Built binaries:"
	@ls -la bin/

# Test targets
test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...

test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	go test -v -race ./...

test-cgo-disabled: ## Run tests with CGO disabled
	@echo "Running tests with CGO disabled..."
	CGO_ENABLED=0 go test -v ./...

test-short: ## Run short tests only
	@echo "Running short tests..."
	go test -v -short ./...

# Coverage targets
coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

coverage-func: ## Show coverage by function
	@echo "Coverage by function:"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-check: ## Check coverage threshold (80%)
	@echo "Checking coverage threshold..."
	@go test -v -coverprofile=coverage.out ./...
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	THRESHOLD=80; \
	echo "Current coverage: $${COVERAGE}%"; \
	echo "Required threshold: $${THRESHOLD}%"; \
	if [ $$(echo "$${COVERAGE} < $${THRESHOLD}" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage $${COVERAGE}% is below threshold $${THRESHOLD}%"; \
		exit 1; \
	else \
		echo "✅ Coverage $${COVERAGE}% meets threshold $${THRESHOLD}%"; \
	fi

# Lint targets
lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	golangci-lint run

lint-fix: ## Run golangci-lint with auto-fix
	@echo "Running golangci-lint with auto-fix..."
	golangci-lint run --fix

# Development targets
dev: ## Run in development mode
	@echo "Starting scheduler in development mode..."
	go run ./cmd/scheduler

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securecodewarrior/sast-scan@latest

# Database targets
migrate: ## Run database migrations
	@echo "Running database migrations..."
	go run ./cmd/scheduler -migrate-only

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache
	go clean -testcache

# CI targets (used by GitHub Actions)
ci-lint: install-tools lint ## CI: Run linting

ci-test: test test-race coverage-check ## CI: Run all tests with coverage check

ci-build: build build-cgo-disabled build-all ## CI: Build all variants

ci-all: ci-lint ci-test ci-build ## CI: Run all CI checks

# Docker targets (optional)
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t enterprise-scheduler:latest .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 enterprise-scheduler:latest

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	go mod verify

# Security scanning
security-scan: ## Run security scan
	@echo "Running security scan..."
	gosec ./...

# Format code
fmt: ## Format Go code
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -w .

# Check for potential issues
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

# All quality checks
quality: fmt vet lint test-race coverage-check ## Run all quality checks

# Release preparation
pre-release: clean quality ci-build ## Prepare for release