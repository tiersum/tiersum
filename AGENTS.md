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
cd deployments/docker && docker-compose up -d  # Starts tiersum (SQLite by default)
```

## Project Structure

Go module: `github.com/tiersum/tiersum` (Go 1.23+)

### Clean Architecture Layers

```
cmd/
  server/         # API server entrypoint (main binary)
  worker/         # Background job processor
  cli/            # CLI tools
configs/          # Configuration files
deployments/
  docker/         # Docker and docker-compose files
internal/
  ports/          # INTERFACE DEFINITIONS (all layers depend on this)
    interfaces.go # Repository, Service, Core interfaces
  adapters/
    repository/   # Repository implementations (Database access)
    llm/          # LLM provider implementations
  domain/
    service/      # Business logic implementations
    core/         # Core domain services (Parser, Summarizer, Indexer)
  api/            # REST handlers (depends on service interfaces)
  app/            # Dependency injection / Composition root
  storage/        # SQLite/PostgreSQL + in-memory cache
  mcp/            # MCP protocol implementation
pkg/types/        # Public API types
```

## Architecture Principles

### Dependency Direction
All dependencies point **inward** toward the domain:

```
API Layer (handlers)
    ↓ depends on
Service Layer (business logic)
    ↓ depends on
Repository Layer (data access)
    ↓ depends on
Infrastructure (db, cache, llm)

ALL layers depend on:
    ports/ (interface definitions)
```

### Key Rules
1. **Ports define interfaces** - All interfaces in `internal/ports/`
2. **Adapters implement** - Concrete implementations in `internal/adapters/`
3. **Domain is isolated** - Business logic only depends on ports
4. **API depends on services** - Handlers use service interfaces, not repositories directly
5. **Composition in app/** - All wiring happens in `internal/app/wire.go`

## Interface Definitions (`internal/ports/`)

### Repository Interfaces (Data Access)
```go
DocumentRepository interface {
    Create(ctx context.Context, doc *types.Document) error
    GetByID(ctx context.Context, id string) (*types.Document, error)
}

SummaryRepository interface {
    Create(ctx context.Context, summary *types.Summary) error
    GetByDocument(ctx context.Context, docID string) ([]types.Summary, error)
}
```

### Service Interfaces (Business Logic)
```go
DocumentService interface {
    Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.Document, error)
    Get(ctx context.Context, id string) (*types.Document, error)
}

QueryService interface {
    Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error)
}
```

### Core Interfaces (Domain Logic)
```go
Parser interface {
    Parse(content string) (*types.ParsedDocument, error)
}

Summarizer interface {
    Summarize(ctx context.Context, content string, level types.SummaryTier) (string, error)
}

Indexer interface {
    Index(ctx context.Context, docID string, content string) error
}
```

## Key Dependencies

- **Web**: Gin (`github.com/gin-gonic/gin`)
- **MCP**: `github.com/mark3labs/mcp-go`
- **SQLite**: `github.com/mattn/go-sqlite3` (default)
- **Postgres**: `github.com/jackc/pgx/v5` (optional)
- **Markdown**: `github.com/yuin/goldmark`
- **CLI**: `github.com/spf13/cobra` + `github.com/spf13/viper`

## Configuration

- Copy `configs/config.example.yaml` → `configs/config.yaml`
- Required env vars: `OPENAI_API_KEY` or `ANTHROPIC_API_KEY`
- Optional: `JWT_SECRET`

## Architecture Notes

- **Dual API**: REST (`/api/v1/*`) + MCP SSE (`/mcp/sse`)
- **4-Tier Summary**: Document → Chapter → Paragraph → Source
- **Storage**: SQLite/PostgreSQL (docs + hierarchy), in-memory cache
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
| `internal/ports` | Interface definitions (dependency inversion) |
| `internal/adapters/repository` | Repository implementations |
| `internal/domain/service` | Business logic implementations |
| `internal/domain/core` | Core domain services |
| `internal/app` | Dependency injection / composition root |
| `pkg/types` | Public types used across all layers |
