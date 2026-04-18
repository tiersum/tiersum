# Installation Guide

## Prerequisites

- **Go 1.23+** (with CGO enabled for SQLite)
- **Database**: SQLite (default, zero-config) or PostgreSQL (optional)
- **LLM API Key**: OpenAI, Anthropic, or compatible provider

## Quick Install

```bash
# Clone repository
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# Install Go dependencies
make deps

# Copy and edit configuration
cp configs/config.example.yaml configs/config.yaml

# Set required environment variables
export OPENAI_API_KEY="your-api-key"
# or
export ANTHROPIC_API_KEY="your-api-key"

# Build (includes embedded frontend)
make build
```

## Docker Compose (Recommended for Production)

```bash
cd deployments/docker && docker-compose up -d
```

Default setup uses SQLite with volume-mounted data directory. See `deployments/docker/README.md` for pre-built image from Alibaba ACR.

## Cold Document Embeddings (Optional)

Semantic vectors for the **cold** index use **all-MiniLM-L6-v2** ONNX files on disk plus the **ONNX Runtime** shared library:

```bash
make fetch-onnxruntime   # ONNX .so / .dylib per platform
make fetch-minilm        # model.onnx + tokenizer.json from Hugging Face
```

If MiniLM fails to load and `cold_index.embedding.provider` is `auto`, TierSum falls back to simple hash embeddings.

See [third_party/onnxruntime/README.md](../../third_party/onnxruntime/README.md) and [third_party/minilm/README.md](../../third_party/minilm/README.md).

## Start Server

```bash
# Run locally (backend + embedded frontend)
make run

# Or run binary directly
./build/tiersum --config configs/config.yaml

# Server ready
# Web UI:   http://localhost:8080/
# REST API: http://localhost:8080/api/v1
# BFF:      http://localhost:8080/bff/v1
# Health:   http://localhost:8080/health
# Metrics:  http://localhost:8080/metrics
# MCP SSE:  http://localhost:8080/mcp/sse
```

## First Launch (Bootstrap)

1. Open the web UI (`http://localhost:8080/`). If uninitialized, you are redirected to **`/init`**.
2. Submit **Initialize** with the desired **admin username**.
3. The response shows, **once**:
   - **Admin access token** (`ts_u_…`) — for browser login
   - **Initial API key** (`tsk_live_…`) — for scripts and automation

Store these safely; they cannot be retrieved again.

See [Auth and Permissions](../design/auth-and-permissions.md) for the full dual-track auth design.
