# AGENTS.md — TierSum

## Quick Commands

```bash
# Build server binary
make build                  # Output: ./build/tiersum
go build -o tiersum ./cmd/server  # Direct go build

# Development
make run                    # Build + run with config.yaml
make dev                    # Hot reload (requires `air`)

# Testing & Quality
make test                   # Run tests with race detection + coverage
make lint                   # Requires golangci-lint
make fmt                    # go fmt + gofmt -s
make vet                    # go vet ./...

# Database
make migrate-up             # Run migrations
make migrate-down           # Rollback migrations
make seed                   # Seed sample data

# Docker Compose (full stack)
cd deployments/docker && docker-compose up -d  # Starts tiersum + postgres
```

## Project Structure

Go module: `github.com/tiersum/tiersum` (Go 1.23+)

```
cmd/
  server/         # API server entrypoint (main binary)
  worker/         # Background job processor
  cli/            # CLI tools
  migrate/        # Database migrations
  seed/           # Data seeding
configs/          # Configuration files
  config.example.yaml
  config.yaml
deployments/
  docker/         # Docker and docker-compose files
internal/
  api/            # REST handlers + MCP server
  core/parser/    # Markdown parser (Goldmark)
  core/summarizer/# LLM abstraction
  core/indexer/   # Hierarchical index builder
  storage/        # PostgreSQL + in-memory cache
  mcp/            # MCP protocol implementation
pkg/types/        # Public API types
skills/           # OpenClaw skill definitions
migrations/       # Database migrations
```

## Key Dependencies

- **Web**: Gin (`github.com/gin-gonic/gin`)
- **MCP**: `github.com/mark3labs/mcp-go`
- **Postgres**: `github.com/jackc/pgx/v5`
- **Markdown**: `github.com/yuin/goldmark`
- **CLI**: `github.com/spf13/cobra` + `github.com/spf13/viper`

## Configuration

- Copy `configs/config.example.yaml` → `configs/config.yaml`
- Required env vars: `OPENAI_API_KEY` or `ANTHROPIC_API_KEY`
- Optional: `JWT_SECRET`

## Architecture Notes

- **Dual API**: REST (`/api/v1/*`) + MCP SSE (`/mcp/sse`)
- **4-Tier Summary**: Document → Chapter → Paragraph → Source
- **Storage**: PostgreSQL (docs + hierarchy), in-memory cache
- **Parser**: Goldmark for Markdown → heading hierarchy

## Build Targets

```bash
make build-linux            # Linux amd64
make build-all              # Linux + Darwin (amd64/arm64) + Windows
make docker-build           # Build container image
```

## Development Notes

- **Hot reload**: Install `air` first (`go install github.com/air-verse/air@latest`)
- **Linting**: Install `golangci-lint` separately
- **DB Migrations**: Use `make migrate-up` / `make migrate-down`
- **Dependencies**: `make deps` runs `go mod download/tidy/verify`

## Important Paths

| Path | Purpose |
|------|---------|
| `cmd/server` | Main entrypoint for server binary |
| `internal/api` | REST + MCP handlers |
| `internal/core` | Business logic (parser, summarizer, indexer) |
| `migrations/` | SQL migrations (if using migrate CLI) |
| `skills/` | OpenClaw skill YAML definitions |
