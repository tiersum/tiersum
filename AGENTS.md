

# AGENTS.md — TierSum

## Project Overview

TierSum is a **Hierarchical Summary Knowledge Base** — a RAG-free document retrieval system powered by multi-layer abstraction and hot/cold document tiering. It uses a two-tier document storage strategy to balance LLM cost and query performance while preserving knowledge architecture through layered summarization.

**Key Characteristics:**

- **Language:** Go 1.23+ (backend), Vue 3 CDN (frontend)
- **Architecture:** 5-Layer Architecture with Interface+Impl Pattern
- **Database:** SQLite (default) or PostgreSQL (optional)
- **Search:** BM25 (Bleve) + Vector (HNSW) hybrid search for cold documents (vectors: on-disk MiniLM ONNX + ONNX Runtime; see `make fetch-minilm` / `make fetch-onnxruntime`)
- **LLM Integration:** OpenAI, Anthropic, or local (Ollama)
- **Protocol Support:** REST API + MCP (Model Context Protocol)
- **Frontend:** Vue 3 + Tailwind CSS + DaisyUI (CDN-based, no Node.js required)

---

## Quick Commands

```bash
# Build server binary
make build                  # Output: ./build/tiersum
go build -o tiersum ./cmd   # Direct go build

# Development
make run                    # Build + run with config.yaml
make dev                    # Hot reload (requires `air`)
make fetch-onnxruntime      # Download ONNX Runtime into third_party/ (MiniLM without OS install)
make fetch-minilm           # Download MiniLM ONNX + tokenizer into third_party/minilm/ (reproducible)

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

---

## Technology Stack

### Backend (Go)


| Component            | Technology                                           |
| -------------------- | ---------------------------------------------------- |
| Web Framework        | Gin (`github.com/gin-gonic/gin`)                     |
| MCP Protocol         | `github.com/mark3labs/mcp-go`                        |
| Search               | Bleve v2 (BM25 text search)                          |
| Vector Search        | HNSW (`github.com/coder/hnsw`)                       |
| Chinese Tokenization | gojieba (`github.com/yanyiwu/gojieba`)               |
| Database             | SQLite (`github.com/mattn/go-sqlite3`) or PostgreSQL |
| CLI                  | Cobra + Viper                                        |
| Logging              | Uber Zap                                             |
| Testing              | Testify                                              |


### Frontend (CDN-based)


| Component     | Technology                    |
| ------------- | ----------------------------- |
| Framework     | Vue 3 (CDN)                   |
| Router        | Vue Router 4 (CDN)            |
| Styling       | Tailwind CSS (CDN)            |
| UI Components | DaisyUI (CDN)                 |
| Markdown      | Marked.js (CDN)               |
| Deployment    | Go embed (embedded in binary) |


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
    Dockerfile               # Multi-stage build (Go + Debian bookworm, ONNX Runtime for MiniLM)
    docker-compose.yml       # SQLite default, optional PostgreSQL
internal/
  api/                       # Layer 1: API Layer
    handler.go               # REST handlers (depends on service.* only, no storage repos)
    handler_test.go          # Handler tests
    mcp.go                   # MCP protocol handlers
  service/                   # Layer 2: Service Layer
    interface.go             # I-prefixed service interfaces (includes IRetrievalService for API read paths)
    errors.go                # Shared service errors (e.g. cold index unavailable)
    svcimpl/                 # Implementation subpackage
      document.go            # DocumentSvc implements IDocumentService
      retrieval.go           # RetrievalSvc implements IRetrievalService (tags/summaries/hot/cold reads for HTTP)
      query.go               # QuerySvc implements IQueryService
      tag_grouping.go        # TagGroupSvc implements ITagGroupService
      document_maintenance.go # DocumentMaintenanceSvc implements IDocumentMaintenanceService (jobs)
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
    coldindex/               # Cold document index: chapter split, BM25 (Bleve), vectors (HNSW), cold text embedders
      index.go               # storage.IColdIndex impl (documents + text Search); SetTextEmbedder optional for internal vectors
      chapter_split.go       # IColdChapterSplitter, MarkdownSplitter, IColdTextEmbedder contract
      coldvec/               # Deterministic hash embedding fallback (no imports from service/api)
      inverted_bleve.go      # Bleve BM25 cold chapter index
      vector_hnsw.go         # HNSW vector cold chapter index
      embed_*.go             # MiniLM / simple cold embeddings; NewTextEmbedderFromViper
  client/                    # Layer 4: Client Layer (third-party systems, e.g. LLM APIs)
    interface.go             # I-prefixed client interfaces (e.g. ILLMProvider)
    llm/                     # LLM client implementations (OpenAI, Anthropic, Ollama)
      openai.go              # OpenAIProvider implements ILLMProvider
  job/                       # Job Layer (background tasks; depends on internal/service only)
    scheduler.go             # Job scheduler
    jobs.go                  # TagGroupJob
    promote_job.go           # Schedules IDocumentMaintenanceService.RunColdPromotionSweep
    promote_consumer.go      # Async queue → PromoteColdDocumentByID
    hotscore_job.go          # Schedules RecalculateDocumentHotScores
  di/                        # Dependency Injection (composition root)
    container.go             # Wires all layers together
pkg/types/                   # Public API types + shared cold-embedding constants
  document.go                # Document, Summary, Tag types
  query.go                   # Query request/response types
  cold_embedding.go          # ColdEmbeddingVectorDimension, DefaultColdChapterMaxTokens
cmd/web/                     # Vue 3 CDN frontend (embedded in binary)
  index.html                 # HTML shell + importmap; loads `js/main.js` (ESM)
  js/                        # ES modules: `main.js`, `api_client.js`, `pages/`, `components/`
  FRONTEND.md                # Frontend stack, routes, **Web UI ↔ REST API** table
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
Layer 4: Client Layer (internal/client/) — third-party APIs (e.g. LLM); not cold-index embeddings

Job Layer (same dependency rule as API): Service Layer only (`internal/service`, e.g. `ITagGroupService`, `IDocumentMaintenanceService`)
```

### Key Rules

1. **Interface+Impl Pattern**: Each layer defines interfaces in `interface.go`, implementations in subpackage (`svcimpl/` for services)
2. **I-prefix Naming**: All interfaces start with I (e.g., `IDocumentService`, `ICache`, `ILLMProvider`)
3. **Layer owns interfaces**: No central ports package, each layer manages its own interfaces
4. **DI in di/**: All wiring happens in `internal/di/container.go`
5. **API unified**: REST and MCP handlers in same package (`internal/api/`)
6. **English Comments Only**: All code comments must be written in English
7. **Strict layer boundaries**: Upper layers (**`internal/api`**, **`internal/job`**) may depend only on **`internal/service`** **interfaces** (`interface.go` and the same package’s contracts) and neutral **`pkg/types`** — not on **`internal/storage`**, **`internal/service/svcimpl`**, **`internal/storage/db`**, or **`internal/client`**. Service code uses storage and client **only through** `internal/storage/interface.go` and `internal/client/interface.go`. **`.cursor/rules/layer-dependencies.mdc`** summarizes allowed edges and forbidden cross-layer shortcuts (concrete repos, bypassing DI).

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
# Build for current platform (includes embedded frontend)
make build

# Build for all platforms (Linux, Darwin, Windows)
make build-all

# Docker build
make docker-build
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


| Provider        | base_url                                                              | Example Model                |
| --------------- | --------------------------------------------------------------------- | ---------------------------- |
| DeepSeek        | `https://api.deepseek.com/v1`                                         | `deepseek-chat`              |
| Groq            | `https://api.groq.com/openai/v1`                                      | `llama-3.1-8b-instant`       |
| Zhipu AI (GLM)  | `https://open.bigmodel.cn/api/paas/v4`                                | `glm-4-flash`                |
| Moonshot (Kimi) | `https://api.moonshot.cn/v1`                                          | `moonshot-v1-8k`             |
| OpenRouter      | `https://openrouter.ai/api/v1`                                        | `anthropic/claude-3.5-haiku` |
| SiliconFlow     | `https://api.siliconflow.cn/v1`                                       | `deepseek-ai/DeepSeek-V2.5`  |
| Azure OpenAI    | `https://{resource}.openai.azure.com/openai/deployments/{deployment}` | deployment name              |


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


| Section             | Purpose                                        |
| ------------------- | ---------------------------------------------- |
| `server`            | HTTP port, CORS, timeouts                      |
| `llm`               | OpenAI/Anthropic/Local provider settings       |
| `storage.database`  | SQLite (default) or PostgreSQL                 |
| `quota`             | Hot document rate limiting (default: 100/hour) |
| `cold_index`        | Cold index: markdown chapter split (`markdown.*`), hybrid search pool (`search.branch_recall_*`), vector-branch embedder (`embedding.*`) |
| `documents.tiering` | Hot/cold thresholds                            |
| `mcp`               | MCP protocol settings                          |


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


| Column        | Type      | Description              |
| ------------- | --------- | ------------------------ |
| id            | TEXT      | Primary key (UUID)       |
| title         | TEXT      | Document title           |
| content       | TEXT      | Full markdown content    |
| format        | TEXT      | 'markdown' or 'md'       |
| tags          | TEXT[]    | Document tags (hot docs) |
| status        | TEXT      | 'hot', 'cold', 'warming' |
| hot_score     | REAL      | Calculated hot score     |
| query_count   | INTEGER   | Number of queries        |
| last_query_at | TIMESTAMP | Last access time         |
| created_at    | TIMESTAMP | Creation time            |


### Summaries Table


| Column      | Type | Description                        |
| ----------- | ---- | ---------------------------------- |
| id          | TEXT | Primary key                        |
| document_id | FK   | Reference to documents             |
| tier        | TEXT | 'document', 'chapter', 'source'    |
| path        | TEXT | 'doc_id' or 'doc_id/chapter_title' |
| content     | TEXT | Summary or source content          |


### Global Tags & Tag Groups

- `global_tags` - Level 2 tags with document counts
- `tag_groups` - Level 1 groups (created by LLM-based tag grouping)

---

## Hot/Cold Document Tiering

### Document Status

- `**hot**` - Full LLM analysis, tagged, indexed with summaries (requires quota)
- `**cold**` - Minimal processing, indexed in cold index (BM25 + vector search)
- `**warming**` - Being promoted from cold to hot (async process)

### Hot vs cold on ingest (`ingest_mode`)

Request body `ingest_mode` (default **auto** when omitted; legacy `force_hot=true` equals **hot**):

- **`hot`** — always use the hot ingest path (async LLM + summaries when needed).
- **`cold`** — always use the cold ingest path (no internal LLM on ingest).
- **`auto`** — hot if pre-built summary+chapters exist, else if hourly quota allows and content length > threshold (default 5000 chars), else cold.

### Cold Document Flow

```
Ingest (cold)
    ↓
Text embedding (`cold_index.embedding`: disk MiniLM via `minilm_model_path` / `fetch-minilm` + ONNX Runtime, else simple hash when MiniLM unavailable)
    ↓
Markdown chapter split (`coldindex.IColdChapterSplitter` in `internal/storage/coldindex`, token budget `cold_index.markdown.chapter_max_tokens`)
    ↓
Add to cold index (Bleve + HNSW) — one row per cold chapter (`document_id` + `path`)
    ↓
Query via BM25 + Vector hybrid search
    ↓
Return matching chapter full text (no keyword snippet)
    ↓
If query_count >= 3 → PromoteQueue
    ↓
PromoteJob (every 5 min) → Full LLM analysis → hot
```

---

## Job Layer (Background Tasks)


| Job             | File              | Interval   | Purpose                                                          |
| --------------- | ----------------- | ---------- | ---------------------------------------------------------------- |
| **TagGroupJob** | `jobs.go`         | 30 minutes | LLM-based tag grouping into L1 groups                            |
| **PromoteJob**  | `promote_job.go`  | 5 minutes  | Delegates to `IDocumentMaintenanceService.RunColdPromotionSweep` (cold→hot when `query_count` ≥ threshold) |
| **HotScoreJob** | `hotscore_job.go` | 1 hour     | Delegates to `IDocumentMaintenanceService.RecalculateDocumentHotScores` |


**Scheduler** (`scheduler.go`):

- Ticks at shortest job interval
- Tracks last execution per job
- 5-minute timeout per job

---

## API Endpoints

### Core API flows (algorithms)

Endpoints that are more than simple CRUD — ingest tiering, progressive query, tag grouping, hot/cold retrieval, hybrid cold search — are documented in **[docs/CORE_API_FLOWS.md](docs/CORE_API_FLOWS.md)** (call chain from REST handlers into services and storage).

### REST API

The **embedded Vue UI** (`cmd/web/js/`) uses a subset of these routes; see **`cmd/web/FRONTEND.md`** → *Web UI ↔ REST API* for the mapping by screen.

| Method | Endpoint                          | Description                                                                           |
| ------ | --------------------------------- | ------------------------------------------------------------------------------------- |
| POST   | `/api/v1/documents`               | Ingest document                                                                       |
| GET    | `/api/v1/documents`               | List documents                                                                        |
| GET    | `/api/v1/documents/:id`           | Get document                                                                          |
| GET    | `/api/v1/documents/:id/summaries` | Get document summaries                                                                |
| GET    | `/api/v1/documents/:id/chapters`  | List chapter summaries for a document                                                 |
| POST   | `/api/v1/query/progressive`       | Progressive query (recommended)                                                       |
| GET    | `/api/v1/tags`                    | List tags (optional `group_ids=comma&max_results=100`)                                |
| GET    | `/api/v1/tags/groups`             | List tag groups (L1)                                                                  |
| GET    | `/api/v1/hot/doc_summaries`       | Hot/warming docs matching `tags`; document-level summary only (`tags`, `max_results`) |
| GET    | `/api/v1/hot/doc_chapters`        | Chapter summaries for `doc_ids` (comma-separated, `max_results` caps doc count)       |
| GET    | `/api/v1/hot/doc_source`          | Original text for `chapter_paths` (comma-separated, `max_results`)                    |
| GET    | `/api/v1/cold/doc_source`         | Cold chapter hits via cold index (`q` comma-separated terms, `max_results`; JSON includes `path` per chapter) |
| POST   | `/api/v1/tags/group`              | Trigger tag grouping                                                                  |
| GET    | `/api/v1/quota`                   | Check quota status                                                                    |
| GET    | `/api/v1/monitoring`              | JSON monitoring snapshot (version, document counts, cold index size, quota)         |
| GET    | `/api/v1/metrics`                 | Prometheus metrics                                                                    |
| GET    | `/health`                         | Health check                                                                          |


### MCP Endpoints


| Path           | Description                                         |
| -------------- | --------------------------------------------------- |
| `/mcp/sse`     | MCP SSE stream (session + endpoint event)           |
| `/mcp/message` | MCP JSON-RPC message POST (`sessionId` query param) |


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
5. **Core API docs:** If the change affects a **non–simple-CRUD** API (multi-step logic, LLM, tiering, hybrid search, tag grouping, hot/cold retrieval), update **`docs/CORE_API_FLOWS.md`** in the same PR/commit. Cursor rule: `.cursor/rules/core-api-flows-doc.mdc`.
6. **AGENTS.md edits:** When changing **`AGENTS.md`**, in the same pass **strengthen Architecture-related sections** (`## Project Structure`, `## Architecture Principles`, and aligned topics such as hot/cold tiering or jobs) so structure, layers, and cross-links stay accurate. Cursor rule: `.cursor/rules/agents-architecture.mdc`.

---

## Security Considerations

- Optional REST API key: set `security.api_key`; clients send `X-API-Key` or `Authorization: Bearer <key>`. MCP `/mcp/`* routes are not protected by this middleware.
- JWT authentication for REST is not implemented (`security.jwt_secret` is reserved).
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


| Variable            | Required | Description                     |
| ------------------- | -------- | ------------------------------- |
| `OPENAI_API_KEY`    | Yes*     | OpenAI API key                  |
| `ANTHROPIC_API_KEY` | Yes*     | Anthropic API key (alternative) |
| `JWT_SECRET`        | No       | JWT signing secret              |


*At least one LLM provider key required

---

## Important Paths


| Path                               | Purpose                                                         |
| ---------------------------------- | --------------------------------------------------------------- |
| `cmd/main.go`                      | Main entrypoint for server binary                               |
| `internal/api`                     | REST + MCP API handlers                                         |
| `internal/service/interface.go`    | Service layer interfaces (I-prefix)                             |
| `internal/service/svcimpl`         | Service implementations                                         |
| `internal/storage/interface.go`    | Storage interfaces (I-prefix)                                   |
| `internal/storage/db`              | Repository implementations                                      |
| `internal/storage/coldindex/index.go` | BM25 + Vector hybrid index over cold **chapters** (`storage.IColdIndex`) |
| `internal/storage/coldindex/chapter_split.go` | `coldindex.IColdChapterSplitter`, `coldindex.IColdTextEmbedder`, default markdown tree / token merge |
| `internal/di/container.go`         | Dependency injection / composition root                         |
| `internal/service/svcimpl/retrieval.go` | `IRetrievalService`: API-only read facade over repos + cold index |
| `internal/client/llm/factory.go`   | LLM provider factory (dynamic selection)                        |
| `internal/client/llm/openai.go`    | OpenAI provider implementation                                  |
| `internal/client/llm/anthropic.go` | Anthropic Claude provider implementation                        |
| `internal/client/llm/ollama.go`    | Local Ollama provider implementation                            |
| `internal/job`                     | Background scheduled tasks (depend on `internal/service` only)   |
| `internal/service/svcimpl/document_maintenance.go` | `IDocumentMaintenanceService`: promotion sweep, queue promote, hot scores |
| `cmd/web/`                         | Vue 3 CDN frontend (embedded); ESM entry `js/main.js`, pages under `js/pages/` |
| `db/migrations/`                   | Database migration files                                        |
| `pkg/types`                        | Public types used across all layers                             |
| `docs/CORE_API_FLOWS.md`           | Core REST API algorithms and call flows (non-trivial endpoints) |
| `docs/COLD_INDEX.md`               | Cold index **core algorithms** (English): chapter extraction, Bleve+HNSW, hybrid merge, config |
| `docs/COLD_INDEX_zh.md`            | 同上，**中文**设计说明 |
| `internal/storage/coldindex/embed_*.go` | MiniLM / simple cold embeddings; `coldindex.NewTextEmbedderFromViper`, `coldindex.FallbackColdTextEmbedding` |
| `internal/storage/coldindex/coldvec/`   | Deterministic hash embedding (cold index fallback)                 |
| `third_party/minilm/README.md`     | Fetching MiniLM `model.onnx` + `tokenizer.json` (gitignored)       |
| `third_party/onnxruntime/README.md`| Vendoring ONNX Runtime libs (gitignored)                            |


---

## Troubleshooting

### Common Issues

**Build fails with SQLite error:**

- Ensure CGO is enabled: `CGO_ENABLED=1`
- On macOS: `brew install sqlite3`

**Frontend not loading:**

- Frontend is embedded in binary via Go embed, no build step needed
- Check `cmd/web/` contains `index.html` and `js/main.js` (and sibling `js/` modules)
- Clear browser cache and hard refresh (Cmd+Shift+R or Ctrl+F5)

**MCP connection issues:**

- Verify `mcp.enabled: true` in config
- Check `/mcp/sse` endpoint is accessible

**Cold document search not working:**

- Check cold index loaded on startup (see startup logs)
- Verify documents have `status: cold` in database

**MiniLM / cold semantic embeddings not loading:**

- Run `make fetch-onnxruntime` and `make fetch-minilm` from repo root (or use Docker image, which bundles both)
- Ensure `cold_index.embedding.minilm_model_path` and `onnx_runtime_path` match your OS (defaults in `configs/config.example.yaml`; paths resolve against process working directory)
- With `provider: auto`, a failed MiniLM init falls back to simple hash embeddings (check logs for the message)

