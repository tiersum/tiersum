

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
make run                    # Build + run with configs/config.yaml
make dev                    # Hot reload (requires `air`)
make fetch-onnxruntime      # Download ONNX Runtime into third_party/ (MiniLM without OS install)
make fetch-minilm           # Download MiniLM ONNX + tokenizer into third_party/minilm/ (reproducible)

# Testing & Quality
make test                   # Run tests with race detection + coverage
make lint                   # Requires golangci-lint
make fmt                    # go fmt + gofmt -s
make vet                    # go vet ./...

# Database
# Forward schema migrations run when the server starts (see cmd/main.go).

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
  main.go                    # API server entrypoint: /api/v1 (+ API key), /bff/v1 (browser BFF), /health & /metrics (public), /mcp/*, embedded UI
configs/
  config.example.yaml        # Configuration template
db/                          # (optional) local data dir in some deployments; baseline DDL is in internal/storage/db/shared/schema.go
deployments/
  docker/
    Dockerfile               # Multi-stage build (Go + Debian bookworm, ONNX Runtime for MiniLM)
    docker-compose.yml       # SQLite default, optional PostgreSQL
internal/
  api/                       # Layer 1: API Layer
    handler.go               # REST handlers (depends on service.* only, no storage repos)
    auth_bff_handlers.go     # BFF-only JSON: bootstrap, login, /me/*, /admin/*
    bff_session_middleware.go # Browser session cookie gate for /bff/v1 (public auth paths exempt)
    bff_human_rbac_middleware.go # Human roles on /bff/v1 after session (viewer read-only; observability admin-only)
    program_auth_middleware.go # DB API key + scope gate for /api/v1
    mcp_gate.go              # MCP tool gate (same IProgramAuth + scopes as REST)
    handler_test.go          # Handler tests
    mcp.go                   # MCP protocol handlers
  service/                   # Layer 2: Service Layer
    interface.go             # I-prefixed facade interfaces (API + Job): documents, query, auth, tags, chapters, …
    types.go                 # Shared sentinel errors and auth-facing DTOs (e.g. ErrColdIndexUnavailable, principals)
    # Service implementations: Go package(s) under impl/, composed **only** in internal/di/container.go (see .cursor/rules/layer-dependencies.mdc)
    impl/document/
      analysis_contracts.go  # Document-analysis capability interfaces (composition only) — not for API/Job
  storage/                   # Layer 3: Storage Layer
    interface.go             # I-prefixed storage interfaces
    auth_entities.go         # Auth row structs (users, sessions, api_keys, …)
    db/                      # SQL repository implementations (composition root: package db)
      unit_of_work_impl.go     # NewUnitOfWork bundles subpackage constructors
      shared/                  # SQLDB, array/IN helpers, row scanner, BaseSchema DDL
      document/                # Document, chapter, tag, topic repositories
      auth/                    # system_state, users, browser_sessions, api_keys, audit
      observability/           # OpenTelemetry span persistence
    cache/                   # Cache implementation
      cache_impl.go          # Cache implements ICache
    coldindex/               # Cold document index: chapter split, BM25 (Bleve), vectors (HNSW), cold text embedders
      cold_index_impl.go      # storage.IColdIndex impl (documents + text Search); SetTextEmbedder optional for internal vectors
      markdown_chapter_splitter_impl.go # IColdChapterSplitter, MarkdownSplitter
      chapter_split_stride.go # Global sliding stride config for markdown splitter
      coldvec/               # Deterministic hash embedding fallback (no imports from service/api)
      cold_inverted_index_bleve_impl.go # Bleve BM25 cold chapter index
      cold_vector_index_hnsw_impl.go    # HNSW vector cold chapter index
      cold_text_embedder_*_impl.go      # MiniLM / simple cold embeddings
      cold_text_embedder_factory.go     # NewTextEmbedderFromViper
      cold_text_embedding_fallback.go   # FallbackColdTextEmbedding helper
  client/                    # Layer 4: Client Layer (third-party systems, e.g. LLM APIs)
    interface.go             # I-prefixed client interfaces (e.g. ILLMProvider)
    llm/                     # LLM client implementations (OpenAI, Anthropic, Ollama)
      llm_provider_factory.go  # ProviderFactory → CreateProvider
      openai_provider_impl.go  # OpenAIProvider implements ILLMProvider
      anthropic_provider_impl.go
      ollama_provider_impl.go
  job/                       # Job Layer (background tasks; depends on internal/service only)
    scheduler.go             # Job scheduler
    jobs.go                  # TopicRegroupJob
    promote_job.go           # Schedules IDocumentMaintenanceService.RunColdPromotionSweep
    promote_consumer.go      # Async queue → IDocumentMaintenanceService.PromoteColdDocumentByID
    hotscore_job.go          # Schedules IDocumentMaintenanceService.RecalculateDocumentHotScores
  di/                        # Dependency Injection (composition root)
    container.go             # Wires all layers together
pkg/metrics/                 # Prometheus metric definitions (LLM, query, documents, jobs); init registers collectors
  metrics.go
pkg/types/                   # Public API types + shared cold-embedding constants
  document.go                # Document, Chapter, Tag types
  query.go                   # Query request/response types
  auth.go                    # AuthRole*, AuthScope*, token expiry mode constants
  cold_embedding.go          # ColdEmbeddingVectorDimension, DefaultColdChapterMaxTokens
cmd/web/                     # Vue 3 CDN frontend (embedded in binary)
  index.html                 # HTML shell + importmap; loads `js/main.js` (ESM)
  js/                        # ES modules: `main.js`, `api_client.js`, `pages/`, `components/`
  FRONTEND.md                # Frontend stack, routes, **Web UI ↔ BFF REST** table
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

Job Layer (same dependency rule as API): Service Layer only (`internal/service`, e.g. `ITopicService`, `IDocumentMaintenanceService`)
```

### Key Rules

1. **Interface+Impl Pattern**: Each layer defines interfaces in `interface.go`; service facades are implemented in dedicated Go package(s) wired from `internal/di/container.go`
2. **I-prefix Naming**: All interfaces start with I (e.g., `IDocumentService`, `ICache`, `ILLMProvider`)
3. **Layer owns interfaces**: No central ports package, each layer manages its own interfaces
4. **DI in di/**: All wiring happens in `internal/di/container.go`
5. **API unified**: REST and MCP handlers in same package (`internal/api/`)
6. **English Comments Only**: All code comments must be written in English
7. **Strict layer boundaries**: Upper layers (**`internal/api`**, **`internal/job`**) may depend only on **`internal/service`** façade contracts (`interface.go`, `types.go`) and neutral **`pkg/types`** — not on **`internal/storage`**, **`internal/storage/db`**, **`internal/client`**, implementation packages under `internal/service/impl/`, or capability-only types (e.g. document analysis contracts). Service code uses storage and client **only through** `internal/storage/interface.go` and `internal/client/interface.go`. **`.cursor/rules/layer-dependencies.mdc`** is the single mandatory Cursor rule for layer edges, DTO placement, service contract shape, and when to update **`docs/algorithms/core-api-flows.md`**.

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

Key configuration file: `configs/config.yaml` (copy from `configs/config.example.yaml`). Keys present in the example file are the ones intended for use; for wiring details, search the codebase for `viper.Get` with the dotted key prefix.

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

# Run specific package tests (example)
go test -v ./internal/api/

# Run with race detection
go test -race ./...
```

### Test Structure

- Test files: `*_test.go` alongside source files
- Mocks live next to tests or in small `*_test.go` helper types as needed
- Uses `testify/assert` and `testify/require`
- Tests cover:
  - Hot/cold document tiering logic
  - Progressive query filtering
  - Topic regrouping
  - Quota management
  - API handlers

### Adding New Tests

```go
func TestFeature(t *testing.T) {
    // Create mocks
    docRepo := NewMockDocumentRepository()
    
    // Create service with mocks
    svc := NewDocumentService(docRepo, ...)
    
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
| summary       | TEXT      | Document-level summary (hot/warming; often empty for cold) |
| content       | TEXT      | Full markdown content    |
| format        | TEXT      | 'markdown' or 'md'       |
| tags          | TEXT[]    | Document tags (hot docs) |
| status        | TEXT      | 'hot', 'cold', 'warming' |
| hot_score     | REAL      | Calculated hot score     |
| query_count   | INTEGER   | Number of queries        |
| last_query_at | TIMESTAMP | Last access time         |
| created_at    | TIMESTAMP | Creation time            |


### Chapters Table (hot-document sections)

Persisted chapter rows for hot/warming documents (path from heading tree or materializer; `summary` / `content` per row).


| Column      | Type | Description                                |
| ----------- | ---- | ------------------------------------------ |
| id          | TEXT | Primary key                                |
| document_id | FK   | Reference to documents                     |
| path        | TEXT | Stable path (e.g. doc_id/section_heading)  |
| title       | TEXT | Section title                              |
| summary     | TEXT | Chapter-level summary                      |
| content     | TEXT | Original section body                      |
| created_at  | TS   | Creation time                              |
| updated_at  | TS   | Last update time                           |


### Catalog tags & topics

- `tags` — deduplicated **catalog tags** (name, document count, optional `topic_id` pointing at a theme)
- `topics` — **topics** (themes) produced by LLM **regroup**; `topic_regroup_log` records regroup runs where present in schema

---

## Hot/Cold Document Tiering

### Document Status

- `**hot**` - Full LLM analysis, tagged, persisted with document-level summary + chapter rows (requires quota)
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
| **TopicRegroupJob** | `jobs.go`     | 30 minutes | When `ShouldRefresh`, `ITopicService.RegroupTags` (LLM topics)   |
| **PromoteJob**  | `promote_job.go`  | 5 minutes  | Delegates to `IDocumentMaintenanceService.RunColdPromotionSweep` (cold→hot when `query_count` ≥ threshold) |
| **HotScoreJob** | `hotscore_job.go` | 1 hour     | Delegates to `IDocumentMaintenanceService.RecalculateDocumentHotScores` |


**Scheduler** (`scheduler.go`):

- Ticks at shortest job interval
- Tracks last execution per job
- 5-minute timeout per job

---

## API Endpoints

### Core API flows (algorithms)

Endpoints that are more than simple CRUD — ingest tiering, progressive query, topic regrouping, hot/cold retrieval, hybrid cold search — are documented in **[docs/algorithms/core-api-flows.md](docs/algorithms/core-api-flows.md)** (call chain from REST handlers into services and storage).

### REST API

The **embedded Vue UI** (`cmd/web/js/`) calls the same handlers under **`/bff/v1`** (see **`cmd/web/FRONTEND.md`** → *Web UI ↔ BFF REST*). Integrations, curl, and MCP-aligned clients use **`/api/v1`** below.

| Method | Endpoint                          | Description                                                                           |
| ------ | --------------------------------- | ------------------------------------------------------------------------------------- |
| POST   | `/api/v1/documents`               | Ingest document                                                                       |
| GET    | `/api/v1/documents`               | List documents                                                                        |
| GET    | `/api/v1/documents/:id`           | Get document                                                                          |
| GET    | `/api/v1/documents/:id/chapters`  | List chapter summaries for a document                                                 |
| POST   | `/api/v1/query/progressive`       | Progressive query (recommended)                                                       |
| GET    | `/api/v1/tags`                    | List tags (optional `topic_ids=comma&max_results`)                                     |
| GET    | `/api/v1/topics`                  | List topics (themes)                                                                  |
| GET    | `/api/v1/hot/doc_summaries`       | Hot/warming docs matching `tags`; document-level summary only (`tags`, `max_results`) |
| GET    | `/api/v1/hot/doc_chapters`        | Chapter summaries for `doc_ids` (comma-separated, `max_results` caps doc count)       |
| GET    | `/api/v1/cold/chapter_hits`       | Cold chapter hits via cold index (`q` comma-separated terms, `max_results`; JSON includes `path` per chapter). |
| POST   | `/api/v1/topics/regroup`          | LLM regroup catalog tags into topics                                                  |
| GET    | `/api/v1/quota`                   | Check quota status                                                                    |
| GET    | `/api/v1/monitoring`              | JSON monitoring snapshot (version, document counts, cold index size + Bleve inverted + HNSW vector stats, quota) |
| GET    | `/metrics`                        | Prometheus metrics (root path; **not** under `/api/v1`; public) |
| GET    | `/health`                         | Liveness JSON (root path; **not** under `/api/v1`; public)        |

The same **relative paths** (e.g. `GET /documents`, `POST /query/progressive`) are also mounted under **`/bff/v1`**, authenticated by **`BFFSessionMiddleware`** (HttpOnly session cookie after browser login) and **`BFFHumanRBAC`** (human roles: `viewer` is read-only on the BFF; observability `GET`s require `admin`), not by `/api/v1` API keys.

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

// In a service implementation package (example)
type MySvc struct{}

func (s *MySvc) DoSomething(ctx context.Context) error { ... }

// Compile-time check
var _ service.IMyService = (*MySvc)(nil)
```

### Adding New Features

1. **Define interface** in layer's `interface.go`
2. **Implement** in a service implementation package (composed from `internal/di`)
3. **Wire** in `di/container.go`
4. **Add tests** in `*_test.go`
5. **Core API docs:** If the change affects a **non–simple-CRUD** API (multi-step logic, LLM, tiering, hybrid search, topic regroup / catalog tags, hot/cold retrieval), update **`docs/algorithms/core-api-flows.md`** in the same PR/commit (see **§6** in `.cursor/rules/layer-dependencies.mdc`).
6. **AGENTS.md edits:** When changing **`AGENTS.md`**, in the same pass **strengthen Architecture-related sections** (`## Project Structure`, `## Architecture Principles`, and aligned topics such as hot/cold tiering or jobs) so structure, layers, and cross-links stay accurate. Cursor rule: `.cursor/rules/agents-architecture.mdc`.

---

## Security Considerations

- **Dual-track auth (design):** see **[docs/design/auth-and-permissions.md](docs/design/auth-and-permissions.md)**; **operator / user steps:** **[README.md](README.md#access-control-and-permissions-user-guide)**.
- **Dual-track auth (summary):** **`/api/v1/*`** and MCP tool calls require **database API keys** validated via **`service.IProgramAuth`** (implementations wired from `internal/di`); browser/admin flows use **`service.IAuthService`**. Send `X-API-Key` or `Authorization: Bearer` with `tsk_live_*` or `tsk_admin_*` values created in the admin UI (or the bootstrap response). Scopes: `read` (default GET + `POST /query/progressive`), `write` (+ document ingest + tag regroup), `admin` (superset). **`/bff/v1/*`** uses **HttpOnly session cookies** after `POST /bff/v1/auth/login` (`BFFSessionMiddleware`, then **`BFFHumanRBAC`**: human roles `admin` / `user` / `viewer`; **`viewer`** is read-only except `POST /query/progressive`; **`GET /monitoring`** and **`GET /traces*`** require human **`admin`**). Until first bootstrap, **`/api/v1`** returns **403** `{ "code": "SYSTEM_NOT_INITIALIZED" }`. **`GET /health`** and **`GET /metrics`** stay public at the root. MCP reads **`TIERSUM_API_KEY`** or `mcp.api_key` for the same validation as REST.
- JWT authentication for REST is not implemented (no corresponding config keys).
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


*At least one LLM provider key required

---

## Important Paths


| Path                               | Purpose                                                         |
| ---------------------------------- | --------------------------------------------------------------- |
| `cmd/main.go`                      | Main entrypoint for server binary                               |
| `internal/api`                     | REST + MCP API handlers                                         |
| `internal/service/interface.go`    | Service layer interfaces (I-prefix)                             |
| `internal/di/container.go`         | Dependency injection — sole composition root for implementations   |
| `internal/storage/interface.go`    | Storage interfaces (I-prefix)                                   |
| `internal/storage/db`              | SQL persistence composition (`NewUnitOfWork`); subpackages: `shared`, `document`, `auth`, `observability` |
| `internal/storage/coldindex/cold_index_impl.go` | BM25 + Vector hybrid index over cold **chapters** (`storage.IColdIndex`) |
| `internal/storage/coldindex/markdown_chapter_splitter_impl.go` | `coldindex.IColdChapterSplitter`, default markdown tree / token merge |
| `internal/client/llm/llm_provider_factory.go` | LLM provider factory (dynamic selection)                        |
| `internal/client/llm/openai_provider_impl.go`    | OpenAI provider implementation                                  |
| `internal/client/llm/anthropic_provider_impl.go` | Anthropic Claude provider implementation                        |
| `internal/client/llm/ollama_provider_impl.go`    | Local Ollama provider implementation                            |
| `internal/job`                     | Background scheduled tasks (depend on `internal/service` only)   |
| `cmd/web/`                         | Vue 3 CDN frontend (embedded); ESM entry `js/main.js`, pages under `js/pages/` |
| `internal/storage/db/shared/schema.go` | Baseline DDL (`shared.BaseSchema`) applied on startup          |
| `pkg/types`                        | Public types used across all layers                             |
| `docs/algorithms/core-api-flows.md`           | Core REST API algorithms and call flows (non-trivial endpoints) |
| `docs/design/auth-and-permissions.md`     | Dual-track auth design: human vs program, roles, scopes, tables, config |
| `docs/algorithms/cold-index/cold-index.md`               | Cold index **core algorithms** (English): chapter extraction, Bleve+HNSW, hybrid merge, config |
| `docs/algorithms/cold-index/cold-index.zh.md`            | 同上，**中文**设计说明 |
| `internal/storage/coldindex/cold_text_embedder_*_impl.go` / `cold_text_embedder_factory.go` / `cold_text_embedding_fallback.go` | MiniLM / simple cold embeddings; `coldindex.NewTextEmbedderFromViper`, `coldindex.FallbackColdTextEmbedding` |
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

