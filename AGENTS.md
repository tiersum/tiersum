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
      tag_clustering.go # TagGroupSvc implements ITagGroupService
      indexer.go       # IndexerSvc implements IIndexer
      summarizer.go    # SummarizerSvc implements ISummarizer
  storage/             # Layer 3: Storage Layer
    interface.go       # I-prefixed storage interfaces
    db/                # Database repository implementations
      repository.go    # DocumentRepo, SummaryRepo, TagRepo, TagGroupRepo, ClusterRefreshLogRepo
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
    jobs.go            # IndexerJob, TagGroupJob
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

## Two-Level Tag-Based Progressive Query

The system uses a hierarchical tag structure for document organization and retrieval:

### Tag Hierarchy

```
Level 1: Tag Groups (Clusters)
    ├── "Cloud Native" (cluster)
    │       ├── Level 2: kubernetes
    │       ├── Level 2: docker
    │       └── Level 2: helm
    ├── "Programming Languages"
    │       ├── Level 2: golang
    │       ├── Level 2: python
    │       └── Level 2: rust
    └── ...
```

### Progressive Query Flow

```
User Query
    │
    ▼
Step 1: Filter L2 Tags (via LLM)
    │──▶ Input: Query + All L2 Tags
    │──▶ Output: Relevant L2 Tags (relevance >= 0.5)
    │
    ▼
Step 2: Query & Filter Documents (via LLM)
    │──▶ Input: Query + Documents matching L2 Tags
    │──▶ Output: Relevant Documents (relevance >= 0.5)
    │
    ▼
Step 3: Query & Filter Chapters (via LLM)
    │──▶ Input: Query + Chapters from filtered docs
    │──▶ Output: Relevant Chapters (relevance >= 0.5)
    │
    ▼
Step 4: Build Results
    └──▶ Return: QueryItem list with paths and content
```

### Tag Clustering Job

The `TagGroupJob` runs every 30 minutes to:
1. Check if clustering is needed (tag count changed or 30 min elapsed)
2. Use LLM to cluster all L2 tags into L1 groups
3. Update cluster assignments in database
4. Log refresh metrics

## Interface Definitions

### Service Layer Interfaces (`internal/service/interface.go`)

```go
// Business Logic
IDocumentService interface {
    // Ingest processes and stores a new document
    // Automatically generates tags, summary, and chapter summaries
    Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error)
    // Get retrieves a document by ID
    Get(ctx context.Context, id string) (*types.Document, error)
}

IQueryService interface {
    // Query performs hierarchical query with LLM filtering (legacy)
    Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error)
    // ProgressiveQuery performs the new two-level tag-based progressive query
    ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error)
}

ITagGroupService interface {
    // ClusterTags performs LLM-based clustering of all global tags
    // Creates Level 1 clusters from Level 2 tags
    ClusterTags(ctx context.Context) error
    // ShouldRefresh checks if clustering should be performed
    ShouldRefresh(ctx context.Context) (bool, error)
    // GetL1Clusters retrieves all Level 1 clusters
    GetL1Clusters(ctx context.Context) ([]types.TagGroup, error)
    // GetL2TagsByCluster retrieves Level 2 tags belonging to a cluster
    GetL2TagsByCluster(ctx context.Context, clusterID string) ([]types.Tag, error)
    // FilterL2TagsByQuery uses LLM to filter L2 tags based on query
    FilterL2TagsByQuery(ctx context.Context, query string, tags []types.Tag) ([]types.TagFilterResult, error)
}

// Core Domain Logic (in service/svcimpl/)
IIndexer interface {
    // Index processes and indexes a document
    // Creates document summary, chapter summaries, and stores source content
    Index(ctx context.Context, doc *types.Document, analysis *types.DocumentAnalysisResult) error
}

ISummarizer interface {
    // AnalyzeDocument performs full document analysis
    // Returns document summary, tags (max 10), and chapter summaries
    AnalyzeDocument(ctx context.Context, title string, content string) (*types.DocumentAnalysisResult, error)
    // FilterDocuments selects relevant documents based on query
    FilterDocuments(ctx context.Context, query string, docs []types.Document) ([]types.LLMFilterResult, error)
    // FilterChapters selects relevant chapters based on query
    FilterChapters(ctx context.Context, query string, chapters []types.Summary) ([]types.LLMFilterResult, error)
}
```

### Storage Layer Interfaces (`internal/storage/interface.go`)

```go
IDocumentRepository interface {
    Create(ctx context.Context, doc *types.Document) error
    GetByID(ctx context.Context, id string) (*types.Document, error)
    // ListByTags retrieves documents that match ANY of the given tags (OR logic)
    ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error)
}

ISummaryRepository interface {
    Create(ctx context.Context, summary *types.Summary) error
    GetByDocument(ctx context.Context, docID string) ([]types.Summary, error)
    // GetByPath retrieves a summary by its exact path
    GetByPath(ctx context.Context, path string) (*types.Summary, error)
    // QueryByTierAndPrefix queries summaries by tier and path prefix
    QueryByTierAndPrefix(ctx context.Context, tier types.SummaryTier, pathPrefix string) ([]types.Summary, error)
    // DeleteByDocument removes all summaries for a document
    DeleteByDocument(ctx context.Context, docID string) error
}

ITagRepository interface {
    Create(ctx context.Context, tag *types.Tag) error
    GetByName(ctx context.Context, name string) (*types.Tag, error)
    List(ctx context.Context) ([]types.Tag, error)
    ListByCluster(ctx context.Context, clusterID string) ([]types.Tag, error)
    IncrementDocumentCount(ctx context.Context, tagName string) error
    DeleteAll(ctx context.Context) error
    GetCount(ctx context.Context) (int, error)
}

ITagGroupRepository interface {
    Create(ctx context.Context, cluster *types.TagGroup) error
    GetByID(ctx context.Context, id string) (*types.TagGroup, error)
    List(ctx context.Context) ([]types.TagGroup, error)
    DeleteAll(ctx context.Context) error
    GetCount(ctx context.Context) (int, error)
}

IClusterRefreshLogRepository interface {
    Create(ctx context.Context, tagCountBefore, tagCountAfter, clusterCount int, durationMs int64) error
    GetLastRefresh(ctx context.Context) (*ClusterRefreshLog, error)
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

### 3-Tier Summary Hierarchy
- **Document**: Document-level summary
- **Chapter**: Section/chapter summary
- **Source**: Original content

### Two-Level Tag System
- **Level 1 (Tag Groups)**: High-level categories created by LLM clustering
- **Level 2 (Tags)**: Individual tags extracted from documents

### LLM-Powered Features
- **Auto-generated tags**: Documents get tags via LLM analysis if not provided
- **Document analysis**: Summary + tags + chapter summaries
- **Tag clustering**: Automatic grouping of related tags into categories
- **Progressive filtering**: LLM filters tags → documents → chapters at each step

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

2. **Implement** in subpackage (e.g., `service/svcimpl/`):
```go
type AnalyzerSvc struct{}

func (a *AnalyzerSvc) Analyze(...) error { ... }

var _ service.IAnalyzer = (*AnalyzerSvc)(nil)  // Compile-time check
```

3. **Wire** in `di/container.go`:
```go
analyzer := svcimpl.NewAnalyzerSvc()
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
