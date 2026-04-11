.PHONY: build test run clean docker lint fmt help fetch-onnxruntime fetch-minilm

# Variables
BINARY_NAME=tiersum
BUILD_DIR=./build
CMD_DIR=./cmd
DOCKER_IMAGE=tiersum/tiersum
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-w -s -X main.Version=$(VERSION)"

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build binary for local OS
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

build-linux: ## Build binary for Linux (amd64)
	@echo "Building for Linux amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

build-all: build-linux ## Build for all platforms
	@echo "Building for all platforms..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

run: build ## Build and run locally
	$(BUILD_DIR)/$(BINARY_NAME) --config config.yaml

dev: ## Run with hot reload (requires air)
	air -c .air.toml

test: ## Run all tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

docker-build: ## Build Docker image (Dockerfile under deployments/docker)
	docker build -f deployments/docker/Dockerfile -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest --build-arg VERSION=$(VERSION) .

docker-push: docker-build ## Push Docker image
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

docker-compose-up: ## Start with docker-compose
	docker-compose up -d

docker-compose-down: ## Stop docker-compose
	docker-compose down

docker-compose-logs: ## View docker-compose logs
	docker-compose logs -f tiersum

deps: ## Download and tidy dependencies
	go mod download
	go mod tidy
	go mod verify

fetch-onnxruntime: ## Vendor ONNX Runtime into third_party/ (MiniLM; optional, no OS package)
	chmod +x scripts/fetch-onnxruntime.sh
	./scripts/fetch-onnxruntime.sh host

fetch-minilm: ## Download MiniLM ONNX + tokenizer into third_party/minilm/ (reproducible model files)
	chmod +x scripts/fetch-minilm.sh
	./scripts/fetch-minilm.sh

generate: ## Generate code (mocks, etc.)
	go generate ./...

migrate-up: ## Run database migrations up
	go run ./cmd/migrate up

migrate-down: ## Run database migrations down
	go run ./cmd/migrate down

seed: ## Seed database with sample data
	go run ./cmd/seed

.DEFAULT_GOAL := help
