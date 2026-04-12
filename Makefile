.PHONY: build build-linux build-all test run dev clean lint fmt vet help \
	deps generate fetch-onnxruntime fetch-minilm \
	docker-build docker-build-amd64 docker-build-arm64 docker-build-both \
	docker-push docker-push-both \
	docker-compose-up docker-compose-down docker-compose-logs

# Variables
BINARY_NAME=tiersum
BUILD_DIR=./build
CMD_DIR=./cmd
DOCKER_IMAGE=tiersum/tiersum
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-w -s -X main.Version=$(VERSION)"

# Docker Compose v2 (`docker compose`). Override if needed, e.g. COMPOSE="docker-compose -f deployments/docker/docker-compose.yml"
COMPOSE ?= docker compose -f deployments/docker/docker-compose.yml

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[-a-zA-Z0-9_]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build binary for local OS
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

# Cross-compiled artifacts use CGO_ENABLED=0: mattn/go-sqlite3 and gojieba (CGO) are not linked.
# For a full-featured binary use `make build` on the target OS, use Docker, or PostgreSQL-only + no jieba paths.
build-linux: ## Build binary for Linux (amd64), CGO off (see Makefile header comment)
	@echo "Building for Linux amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

build-all: build-linux ## Build for all platforms (CGO off; same limits as build-linux)
	@echo "Building for all platforms..."
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

run: build ## Build and run locally
	$(BUILD_DIR)/$(BINARY_NAME) --config configs/config.yaml

dev: ## Run with hot reload (requires air)
	air -c .air.toml

test: ## Run all tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

fmt: ## Format code (module packages only)
	go fmt ./...
	@gofmt -s -w $$(go list -f '{{.Dir}}' ./...)

vet: ## Run go vet
	go vet ./...

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

DOCKERFILE=deployments/docker/Dockerfile

# Native image for the machine running Docker (amd64 Mac/PC → linux/amd64, Apple Silicon → linux/arm64).
docker-build: ## Build Docker image for host architecture (tags :VERSION and :latest)
	docker build -f $(DOCKERFILE) -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest --build-arg VERSION=$(VERSION) .

# Explicit architectures (requires Docker Buildx; cross-build may use QEMU — first run can be slow).
docker-build-amd64: ## Build linux/amd64 image (tags :VERSION-amd64 and :latest-amd64)
	docker buildx build --platform linux/amd64 --load -f $(DOCKERFILE) \
		-t $(DOCKER_IMAGE):$(VERSION)-amd64 -t $(DOCKER_IMAGE):latest-amd64 \
		--build-arg VERSION=$(VERSION) .

docker-build-arm64: ## Build linux/arm64 image (tags :VERSION-arm64 and :latest-arm64)
	docker buildx build --platform linux/arm64 --load -f $(DOCKERFILE) \
		-t $(DOCKER_IMAGE):$(VERSION)-arm64 -t $(DOCKER_IMAGE):latest-arm64 \
		--build-arg VERSION=$(VERSION) .

docker-build-both: docker-build-amd64 docker-build-arm64 ## Build both linux/amd64 and linux/arm64 images

docker-push: docker-build ## Push Docker image (:VERSION and :latest, host arch)
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

# Pushes separate per-arch tags, not a single multi-arch manifest list (for that, use buildx --platform a,b --push one tag).
docker-push-both: docker-build-both ## Push per-arch tags (:VERSION-amd64, :latest-amd64, :*-arm64)
	docker push $(DOCKER_IMAGE):$(VERSION)-amd64
	docker push $(DOCKER_IMAGE):latest-amd64
	docker push $(DOCKER_IMAGE):$(VERSION)-arm64
	docker push $(DOCKER_IMAGE):latest-arm64

docker-compose-up: ## Start stack (compose file under deployments/docker/)
	$(COMPOSE) up -d

docker-compose-down: ## Stop stack
	$(COMPOSE) down

docker-compose-logs: ## View tiersum service logs
	$(COMPOSE) logs -f tiersum

deps: ## Download and tidy dependencies
	go mod download
	go mod tidy
	go mod verify

fetch-onnxruntime: ## Vendor ONNX Runtime into third_party/ (MiniLM; optional, no OS package)
	bash scripts/fetch-onnxruntime.sh host

fetch-minilm: ## Download MiniLM ONNX + tokenizer into third_party/minilm/ (reproducible model files)
	bash scripts/fetch-minilm.sh

generate: ## Run go generate ./... (no-op until packages add //go:generate directives)
	go generate ./...

.DEFAULT_GOAL := help
