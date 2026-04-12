# TierSum API server — multi-stage build.
#
# Cold embeddings (same as local dev):
#   - Runtime image runs `make fetch-onnxruntime` + `make fetch-minilm` (scripts + URLs match repo root).
#   - ONNX lands under third_party/onnxruntime/linux_{amd64,arm64}/ (see scripts/fetch-onnxruntime.sh).
#   - MiniLM under third_party/minilm/ (see scripts/fetch-minilm.sh).
#   - config.yaml is patched so onnx_runtime_path matches the container CPU (linux_amd64 vs linux_arm64).
#
# Build args (optional, same env vars the shell scripts honor):
#   ONNXRUNTIME_VERSION — default must stay aligned with scripts/fetch-onnxruntime.sh
#   MINILM_HF_REPO      — Hugging Face repo id for fetch-minilm.sh
#
# Debian bookworm (glibc): official ONNX Linux builds are glibc-linked; Alpine/musl would not load them.

# -----------------------------------------------------------------------------
# Build stage (CGO on: go-sqlite3, gojieba, onnxruntime_go need a C/C++ toolchain in-image)
# -----------------------------------------------------------------------------
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# go.mod may require a newer toolchain than the image default
ENV GOTOOLCHAIN=auto
ENV CGO_ENABLED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    gcc \
    g++ \
    libc6-dev \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY ../deployments/docker .

ARG VERSION=dev
# Native linux/${TARGETARCH} inside this stage (matches docker --platform); do not use CGO_ENABLED=0 cross-builds for this binary.
RUN GOOS=linux go build -trimpath \
    -ldflags="-w -s -X main.Version=${VERSION}" \
    -o /out/tiersum \
    ./cmd

# -----------------------------------------------------------------------------
# Runtime stage
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim

WORKDIR /app

ARG ONNXRUNTIME_VERSION=1.24.1
ENV ONNXRUNTIME_VERSION=${ONNXRUNTIME_VERSION}

ARG MINILM_HF_REPO=sentence-transformers/all-MiniLM-L6-v2
ENV MINILM_HF_REPO=${MINILM_HF_REPO}

COPY Makefile ./Makefile
COPY scripts/fetch-onnxruntime.sh scripts/fetch-minilm.sh ./scripts/

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    make \
    libsqlite3-0 \
    libstdc++6 \
    && make fetch-onnxruntime \
    && make fetch-minilm \
    && apt-get purge -y curl make \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /app/data

COPY --from=builder /out/tiersum ./tiersum
COPY --from=builder /app/configs/config.example.yaml ./config.yaml

# config.example defaults to linux_amd64; align path with the image architecture.
RUN ARCH="$(uname -m)"; \
    case "$ARCH" in \
      x86_64) SUB=linux_amd64 ;; \
      aarch64) SUB=linux_arm64 ;; \
      *) echo "unsupported architecture: ${ARCH}"; exit 1 ;; \
    esac; \
    sed -i -E "s|^([[:space:]]*onnx_runtime_path:).*|\1 third_party/onnxruntime/${SUB}/lib/libonnxruntime.so|" /app/config.yaml

EXPOSE 8080

CMD ["./tiersum", "--config", "config.yaml"]
