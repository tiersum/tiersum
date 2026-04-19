# syntax=docker/dockerfile:1
# TierSum API server — multi-stage build. Intended for native linux/amd64 or linux/arm64
# (CI uses one runner per arch — no QEMU, no CGO cross-compiler).
# Runtime: ONNX + MiniLM via make fetch-* (same as local dev).

FROM golang:1.24-bookworm AS builder

WORKDIR /app
ENV GOTOOLCHAIN=auto
ENV CGO_ENABLED=1

ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
		git \
		ca-certificates \
		gcc \
		g++ \
		libc6-dev \
		libsqlite3-dev \
	&& rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
# go.mod replace github.com/google/renameio => ./pkg/patchrenameio must exist before go mod download
COPY pkg/patchrenameio ./pkg/patchrenameio
RUN go mod download

COPY . .

ARG VERSION=dev
RUN GOOS=linux GOARCH="${TARGETARCH}" \
	go build -trimpath \
	-ldflags="-w -s -X main.Version=${VERSION}" \
	-o /out/tiersum \
	./cmd

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

# Copy binary and config
COPY --from=builder /out/tiersum ./tiersum
COPY --from=builder /app/configs/config.example.yaml ./config.yaml

# Copy gojieba dictionary files required for Chinese tokenization at runtime
COPY --from=builder /go/pkg/mod/github.com/yanyiwu/gojieba@*/deps/cppjieba/dict /app/deps/cppjieba/dict
ENV GOJIEA_DICT_PATH=/app/deps/cppjieba/dict

RUN ARCH="$(uname -m)"; \
	case "$ARCH" in \
		x86_64) SUB=linux_amd64 ;; \
		aarch64) SUB=linux_arm64 ;; \
		*) echo "unsupported architecture: ${ARCH}"; exit 1 ;; \
	esac; \
	sed -i -E "s|^([[:space:]]*onnx_runtime_path:).*|\1 third_party/onnxruntime/${SUB}/lib/libonnxruntime.so|" /app/config.yaml

EXPOSE 8080

CMD ["./tiersum", "--config", "config.yaml"]
