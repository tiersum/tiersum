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

# Frontend (Next.js)
cd web && npm install       # Install dependencies
cd web && npm run dev       # Development server
cd web && npm run build     # Build static files to web/dist/
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
    003_topic_source.sql
    004_add_is_source.sql
    005_tag_clustering.sql
    006_hot_cold_tier.sql
    007_add_embedding.sql
internal/
  api/                 # Layer 1: API Layer
    handler.go         # REST API handlers
    mcp.go             # MCP protocol handlers
  service/             # Layer 2: Service Layer
    interface.go       # I-prefixed service interfaces
    svcimpl/           # Implementation subpackage
      document.go      # DocumentSvc implements IDocumentService
      query.go         # QuerySvc implements IQueryService
      tag_grouping.go  # TagGroupSvc implements ITagGroupService
      indexer.go       # IndexerSvc implements IIndexer
      summarizer.go    # SummarizerSvc implements ISummarizer
      quota.go         # QuotaManager for hot doc rate limiting
  storage/             # Layer 3: Storage Layer
    interface.go       # I-prefixed storage interfaces
    db/                # Database repository implementations
      repository.go    # DocumentRepo, SummaryRepo, TagRepo, TagGroupRepo
      schema.go        # Database schema definitions
      migrator.go      # Schema migration manager
    cache/             # Cache implementation
      cache.go         # Cache implements ICache
    memory/            # In-memory index for cold documents
      index.go         # BM25 + Vector hybrid index with snippet extraction
  client/              # Layer 4: Client Layer
    interface.go       # I-prefixed client interfaces
    llm/               # LLM client implementations
      openai.go        # OpenAIProvider implements ILLMProvider
  job/                 # Job Layer (background tasks)
    scheduler.go       # Job scheduler
    jobs.go            # IndexerJob, TagGroupJob
    promote_job.go     # Cold-to-hot promotion job
    hotscore_job.go    # Hot score recalculation job
  di/                  # Dependency Injection (composition root)
    container.go       # Wires all layers together
web/                   # Next.js frontend (Next.js 14 + shadcn/ui)
  app/                 # App Router
    page.tsx           # Query homepage with progressive search
    docs/              # Document pages
    tags/              # Tag browser
  components/ui/       # shadcn/ui components
  lib/api.ts           # API client
  dist/                # Static export output (for Gin hosting)
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
6. **English Comments Only**: All code comments must be written in English

## Hot/Cold Document Tiering

TierSum uses a two-tier document storage strategy to balance LLM cost and query performance:

### Document Status

- **`hot`** - Full LLM analysis, tagged, indexed with summaries (requires quota)
- **`cold`** - Minimal processing, stored in memory index with BM25 + vector search
- **`warming`** - Being promoted from cold to hot (async process)

### Hot Document Criteria

A document becomes hot when:
1. Quota available (default 100/hour)
2. AND one of:
   - `force_hot=true` in request
   - Has pre-built summary/chapters
   - Content length > 5000 characters

### Cold Document Flow

```
Ingest (cold)
    ↓
Generate simple embedding (char n-gram hash)
    ↓
Add to Memory Index (Bleve + HNSW)
    ↓
Query via BM25 + Vector hybrid search
    ↓
Extract keyword-based snippets
    ↓
If query_count >= 3 → PromoteQueue
    ↓
PromoteJob (every 5 min) → Full LLM analysis → hot
```

### Memory Index (internal/storage/memory/index.go)

**Components:**
- **Bleve** - BM25 text search with TF-IDF scoring
- **HNSW** - Vector similarity search (384 dimensions, cosine distance)
- **Hybrid Search** - Combines BM25 (50%) + Vector (50%) scores

**Snippet Extraction Algorithm:**
1. **Keyword positioning** - Extract top 10 keywords from query, locate all positions
2. **Context window** - Take 200 chars before/after each keyword match
3. **Deduplication & merging** - Merge overlapping snippets (threshold: 50 chars)
4. **Max snippets** - Return up to 3 merged snippets per document

```go
const (
    ContextWindowSize = 200   // Characters before/after keyword
    MaxSnippetLength  = 500   // Maximum snippet length
    MaxSnippetsPerDoc = 3     // Maximum snippets per document
    MergeDistance     = 50    // Distance threshold for merging
)
```

## Two-Level Tag-Based Progressive Query

The system uses a hierarchical tag structure for document organization and retrieval:

### Tag Hierarchy

```
Level 1: Tag Groups (created by LLM clustering)
    ├── "Cloud Native"
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
Step 1: L2 Tag Filtering (adaptive)
    │──▶ If tag count < 200: Direct L2 filter with LLM
    │──▶ If tag count >= 200: L1 → L2 two-level filter
    │       • LLM selects 1-3 relevant L1 groups
    │       • Collect L2 tags from selected groups
    │       • LLM filters L2 tags (relevance >= 0.5)
    │
    ▼
Step 2: Query & Filter Documents
    │──▶ Query documents by L2 tags (OR logic)
    │──▶ Separate hot/cold documents
    │──▶ Hot docs: LLM filter (relevance >= 0.5)
    │──▶ Cold docs: Keyword matching
    │
    ▼
Step 3: Query & Filter Chapters
    │──▶ Get chapters for hot docs from summary repo
    │──▶ Create pseudo-chapters for cold docs (snippets)
    │──▶ LLM filter chapters (relevance >= 0.5)
    │
    ▼
Step 4: Cold Path (parallel)
    │──▶ BM25 + Vector hybrid search on cold docs
    │──▶ Extract keyword-based snippets
    │
    ▼
Step 5: Merge Results
    │──▶ Combine hot and cold results
    │──▶ Deduplicate by document ID
    │──▶ Boost relevance if found in both paths
    │──▶ Sort by relevance, return top K
    │
    ▼
Step 6: Track Access
    └──▶ Increment query_count
        └──▶ If cold doc queried 3+ times → PromoteQueue
```

### Tag Grouping Job

The `TagGroupJob` runs every 30 minutes to:
1. Check if grouping is needed (tag count changed or 30 min elapsed)
2. Use LLM to group all L2 tags into 3-10 L1 groups (target: ~10 tags per group)
3. Clear existing groups, create new ones
4. Update group assignments in database
5. Log refresh metrics

## Job Layer (Background Tasks)

| Job | File | Interval | Purpose |
|-----|------|----------|---------|
| **IndexerJob** | `jobs.go` | 1 minute | Processes pending document indexing |
| **TagGroupJob** | `jobs.go` | 30 minutes | LLM-based tag clustering into L1 groups |
| **PromoteJob** | `promote_job.go` | 5 minutes | Promotes cold docs (query_count > 3) to hot |
| **HotScoreJob** | `hotscore_job.go` | 1 hour | Updates hot scores: `query_count / (1 + hours_since_last_query)` |

**Scheduler** (`scheduler.go`):
- Ticks at shortest job interval
- Tracks last execution per job
- 5-minute timeout per job

## Interface Definitions

### Service Layer Interfaces (`internal/service/interface.go`)

```go
// Business Logic
IDocumentService interface {
    // Ingest processes and stores a new document
    // Automatically generates tags, summary, and chapter summaries
    // Uses hot/cold tiering based on quota and content size
    Ingest(ctx context.Context, req types.CreateDocumentRequest) (*types.CreateDocumentResponse, error)
    // Get retrieves a document by ID
    Get(ctx context.Context, id string) (*types.Document, error)
}

IQueryService interface {
    // Query performs hierarchical query with LLM filtering (legacy)
    Query(ctx context.Context, question string, depth types.SummaryTier) ([]types.QueryResult, error)
    // ProgressiveQuery performs the new two-level tag-based progressive query
    // Combines hot path (LLM filtering) and cold path (BM25 + vector search)
    ProgressiveQuery(ctx context.Context, req types.ProgressiveQueryRequest) (*types.ProgressiveQueryResponse, error)
}

ITagGroupService interface {
    // ClusterTags performs LLM-based clustering of all global tags
    // Creates Level 1 clusters from Level 2 tags
    ClusterTags(ctx context.Context) error
    // ShouldRefresh checks if clustering should be performed
    ShouldRefresh(ctx context.Context) (bool, error)
    // GetL1Groups retrieves all Level 1 clusters
    GetL1Groups(ctx context.Context) ([]types.TagGroup, error)
    // GetL2TagsByGroup retrieves Level 2 tags belonging to a group
    GetL2TagsByGroup(ctx context.Context, groupID string) ([]types.Tag, error)
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
    // GroupTags performs LLM-based clustering of tags into groups
    GroupTags(ctx context.Context, tags []string) (*types.TagGroupingResult, error)
}
```

### Storage Layer Interfaces (`internal/storage/interface.go`)

```go
IDocumentRepository interface {
    Create(ctx context.Context, doc *types.Document) error
    GetByID(ctx context.Context, id string) (*types.Document, error)
    // ListByTags retrieves documents that match ANY of the given tags (OR logic)
    ListByTags(ctx context.Context, tags []string, limit int) ([]types.Document, error)
    // ListByStatus retrieves documents by status (hot/cold/warming)
    ListByStatus(ctx context.Context, status types.DocumentStatus, limit int) ([]types.Document, error)
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
    ListByGroup(ctx context.Context, groupID string) ([]types.Tag, error)
    IncrementDocumentCount(ctx context.Context, tagName string) error
    DeleteAll(ctx context.Context) error
    GetCount(ctx context.Context) (int, error)
}

ITagGroupRepository interface {
    Create(ctx context.Context, group *types.TagGroup) error
    GetByID(ctx context.Context, id string) (*types.TagGroup, error)
    List(ctx context.Context) ([]types.TagGroup, error)
    DeleteAll(ctx context.Context) error
    GetCount(ctx context.Context) (int, error)
}

ITagGroupRefreshLogRepository interface {
    Create(ctx context.Context, tagCountBefore, tagCountAfter, groupCount int, durationMs int64) error
    GetLastRefresh(ctx context.Context) (*TagGroupRefreshLog, error)
}

ICache interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{})
}

// IInMemoryIndex provides BM25 + Vector hybrid search for cold documents
type IInMemoryIndex interface {
    AddDocument(doc *types.Document, embedding []float32) error
    RemoveDocument(docID string) error
    Search(ctx context.Context, queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error)
    SearchWithBleve(queryText string, topK int) ([]SearchResult, error)
    SearchWithVector(queryEmbedding []float32, topK int, queryText string) ([]SearchResult, error)
    HybridSearch(queryText string, queryEmbedding []float32, topK int) ([]SearchResult, error)
    RebuildFromDocuments(ctx context.Context, docs []types.Document, getEmbedding func(doc *types.Document) []float32) error
    Close() error
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
- **Document**: Document-level summary (max 300 chars)
- **Chapter**: Section/chapter summary + source content
- **Source**: Original content stored separately

### Two-Level Tag System
- **Level 1 (Tag Groups)**: High-level categories created by LLM grouping (3-10 groups)
- **Level 2 (Tags)**: Individual tags extracted from documents (max 10 per doc)

### Hot/Cold Tiering
- **Hot**: Full LLM analysis with tiered summaries, tag-based retrieval
- **Cold**: BM25 + Vector hybrid search with keyword-based snippet extraction
- **Promotion**: Automatic upgrade after 3+ queries

### LLM-Powered Features
- **Auto-generated tags**: Documents get tags via LLM analysis if not provided
- **Document analysis**: Summary + tags + chapter summaries in single prompt
- **Tag grouping**: Automatic clustering of related tags into L1 categories
- **Progressive filtering**: LLM filters tags → documents → chapters at each step

### Hybrid Search (Cold Documents)
- **BM25**: Keyword-based text search using Bleve
- **Vector**: Semantic similarity using HNSW (384-dim embeddings)
- **Hybrid**: Combined scoring (50/50 weight) with deduplication
- **Snippets**: Keyword-based context extraction (200 char window)

### Dual API
- **REST API**: `/api/v1/*` for HTTP clients
- **MCP API**: `/mcp/sse` for Model Control Protocol (tools for agents)

### Web UI (Next.js 14)
- **Query Page**: Progressive search with split-panel results
- **Document Page**: Metadata, tags, chapter navigation, code highlighting
- **Tag Browser**: Two-level tag navigation (L1 groups, L2 tags)
- **Tech Stack**: Next.js 14 + TypeScript + Tailwind CSS + shadcn/ui
- **Theme**: Slate dark theme, exported to `web/dist/` for Gin hosting

## Key Dependencies

- **Web**: Gin (`github.com/gin-gonic/gin`)
- **MCP**: `github.com/mark3labs/mcp-go`
- **Search**: Bleve (`github.com/blevesearch/bleve/v2`) for BM25
- **Vector**: HNSW (`github.com/chewxy/hnsw`) for vector similarity
- **SQLite**: `github.com/mattn/go-sqlite3` (default)
- **Postgres**: `github.com/jackc/pgx/v5` (optional)
- **Markdown**: `github.com/yuin/goldmark`
- **CLI**: `github.com/spf13/cobra` + `github.com/spf13/viper`
- **Jobs**: Internal scheduler with configurable intervals
- **Frontend**: Next.js 14, React 19, Tailwind CSS 4, shadcn/ui

## Configuration

- Copy `configs/config.example.yaml` → `configs/config.yaml`
- Required env vars: `OPENAI_API_KEY` or `ANTHROPIC_API_KEY`
- Optional: `JWT_SECRET`

### Key Configuration Options

```yaml
server:
  port: 8080
  web_dir: "./web/dist"  # Static files for frontend

llm:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
    temperature: 0.3

quota:
  per_hour: 100  # Max hot documents per hour

documents:
  tiering:
    hot_content_threshold: 5000  # Min chars for hot tier
    cold_promotion_threshold: 3  # Query count for promotion

memory_index:
  vector_dimension: 384
  hnsw_m: 16
  hnsw_ef_construction: 200
  hnsw_ef_search: 100
```

## Database Schema Versions

| Version | Name | Key Changes |
|---------|------|-------------|
| 1 | Initial schema | documents, summaries tables |
| 2 | Topic summaries | Added topic_summaries, topic_documents |
| 3 | Topic source | Added source column |
| 4 | Hierarchy flags | Added is_source flag |
| 5 | Tag clustering | Added global_tags, tag_clusters; removed topics |
| 6 | Hot/cold tiering | Added status, hot_score, query_count, last_query_at |
| 7 | Embeddings | Added embedding column for vector search |

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
- **Frontend**: Build with `cd web && npm run build`, output to `web/dist/`

## Important Paths

| Path | Purpose |
|------|---------|
| `cmd/main.go` | Main entrypoint for server binary |
| `internal/api` | REST + MCP API handlers |
| `internal/service/interface.go` | Service layer interfaces (I-prefix) |
| `internal/service/svcimpl` | Service implementations |
| `internal/storage/interface.go` | Storage interfaces (I-prefix) |
| `internal/storage/db` | Repository implementations |
| `internal/storage/memory/index.go` | BM25 + Vector index with snippet extraction |
| `internal/storage/db/schema.go` | Database schema definitions |
| `internal/client/interface.go` | Client interfaces (I-prefix) |
| `internal/di` | Dependency injection / composition root |
| `internal/job` | Background scheduled tasks |
| `web/` | Next.js frontend |
| `web/dist/` | Static export for Gin hosting |
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
- `internal/storage/memory/` - In-memory BM25 + vector index
- `internal/client/interface.go` - Client interfaces (I-prefix)
- `internal/di/` - Dependency injection
- `internal/api/` - Unified API layer (REST + MCP)
- `web/` - Next.js frontend
