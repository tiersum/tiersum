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

### 5-Layer Architecture with Interface+Impl Pattern

```
cmd/
  main.go              # API server entrypoint (main binary)
configs/               # Configuration files
deployments/
  docker/              # Docker and docker-compose files
db/
  migrations/          # Database migration files
    001_initial_schema.sql
    002_topic_summaries.sql
internal/
  api/                 # Layer 1: API Layer
    handler.go         # REST API handlers
    mcp.go             # MCP protocol handlers
  service/             # Layer 2: Service Layer
    interface.go       # I-prefixed service interfaces
    svcimpl/           # Implementation subpackage
      document.go      # DocumentSvc implements IDocumentService
      query.go         # QuerySvc implements IQueryService
      topic.go         # TopicSvc implements ITopicService
      indexer.go       # IndexerSvc, SummarizerSvc, ParserSvc
  storage/             # Layer 3: Storage Layer
    interface.go       # I-prefixed storage interfaces
    db/                # Database repository implementations
      repository.go    # DocumentRepo, SummaryRepo, TopicSummaryRepo
      schema.go        # Database schema definitions
      migrator.go      # Schema migration manager
    cache/             # Cache implementation
      cache.go         # Cache implements ICache
  client/              # Layer 4: Client Layer
    interface.go       # I-prefixed client interfaces
    llm/               # LLM client implementations
      openai.go        # OpenAIProvider implements ILLMProvider
  job/                 # Job Layer (background tasks)
    scheduler.go       # Job scheduler
    jobs.go            # IndexerJob, TopicAggregatorJob, CacheCleanupJob
  di/                  # Dependency Injection (composition root)
    container.go       # Wires all layers together
pkg/types/             # Public API types
```

## Architecture Principles

### 5-Layer Dependency Direction

```
Layer 1: API Layer (internal/api/)
    ↓ uses
Layer 2: Service Layer (internal/service/)
    ↓ uses
Layer 3: Storage Layer (internal/storage/)
    ↓ uses
Layer 4: Client Layer (internal/client/)

Job Layer can use: Service Layer, Storage Layer
```

### Key Rules

1. **Interface+Impl Pattern**: Each layer defines interfaces in `interface.go`, implementations in subpackage
2. **I-prefix Naming**: All interfaces start with I (e.g., `IDocumentService`, `ICache`, `ILLMProvider`)
3. **Layer owns interfaces**: No central ports package, each layer manages its own interfaces
4. **DI in di/**: All wiring happens in `internal/di/container.go`
5. **API unified**: REST and MCP handlers in same package (`internal/api/`)

## Automatic Topic Aggregation

Documents are automatically organized into topics based on tag overlap:

### Flow
```
Document Ingest
    │
    ├─► LLM generates tags (if not provided)
    │
    ├─► Document saved to DB
    │
    ├─► Async: Generate 4-tier summaries (Document→Chapter→Paragraph)
    │
    └─► Async: Match to existing topics by tag overlap (2+ matching tags)
            │
            ▼
    TopicAggregatorJob (runs every 5 min)
            │
            ├─► Scan topics with few documents
            │
            └─► Create new topics from documents sharing common tags
```

### Tag Matching Rules
- A document is added to a topic if they share **2+ tags** (or 1 if document has only 1 tag)
- Matching happens asynchronously after document ingestion
- Duplicate document entries in same topic are prevented

## Interface Definitions

### Service Layer Interfaces (`internal/service/interface.go`)

```go
// Business Logic
IDocumentService interface {
    Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.Document, error)
    Get(ctx context.Context, id string) (*types.Document, error)
}

IQueryService interface {
    Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error)
}

ITopicService interface {
    CreateTopicFromDocuments(ctx context.Context, topicName string, docIDs []string) (*types.TopicSummary, error)
    GetTopic(ctx context.Context, id string) (*types.TopicSummary, error)
    ListTopics(ctx context.Context) ([]types.TopicSummary, error)
    FindTopicsByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error)
    // Auto-matching: adds document to topics with overlapping tags
    AddDocumentToTopics(ctx context.Context, docID string, docTags []string) (int, error)
    AutoCreateTopicFromTag(ctx context.Context, tag string, minDocs int) (*types.TopicSummary, error)
}

// Core Domain Logic (in service/svcimpl/)
IIndexer interface {
    Index(ctx context.Context, docID string, content string) error
}

ISummarizer interface {
    Summarize(ctx context.Context, content string, level types.SummaryTier) (string, error)
    AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error)
    GenerateTopicSummary(ctx context.Context, topicName string, documents []*types.Document) (*types.TopicSummary, error)
}

IParser interface {
    Parse(content string) (*types.ParsedDocument, error)
}
```

### Storage Layer Interfaces (`internal/storage/interface.go`)

```go
IDocumentRepository interface {
    Create(ctx context.Context, doc *types.Document) error
    GetByID(ctx context.Context, id string) (*types.Document, error)
}

ISummaryRepository interface {
    Create(ctx context.Context, summary *types.Summary) error
    GetByDocument(ctx context.Context, docID string) ([]types.Summary, error)
}

ITopicSummaryRepository interface {
    Create(ctx context.Context, topic *types.TopicSummary) error
    GetByID(ctx context.Context, id string) (*types.TopicSummary, error)
    List(ctx context.Context) ([]types.TopicSummary, error)
    FindByTags(ctx context.Context, tags []string) ([]types.TopicSummary, error)
    AddDocument(ctx context.Context, topicID string, docID string) error
    RemoveDocument(ctx context.Context, topicID string, docID string) error
}

ICache interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{})
}
```

### Client Layer Interfaces (`internal/client/interface.go`)

```go
ILLMProvider interface {
    Generate(ctx context.Context, prompt string, maxTokens int) (string, error)
}
```

## Key Features

### 5-Tier Summary Hierarchy
- **Topic**: Cross-document theme summary (highest level)
- **Document**: Document-level summary
- **Chapter**: Section/chapter summary
- **Paragraph**: Paragraph summary
- **Source**: Original content

### LLM-Powered Features
- **Auto-generated tags**: Documents get tags via LLM analysis if not provided
- **Document analysis**: Summary + tags + topic + key points
- **Topic synthesis**: Multi-document theme summary generation

### Dual API
- **REST API**: `/api/v1/*` for HTTP clients
- **MCP API**: `/mcp/sse` for Model Control Protocol

## Key Dependencies

- **Web**: Gin (`github.com/gin-gonic/gin`)
- **MCP**: `github.com/mark3labs/mcp-go`
- **SQLite**: `github.com/mattn/go-sqlite3` (default)
- **Postgres**: `github.com/jackc/pgx/v5` (optional)
- **Markdown**: `github.com/yuin/goldmark`
- **CLI**: `github.com/spf13/cobra` + `github.com/spf13/viper`
- **Jobs**: Internal scheduler with configurable intervals

## Configuration

- Copy `configs/config.example.yaml` → `configs/config.yaml`
- Required env vars: `OPENAI_API_KEY` or `ANTHROPIC_API_KEY`
- Optional: `JWT_SECRET`

## Adding New Features

1. **Define interface** in layer's `interface.go`:
```go
type IAnalyzer interface {
    Analyze(ctx context.Context, doc *types.Document) error
}
```

2. **Implement** in subpackage (e.g., `service/impl/`):
```go
type AnalyzerSvc struct{}

func (a *AnalyzerSvc) Analyze(...) error { ... }

var _ service.IAnalyzer = (*AnalyzerSvc)(nil)  // Compile-time check
```

3. **Wire** in `di/container.go`:
```go
analyzer := impl.NewAnalyzerSvc()
deps := &Dependencies{ Analyzer: analyzer, ... }
```

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
| `cmd/main.go` | Main entrypoint for server binary |
| `internal/api` | REST + MCP API handlers |
| `internal/service/interface.go` | Service layer interfaces (I-prefix) |
| `internal/service/svcimpl` | Service implementations |
| `internal/storage/interface.go` | Storage interfaces (I-prefix) |
| `internal/storage/db` | Repository implementations |
| `internal/storage/db/schema.go` | Database schema definitions |
| `internal/client/interface.go` | Client interfaces (I-prefix) |
| `internal/di` | Dependency injection / composition root |
| `internal/job` | Background scheduled tasks |
| `db/migrations/` | Database migration files |
| `pkg/types` | Public types used across all layers |

## Architecture Evolution

### Previous Structure (Clean Architecture)
- `internal/ports/` - Central interface definitions
- `internal/adapters/` - Repository implementations
- `internal/domain/service/` - Business logic
- `internal/domain/core/` - Core domain logic
- `internal/app/` - Dependency injection
- `internal/mcp/` - MCP handlers

### Current Structure (Interface+Impl Pattern)
- `cmd/main.go` - Single entry point
- `db/migrations/` - Database migrations
- `internal/service/interface.go` - Service interfaces (I-prefix)
- `internal/service/svcimpl/` - Service implementations
- `internal/storage/interface.go` - Storage interfaces (I-prefix)
- `internal/storage/db/` - Repository implementations + schema
- `internal/client/interface.go` - Client interfaces (I-prefix)
- `internal/di/` - Dependency injection
- `internal/api/` - Unified API layer (REST + MCP)
