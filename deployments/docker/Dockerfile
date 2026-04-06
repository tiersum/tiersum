# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies (including gcc for SQLite CGO)
RUN apk add --no-cache git make gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary (CGO enabled for SQLite)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o tiersum ./cmd/server

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS and sqlite libs
RUN apk --no-cache add ca-certificates libstdc++

# Create data directory for SQLite
RUN mkdir -p /app/data

# Copy binary from builder
COPY --from=builder /app/tiersum .

# Copy default config
COPY --from=builder /app/configs/config.example.yaml ./config.yaml

# Expose port
EXPOSE 8080

# Run
CMD ["./tiersum", "--config", "config.yaml"]
