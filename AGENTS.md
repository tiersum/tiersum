<!-- From: /Users/luodaijun/GolandProjects/tiersum/AGENTS.md -->
# AGENTS.md — TierSum

## Project Overview

TierSum is a **Hierarchical Summary Knowledge Base** — a RAG-free document retrieval system powered by multi-layer abstraction and hot/cold document tiering. It uses a two-tier document storage strategy to balance LLM cost and query performance while preserving knowledge architecture through layered summarization.

**Key Characteristics:**
- **Language:** Go 1.23+ (backend), TypeScript/React (frontend)
- **Architecture:** 5-Layer Architecture with Interface+Impl Pattern
- **Database:** SQLite (default) or PostgreSQL (optional)
- **Search:** BM25 (Bleve) + Vector (HNSW) hybrid search for cold documents
- **LLM Integration:** OpenAI, Anthropic, or local (Ollama)
- **Protocol Support:** REST API + MCP (Model Context Protocol)
- **Frontend:** Next.js 14 with Slate dark theme

---

## Quick Commands

```bash
# Build server binary
make build                  # Output: ./build/tiersum
go build -o tiersum ./cmd   # Direct go build

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

---

## Technology Stack

### Backend (Go)
| Component | Technology |
|-----------|------------|
| Web Framework | Gin (`github.com/gin-gonic/gin`) |
| MCP Protocol | `github.com/mark3labs/mcp-go` |
| Search | Bleve v2 (BM25 text search) |
| Vector Search | HNSW (`github.com/coder/hnsw`) |
| Chinese Tokenization | gojieba (`github.com/yanyiwu/gojieba`) |
| Database | SQLite (`github.com/mattn/go-sqlite3`) or PostgreSQL |
| CLI | Cobra + Viper |
| Logging | Uber Zap |
| Testing | Testify |

### Frontend (Next.js)
| Component | Technology |
|-----------|------------|
| Framework | Next.js 16.2.2 (App Router) |
| Language | TypeScript 5 |
| React | React 19.2.4 |
| Styling | Tailwind CSS 4 |
| UI Components | shadcn/ui |
| Icons | Lucide React |

---

## Project Structure

```
cmd/
  main.go                    # API server entrypoint (single binary)
configs/
  config.example.yaml        # Configuration template
db/
  migrations/                # Database migration files
    001_initial_schema.sql
    002_topic_summaries.sql
deployments/
  docker/
    Dockerfile               # Multi-stage build (Go 1.23-alpine)
    docker-compose.yml       # SQLite default, optional PostgreSQL
internal/
  api/                       # Layer 1: API Layer
    handler.go               # REST API handlers
    handler_test.go          # Handler tests
    mcp.go                   # MCP protocol handlers
  service/                   # Layer 2: Service Layer
    interface.go             # I-prefixed service interfaces
    svcimpl/                 # Implementation subpackage
      document.go            # DocumentSvc implements IDocumentService
      query.go               # QuerySvc implements IQueryService
      tag_grouping.go        # TagGroupSvc implements ITagGroupService
      indexer.go             # IndexerSvc implements IIndexer
      summarizer.go          # SummarizerSvc implements ISummarizer
      quota.go               # QuotaManager for hot doc rate limiting
      *_test.go              # Unit tests with mocks
  storage/                   # Layer 3: Storage Layer
    interface.go             # I-prefixed storage interfaces
    db/                      # Database repository implementations
      repository.go          # DocumentRepo, SummaryRepo, TagRepo, TagGroupRepo
      schema.go              # Database schema definitions
      migrator.go            # Schema migration manager
    cache/                   # Cache implementation
      cache.go               # Cache implements ICache
    memory/                  # In-memory index for cold documents
      index.go               # BM25 + Vector hybrid index with Chinese support
  client/                    # Layer 4: Client Layer
    interface.go             # I-prefixed client interfaces
    llm/                     # LLM client implementations
      openai.go              # OpenAIProvider implements ILLMProvider
  job/                       # Job Layer (background tasks)
    scheduler.go             # Job scheduler
    jobs.go                  # IndexerJob, TagGroupJob
    promote_job.go           # Cold-to-hot promotion job
    hotscore_job.go          # Hot score recalculation job
  di/                        # Dependency Injection (composition root)
    container.go             # Wires all layers together
pkg/types/                   # Public API types
  document.go                # Document, Summary, Tag types
  query.go                   # Query request/response types
web/                         # Next.js frontend
  app/                       # App Router
    page.tsx                 # Query homepage with progressive search
    docs/                    # Document pages
    tags/                    # Tag browser
  components/ui/             # shadcn/ui components
  lib/api.ts                 # API client
  dist/                      # Static export output (for Gin hosting)
```

---

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

1. **Interface+Impl Pattern**: Each layer defines interfaces in `interface.go`, implementations in subpackage (`svcimpl/` for services)
2. **I-prefix Naming**: All interfaces start with I (e.g., `IDocumentService`, `ICache`, `ILLMProvider`)
3. **Layer owns interfaces**: No central ports package, each layer manages its own interfaces
4. **DI in di/**: All wiring happens in `internal/di/container.go`
5. **API unified**: REST and MCP handlers in same package (`internal/api/`)
6. **English Comments Only**: All code comments must be written in English

---

## Build Process

### Local Development
```bash
# Install dependencies
make deps

# Copy and configure
cp configs/config.example.yaml configs/config.yaml
# Edit configs/config.yaml and set OPENAI_API_KEY

# Build
make build

# Run with hot reload (requires air)
make dev
```

### Production Build
```bash
# Build for current platform
make build

# Build for all platforms (Linux, Darwin, Windows)
make build-all

# Docker build
make docker-build
```

### Frontend Build
```bash
cd web
npm install
npm run build  # Output: web/dist/ (served by Gin)
```

---

## Configuration

Key configuration file: `configs/config.yaml` (copy from `config.example.yaml`)

### Required Settings
```yaml
llm:
  provider: openai  # Options: openai, anthropic, local (ollama)
  openai:
    api_key: ${OPENAI_API_KEY}  # Required for openai provider
```

### LLM Provider Configuration

**OpenAI (default):**
```yaml
llm:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
    max_tokens: 2000
    temperature: 0.3
```

**Anthropic Claude:**
```yaml
llm:
  provider: anthropic
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    base_url: https://api.anthropic.com
    model: claude-3-haiku-20240307
    max_tokens: 2000
    temperature: 0.3
```

**Local/Ollama:**
```yaml
llm:
  provider: local  # or 'ollama'
  local:
    base_url: http://localhost:11434
    model: llama3.2
    timeout: 60s
```

**OpenAI-Compatible Providers:**
Any provider with OpenAI-compatible API can use `provider: openai` with custom `base_url`:

| Provider | base_url | Example Model |
|----------|----------|---------------|
| DeepSeek | `https://api.deepseek.com/v1` | `deepseek-chat` |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.1-8b-instant` |
| 智谱 AI (Zhipu) | `https://open.bigmodel.cn/api/paas/v4` | `glm-4-flash` |
| Moonshot (Kimi) | `https://api.moonshot.cn/v1` | `moonshot-v1-8k` |
| OpenRouter | `https://openrouter.ai/api/v1` | `anthropic/claude-3.5-haiku` |
| SiliconFlow | `https://api.siliconflow.cn/v1` | `deepseek-ai/DeepSeek-V2.5` |
| Azure OpenAI | `https://{resource}.openai.azure.com/openai/deployments/{deployment}` | deployment name |

Example for DeepSeek:
```yaml
llm:
  provider: openai
  openai:
    api_key: ${DEEPSEEK_API_KEY}
    base_url: https://api.deepseek.com/v1
    model: deepseek-chat
    max_tokens: 2000
    temperature: 0.3
```

### Key Configuration Sections
| Section | Purpose |
|---------|---------|
| `server` | HTTP port, CORS, timeouts |
| `llm` | OpenAI/Anthropic/Local provider settings |
| `storage.database` | SQLite (default) or PostgreSQL |
| `quota` | Hot document rate limiting (default: 100/hour) |
| `memory_index` | HNSW parameters for vector search |
| `documents.tiering` | Hot/cold thresholds |
| `mcp` | MCP protocol settings |

---

## Testing Strategy

### Running Tests
```bash
# Run all tests with coverage
make test

# Run specific package tests
go test -v ./internal/service/svcimpl/

# Run with race detection
go test -race ./...
```

### Test Structure
- Test files: `*_test.go` alongside source files
- Mock implementations in `internal/service/svcimpl/mocks_test.go`
- Uses `testify/assert` and `testify/require`
- Tests cover:
  - Hot/cold document tiering logic
  - Progressive query filtering
  - Tag grouping
  - Quota management
  - API handlers

### Adding New Tests
```go
func TestFeature(t *testing.T) {
    // Create mocks
    docRepo := NewMockDocumentRepository()
    
    // Create service with mocks
    svc := NewDocumentSvc(docRepo, ...)
    
    // Execute and assert
    result, err := svc.Method(ctx, req)
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

---

## Database Schema

### Documents Table
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key (UUID) |
| title | TEXT | Document title |
| content | TEXT | Full markdown content |
| format | TEXT | 'markdown' or 'md' |
| tags | TEXT[] | Document tags (hot docs) |
| status | TEXT | 'hot', 'cold', 'warming' |
| hot_score | REAL | Calculated hot score |
| query_count | INTEGER | Number of queries |
| last_query_at | TIMESTAMP | Last access time |
| created_at | TIMESTAMP | Creation time |

### Summaries Table
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| document_id | FK | Reference to documents |
| tier | TEXT | 'document', 'chapter', 'source' |
| path | TEXT | 'doc_id' or 'doc_id/chapter_title' |
| content | TEXT | Summary or source content |

### Global Tags & Tag Clusters
- `global_tags` - Level 2 tags with document counts
- `tag_clusters` - Level 1 groups (created by LLM clustering)

---

## Hot/Cold Document Tiering

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

---

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

---

## API Endpoints

### REST API
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/documents` | Ingest document |
| GET | `/api/v1/documents` | List documents |
| GET | `/api/v1/documents/:id` | Get document |
| GET | `/api/v1/documents/:id/summaries` | Get document summaries |
| GET | `/api/v1/query` | Legacy hierarchical query |
| POST | `/api/v1/query/progressive` | Progressive query (recommended) |
| GET | `/api/v1/tags` | List all tags |
| GET | `/api/v1/tags/groups` | List tag groups (L1) |
| POST | `/api/v1/tags/group` | Trigger tag grouping |
| GET | `/api/v1/quota` | Check quota status |
| GET | `/health` | Health check |

### MCP Endpoints
| Path | Description |
|------|-------------|
| `/mcp/sse` | MCP Server-Sent Events endpoint |

---

## Development Conventions

### Code Style
- **Go**: Standard Go formatting (`go fmt`)
- **Imports**: Grouped - standard lib, third-party, internal
- **Comments**: English only, complete sentences
- **Error Handling**: Explicit error returns, wrapped with context

### Interface Pattern
```go
// In interface.go
type IMyService interface {
    DoSomething(ctx context.Context) error
}

// In svcimpl/my_service.go
type MySvc struct{}

func (s *MySvc) DoSomething(ctx context.Context) error { ... }

// Compile-time check
var _ service.IMyService = (*MySvc)(nil)
```

### Adding New Features
1. **Define interface** in layer's `interface.go`
2. **Implement** in subpackage (e.g., `service/svcimpl/`)
3. **Wire** in `di/container.go`
4. **Add tests** in `*_test.go`

---

## Security Considerations

- API key authentication supported (optional, via config)
- JWT token authentication supported
- CORS configuration for web UI
- No sensitive data in logs (use zap logging)
- Database credentials via environment variables
- LLM API keys via environment variables

---

## Deployment

### Docker (Recommended)
```bash
cd deployments/docker
docker-compose up -d
```

Default setup uses SQLite with volume-mounted data directory.

### Environment Variables
| Variable | Required | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | Yes* | OpenAI API key |
| `ANTHROPIC_API_KEY` | Yes* | Anthropic API key (alternative) |
| `JWT_SECRET` | No | JWT signing secret |

*At least one LLM provider key required

---

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
| `internal/di/container.go` | Dependency injection / composition root |
| `internal/client/llm/factory.go` | LLM provider factory (dynamic selection) |
| `internal/client/llm/openai.go` | OpenAI provider implementation |
| `internal/client/llm/anthropic.go` | Anthropic Claude provider implementation |
| `internal/client/llm/ollama.go` | Local Ollama provider implementation |
| `internal/job` | Background scheduled tasks |
| `web/` | Next.js frontend |
| `web/dist/` | Static export for Gin hosting |
| `db/migrations/` | Database migration files |
| `pkg/types` | Public types used across all layers |

---

## Troubleshooting

### Common Issues

**Build fails with SQLite error:**
- Ensure CGO is enabled: `CGO_ENABLED=1`
- On macOS: `brew install sqlite3`

**Frontend not loading:**
- Check `web/dist/` exists after `npm run build`
- Verify `server.web_dir` in config points to correct path

**MCP connection issues:**
- Verify `mcp.enabled: true` in config
- Check `/mcp/sse` endpoint is accessible

**Cold document search not working:**
- Check memory index loaded on startup (see startup logs)
- Verify documents have `status: cold` in database
