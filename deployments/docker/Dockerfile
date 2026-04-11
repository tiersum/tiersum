# TierSum API server — multi-stage build.
#
# ONNX Runtime + MiniLM ONNX (cold embeddings):
#   - Runtime stage installs Linux ORT under /usr/local/lib and downloads HF MiniLM into /app/third_party/minilm/.
#   - Local dev: `make fetch-onnxruntime` + `make fetch-minilm` (see configs/config.example.yaml defaults).
#   - Keep ARG ONNXRUNTIME_VERSION aligned with scripts/fetch-onnxruntime.sh (default 1.19.2).
#
# Debian bookworm (glibc): official ONNX Linux builds are glibc-linked; Alpine/musl would not load them.

# -----------------------------------------------------------------------------
# Build stage
# -----------------------------------------------------------------------------
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# go.mod may require a newer toolchain than the image default
ENV GOTOOLCHAIN=auto

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -o /out/tiersum \
    ./cmd

# -----------------------------------------------------------------------------
# Runtime stage
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim

WORKDIR /app

# Pin a release that exists on GitHub (see microsoft/onnxruntime releases).
ARG ONNXRUNTIME_VERSION=1.19.2

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libsqlite3-0 \
    libstdc++6 \
    && set -eux; \
    ARCH="$(uname -m)"; \
    case "$ARCH" in \
      x86_64) ORT_ARCH=x64 ;; \
      aarch64|arm64) ORT_ARCH=aarch64 ;; \
      *) echo "unsupported architecture ${ARCH}"; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/ort.tgz \
      "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-${ORT_ARCH}-${ONNXRUNTIME_VERSION}.tgz"; \
    tar xf /tmp/ort.tgz -C /tmp; \
    cp "/tmp/onnxruntime-linux-${ORT_ARCH}-${ONNXRUNTIME_VERSION}/lib"/libonnxruntime.so* /usr/local/lib/; \
    ldconfig; \
    rm -rf /tmp/ort.tgz "/tmp/onnxruntime-linux-${ORT_ARCH}-${ONNXRUNTIME_VERSION}"; \
    mkdir -p /app/third_party/minilm; \
    curl -fsSL -o /app/third_party/minilm/model.onnx -H "Accept: application/octet-stream" \
      "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"; \
    curl -fsSL -o /app/third_party/minilm/tokenizer.json -H "Accept: application/octet-stream" \
      "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"; \
    apt-get purge -y curl; \
    apt-get autoremove -y; \
    rm -rf /var/lib/apt/lists/*

# onnxruntime_go / MiniLM loader (optional; config can override)
ENV ONNXRUNTIME_LIB_PATH=/usr/local/lib/libonnxruntime.so

RUN mkdir -p /app/data

COPY --from=builder /out/tiersum ./tiersum
COPY --from=builder /app/configs/config.example.yaml ./config.yaml
# config.example defaults to third_party/onnxruntime/linux_amd64/...; image installs ORT under /usr/local/lib.
RUN sed -i -E 's|^([[:space:]]*onnx_runtime_path:).*|\1 /usr/local/lib/libonnxruntime.so|' /app/config.yaml

EXPOSE 8080

CMD ["./tiersum", "--config", "config.yaml"]
