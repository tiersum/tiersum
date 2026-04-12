.PHONY: build build-linux build-linux-amd64 build-linux-arm64 \
	build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-all \
	release-assets release-pack release-clean \
	test run dev clean lint fmt vet help \
	deps generate fetch-onnxruntime fetch-minilm \
	docker-build docker-build-amd64 docker-build-arm64 docker-build-both \
	docker-push docker-push-both \
	docker-compose-up docker-compose-down docker-compose-logs

# Variables
BINARY_NAME=tiersum
BUILD_DIR=./build
DIST_DIR=./dist
CMD_DIR=./cmd
DOCKER_IMAGE=tiersum/tiersum
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
# For archive names: prefer GitHub Actions tag (e.g. v1.2.3), else VERSION.
RELEASE_VERSION ?= $(if $(GITHUB_REF_NAME),$(GITHUB_REF_NAME),$(VERSION))
RELEASE_STAMP := $(shell echo "$(RELEASE_VERSION)" | tr '/' '-')
LDFLAGS=-ldflags "-w -s -X main.Version=$(VERSION)"

UNAME_S := $(shell uname -s 2>/dev/null || echo unknown)
ifeq ($(UNAME_S),Darwin)
SHA256_CMD = shasum -a 256
else
SHA256_CMD = sha256sum
endif

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
build-linux-amd64: ## Build Linux amd64 binary (CGO off; see Makefile header comment)
	@echo "Building $(BINARY_NAME) for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

build-linux-arm64: ## Build Linux arm64 binary (CGO off)
	@echo "Building $(BINARY_NAME) for linux/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)

build-darwin-amd64: ## Build macOS amd64 binary (CGO off)
	@echo "Building $(BINARY_NAME) for darwin/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)

build-darwin-arm64: ## Build macOS arm64 binary (CGO off)
	@echo "Building $(BINARY_NAME) for darwin/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

build-windows-amd64: ## Build Windows amd64 binary (CGO off)
	@echo "Building $(BINARY_NAME) for windows/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

build-linux: build-linux-amd64 ## Alias: Linux amd64 (backward compatible)

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 ## All release platforms (CGO off)
	@echo "Built cross-platform binaries under $(BUILD_DIR)/"

# GitHub Release: tar.gz / zip per OS+arch + SHA256SUMS in $(DIST_DIR)/
release-clean: ## Remove $(DIST_DIR) (release archives)
	rm -rf $(DIST_DIR)

# Pack pre-built binaries from $(BUILD_DIR) (tiersum-linux-amd64, …, tiersum-windows-amd64.exe). Used by CI after per-OS CGO builds.
release-pack: ## Create $(DIST_DIR) archives + SHA256SUMS from existing $(BUILD_DIR) binaries
	@echo "Packing release archives (stamp: $(RELEASE_STAMP))..."
	@mkdir -p $(DIST_DIR)
	@set -e; \
	REL="$(RELEASE_STAMP)"; \
	ST="$(DIST_DIR)/.stage"; \
	for spec in linux:amd64:$(BINARY_NAME)-linux-amd64 \
	            linux:arm64:$(BINARY_NAME)-linux-arm64 \
	            darwin:amd64:$(BINARY_NAME)-darwin-amd64 \
	            darwin:arm64:$(BINARY_NAME)-darwin-arm64; do \
	  os=$${spec%%:*}; rest=$${spec#*:}; arch=$${rest%%:*}; bin=$${rest#*:}; \
	  rm -rf "$$ST"; mkdir -p "$$ST"; \
	  cp "$(BUILD_DIR)/$$bin" "$$ST/$(BINARY_NAME)"; \
	  tar -czf "$(DIST_DIR)/$(BINARY_NAME)_$${REL}_$${os}_$${arch}.tar.gz" -C "$$ST" "$(BINARY_NAME)"; \
	done; \
	rm -rf "$$ST"; mkdir -p "$$ST"; \
	cp "$(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe" "$$ST/$(BINARY_NAME).exe"; \
	( cd "$$ST" && zip -q "../$(BINARY_NAME)_$${REL}_windows_amd64.zip" "$(BINARY_NAME).exe" ); \
	rm -rf "$$ST"; \
	( cd "$(DIST_DIR)" && $(SHA256_CMD) $(BINARY_NAME)_"$${REL}"_*.tar.gz $(BINARY_NAME)_"$${REL}"_*.zip > SHA256SUMS ); \
	echo "Release assets:"; ls -1 "$(DIST_DIR)"

release-assets: release-clean build-all release-pack ## Cross-build (CGO off) + pack for GitHub Assets (local only; CI uses release-pack)

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
	rm -rf $(BUILD_DIR) $(DIST_DIR)
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
