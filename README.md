# TierSum

> **Hierarchical Summary Knowledge Base** — A RAG-free document retrieval system powered by multi-layer abstraction and hot/cold document tiering.

[Go Version](https://golang.org)
[MCP Protocol](https://modelcontextprotocol.io)
[License](LICENSE)

[English](README.md) | [简体中文](README_zh.md)

---

## Why TierSum?

Traditional RAG systems chop documents into arbitrary chunks, losing hierarchical context and semantic structure. **TierSum preserves knowledge architecture** through layered summarization with intelligent tag-based navigation:

```
┌─────────────────────────────────────────────────────────────┐
│  Tag Groups (L1)                                            │
│  ├── Cloud Native                                           │
│  │      └── Tags: kubernetes, docker, helm                  │
│  └── Programming Languages                                  │
│         └── Tags: golang, python, rust                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────┐
│  Document Summary                   │  ← 30,000ft perspective
├─────────────────────────────────────┤
│  Chapter Summary                    │  ← 10,000ft perspective  
├─────────────────────────────────────┤
│  Source Text                        │  ← Original content
└─────────────────────────────────────┘
```

**Query flows through intelligent filtering**: Start with LLM-filtered tags, then documents, then chapters — each step refined by LLM relevance scoring. No vector similarity guessing — **precise hierarchical navigation**.

---

## Core Features


| Feature                         | Description                                                                       |
| ------------------------------- | --------------------------------------------------------------------------------- |
| **Hot/Cold Tiering**            | Smart document storage: Hot (full LLM analysis) vs Cold (BM25 + vector search)    |
| **3-Tier Summarization**        | Document → Chapter → Source, auto-generated via LLM                               |
| **Two-Level Tag Hierarchy**     | L1 Tag Groups → L2 Tags (auto-generated)                                          |
| **Progressive Query**           | LLM filters tags → documents → chapters at each step                              |
| **Auto Tag Grouping**           | LLM automatically groups related tags into categories                             |
| **BM25 + Vector Hybrid Search** | Keyword + semantic search over cold markdown chapters (full chapter text in hits) |
| **RAG Alternative**             | Zero chunk fragmentation; full context preservation                               |
| **Dual API**                    | REST API + MCP Tools for seamless agent integration                               |
| **Modern Web UI**               | Vue 3 CDN frontend with Tailwind + DaisyUI dark theme                             |
| **Markdown-Native**             | Optimized for `.md`; extensible skills for PDF/HTML/Docs                          |
| **Incremental Updates**         | Smart diffing — re-summarize only changed sections (planned)                      |


---

## Hot/Cold Document Tiering

TierSum uses a two-tier system to balance LLM cost and retrieval performance:

### Hot Documents (Full Analysis)

- ✅ Full LLM analysis with document + chapter summaries
- ✅ Up to 10 auto-generated tags
- ✅ LLM-based filtering during queries
- ✅ Stored in database with tiered summaries
- ⚡ Requires quota (100/hour default)

**Criteria (ingest_mode `auto`)**: prebuilt summary+chapters OR (quota available AND content > 5000 chars). Override with `ingest_mode`: `hot` or `cold`.

### Cold Documents (Efficient Storage)

- ✅ Minimal processing, no LLM analysis
- ✅ BM25 + Vector hybrid search (Bleve + HNSW)
- ✅ Markdown tree split into chapters; hybrid search returns full matching chapter text
- ✅ Automatic promotion after 3+ queries
- ⚡ No quota consumption

**Storage**: Cold index (in-process) with 384-dim embeddings for the vector branch

```
┌─────────────────────────────────────────────────────────────┐
│                    Hot Documents                            │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Full LLM Analysis → Tags + Summaries + Chapters     │  │
│  │  Progressive Query (L1→L2→Docs→Chapters)             │  │
│  └───────────────────────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                    Cold Documents                           │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  BM25 + Vector Hybrid Search                          │  │
│  │  Chapter-level index + full chapter text in hits      │  │
│  │  Auto-promote after 3 queries → Hot                   │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Quick Start

### Prerequisites

- Go 1.23+ (with CGO enabled for SQLite)
- Database: SQLite (default) or PostgreSQL (optional)
- LLM API Key: OpenAI or Anthropic

### Installation

```bash
# Clone repository
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# Install Go dependencies
make deps

# Copy and edit configuration
cp configs/config.example.yaml configs/config.yaml

# Set required environment variables
export OPENAI_API_KEY="your-api-key"
# or
export ANTHROPIC_API_KEY="your-api-key"

# Build backend (includes embedded frontend)
make build

# Or use Docker Compose (includes all services)
cd deployments/docker && docker-compose up -d

# Pre-built image from Alibaba ACR (login + docker pull): see deployments/docker/README.md
```

### Configuration

**SQLite (Default - Zero Config):**

```yaml
# configs/config.yaml
server:
  port: 8080

llm:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini

storage:
  database:
    driver: sqlite3
    dsn: ./data/tiersum.db

quota:
  per_hour: 100  # Hot documents per hour

documents:
  tiering:
    hot_content_threshold: 5000  # Min chars for hot tier
    cold_promotion_threshold: 3  # Query count for auto-promotion
```

**PostgreSQL (Optional - for high concurrency):**

```yaml
storage:
  database:
    driver: postgres
    dsn: postgres://user:password@localhost:5432/tiersum?sslmode=disable
```

### Cold embeddings (MiniLM + ONNX Runtime)

Semantic vectors for the **cold** index use **all-MiniLM-L6-v2** ONNX files on disk plus the **ONNX Runtime** shared library (nothing is `go:embed`’d for the neural model). Defaults in `configs/config.example.yaml` point at `third_party/...` after:

```bash
make fetch-onnxruntime   # ONNX .so / .dylib per platform
make fetch-minilm        # model.onnx + tokenizer.json from Hugging Face
```

Large artifacts are **gitignored**; run the commands above locally or in CI. The **Dockerfile** runs the same `**make fetch-onnxruntime`** and `**make fetch-minilm**` inside the image (same scripts and default versions as on your machine), then sets `onnx_runtime_path` in the baked `config.yaml` to the matching `third_party/onnxruntime/linux_*` library. If MiniLM fails to load and `cold_index.embedding.provider` is `auto`, TierSum falls back to simple hash embeddings.

See [third_party/onnxruntime/README.md](third_party/onnxruntime/README.md) and [third_party/minilm/README.md](third_party/minilm/README.md).

### Start Server

```bash
# Run locally (backend + frontend)
make run

# Or run binary directly
./build/tiersum --config configs/config.yaml

# Server ready
# Web UI:   http://localhost:8080/
# REST API: http://localhost:8080/api/v1   (DB API key after bootstrap; see below)
# BFF:      http://localhost:8080/bff/v1  (embedded UI; browser session cookie)
# Health:   http://localhost:8080/health  (liveness JSON; no API key)
# Metrics:  http://localhost:8080/metrics   (Prometheus scrape; no API key)
# MCP SSE:  http://localhost:8080/mcp/sse
```

---

## Access control and permissions (user guide)

TierSum uses **two tracks**: the **browser** (embedded UI at `/`, session cookie on `/bff/v1`) and **programs** (REST `/api/v1`, MCP tools) with **database-stored API keys**. Design details: [docs/AUTH_AND_PERMISSIONS.md](docs/AUTH_AND_PERMISSIONS.md).

### First launch (bootstrap)

1. Open the web UI (e.g. `http://localhost:8080/`). If the system is not initialized, you are redirected to **`/init`**.
2. Submit **Initialize** with the desired **admin username**. The response shows, **once**:
   - **Admin access token** (`ts_u_…`) — human login secret for the browser.
   - **Initial API key** (`tsk_live_…` or similar) — use for `curl`, scripts, and automation (store safely; it cannot be retrieved again).
3. After bootstrap, unauthenticated access to protected `/api/v1` and `/bff/v1` is rejected until you log in or send a valid API key.

### Sign in from the browser

1. Go to **`/login`** (or follow the UI after bootstrap).
2. Paste the **admin access token** (or a token issued for another user by an admin). The UI sends a small **fingerprint** (timezone, optional client signal) with login.
3. On success, the server sets an **HttpOnly** **`tiersum_session`** cookie. Normal browsing uses **`/bff/v1`** with `credentials: include` (no token in `localStorage`).

**Device limits:** each user has a **maximum number of distinct bound browsers** (`auth.browser.default_max_devices` in config for new users). If the limit is reached, sign out an old device under **Management → Devices & sessions** (`/settings`), or ask an **admin** to remove a session.

### Who can do what in the UI

| Area | Who | What |
| ---- | --- | ---- |
| **Search, documents, tags** | Any signed-in **user** or **admin** | Core product features via `/bff/v1`. |
| **Management → Observability** (`/observability`) | Any signed-in **user** or **admin** | Monitoring snapshot, cold probe, stored traces (top bar after login). |
| **Management → Devices & sessions** (`/settings`) | Everyone | Rename devices, sign out individual sessions, **Sign out all my devices**. **Admins** additionally see **every user’s** browser sessions on this page. |
| **Management → Users & API keys** (`/admin`) | **`admin` role only** | Create users, reset user tokens, list/create/revoke **API keys**, view **all devices** (Devices tab on this page), see key usage snapshot. |
| **Management → Configuration** (`/admin/config`) | **`admin` role only** | Read-only redacted effective config (`GET /bff/v1/admin/config/snapshot`). |

### Calling `/api/v1` from scripts (program track)

After bootstrap, every protected request needs a **key** created in the Admin UI (or the bootstrap payload):

```bash
export TIERSUM_API_KEY='tsk_live_...'   # example; use your real key

curl -sS -H "X-API-Key: $TIERSUM_API_KEY" http://localhost:8080/api/v1/documents
# or
curl -sS -H "Authorization: Bearer $TIERSUM_API_KEY" http://localhost:8080/api/v1/documents
```

**Scopes (on the key):**

- **`read`** — typical GETs and `POST /api/v1/query/progressive`.
- **`write`** — includes **document ingest** (`POST /documents`) and **tag regroup** (`POST /tags/group`).
- **`admin`** — superset of `write` for route checks (still a **service** credential; not the same as human **admin** role in the UI).

Wrong or missing key → **401** `invalid_key`; insufficient scope → **403** `insufficient_scope` with `required` scope.

### MCP (agents)

Configure the same **database API key** as for REST:

- Environment **`TIERSUM_API_KEY`**, or
- Config **`mcp.api_key`** (optional duplicate of the key string)

MCP tools use the **same scope rules** as `/api/v1`. `/mcp/*` is **not** covered by the browser session cookie.

### Operators and troubleshooting

- **Reset a user’s password (access token):** Admin UI → **Users** → **Reset token** (invalidates all that user’s browser sessions).
- **Revoke automation access:** Admin UI → **API keys** → **Revoke**.
- **Cannot call API after restore / new DB:** Run migrations and complete **`/init`** again for that database file.
- **Health / metrics:** `GET /health` and `GET /metrics` stay **public** (no TierSum API key).

---

## API Usage

**Core flows** (ingest tiering, progressive query, tag grouping, hot/cold retrieval, hybrid cold search): see [docs/CORE_API_FLOWS.md](docs/CORE_API_FLOWS.md).

**Auth design:** [docs/AUTH_AND_PERMISSIONS.md](docs/AUTH_AND_PERMISSIONS.md).

### REST API

After bootstrap, add **`X-API-Key`** or **`Authorization: Bearer`** on `/api/v1` requests (see [Access control and permissions (user guide)](#access-control-and-permissions-user-guide)).

```bash
# Ingest document (auto-tags via LLM if not provided)
curl -X POST http://localhost:8080/api/v1/documents \
  -H "X-API-Key: $TIERSUM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Kubernetes Architecture",
    "content": "# Kubernetes Architecture\n\n## Control Plane...",
    "format": "markdown",
    "ingest_mode": "hot"   # optional: auto (default) | hot | cold
  }'

# Progressive query (recommended) - searches both hot and cold docs
curl -X POST http://localhost:8080/api/v1/query/progressive \
  -H "X-API-Key: $TIERSUM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "question": "How does kube-scheduler work?",
    "max_results": 100
  }'

# Remaining examples: add -H "X-API-Key: $TIERSUM_API_KEY" to each /api/v1 request.

# Batch retrieval (hot / cold)
curl "http://localhost:8080/api/v1/hot/doc_summaries?tags=kubernetes,docker&max_results=100"
curl "http://localhost:8080/api/v1/hot/doc_chapters?doc_ids=uuid1,uuid2&max_results=100"
curl "http://localhost:8080/api/v1/hot/doc_source?chapter_paths=docId/chapter-title&max_results=100"
curl "http://localhost:8080/api/v1/cold/doc_source?q=scheduler,pods&max_results=100"

# List tag groups (Level 1)
curl "http://localhost:8080/api/v1/tags/groups"

# List tags filtered by L1 groups (comma-separated group_ids; optional max_results)
curl "http://localhost:8080/api/v1/tags?group_ids=group1,group2&max_results=100"

# Trigger tag grouping manually
curl -X POST http://localhost:8080/api/v1/tags/group

# Get document
curl "http://localhost:8080/api/v1/documents/{id}"

# Get document summaries
curl "http://localhost:8080/api/v1/documents/{id}/summaries"

# Check quota
curl "http://localhost:8080/api/v1/quota"
```

### MCP Tools (for Agents)

MCP tool names and JSON bodies align with the REST API under `/api/v1` (see `internal/api/mcp.go`). Supply a valid **database API key** via **`TIERSUM_API_KEY`** or **`mcp.api_key`** in config (same scopes as REST; see [user guide](#access-control-and-permissions-user-guide)).


| Tool                             | REST equivalent                                        |
| -------------------------------- | ------------------------------------------------------ |
| `api_v1_documents_post`          | `POST /documents`                                      |
| `api_v1_documents_list`          | `GET /documents`                                       |
| `api_v1_documents_get`           | `GET /documents/:id` (`id`)                            |
| `api_v1_documents_chapters_get`  | `GET /documents/:id/chapters` (`id`)                   |
| `api_v1_documents_summaries_get` | `GET /documents/:id/summaries` (`id`)                  |
| `api_v1_query_progressive_post`  | `POST /query/progressive` (`question`, `max_results`)  |
| `api_v1_tags_get`                | `GET /tags` (`group_ids`, `max_results` optional)      |
| `api_v1_tags_groups_get`         | `GET /tags/groups`                                     |
| `api_v1_tags_group_post`         | `POST /tags/group`                                     |
| `api_v1_hot_doc_summaries_get`   | `GET /hot/doc_summaries` (`tags`, `max_results`)       |
| `api_v1_hot_doc_chapters_get`    | `GET /hot/doc_chapters` (`doc_ids`, `max_results`)     |
| `api_v1_hot_doc_source_get`      | `GET /hot/doc_source` (`chapter_paths`, `max_results`) |
| `api_v1_cold_doc_source_get`     | `GET /cold/doc_source` (`q`, `max_results`)            |
| `api_v1_quota_get`               | `GET /quota`                                           |
| `api_v1_metrics_get`             | `GET /metrics`                                         |


**Claude Desktop Integration**:

```json
{
  "mcpServers": {
    "tiersum": {
      "command": "npx",
      "args": ["-y", "@anthropics/mcp-proxy", "http://localhost:8080/mcp/sse"]
    }
  }
}
```

---

## Architecture

TierSum uses a **5-Layer Architecture** with Interface+Impl Pattern:

```
┌─────────────────────────────────────────────────────────────┐
│  Client Layer                                                │
│  (REST API / MCP / Web UI)                                  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  API Layer        (REST handlers + MCP server)              │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  Service Layer    (Business logic + LLM integration)        │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  Storage Layer    (DB repositories + Cache + cold index)     │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  Client Layer     (LLM providers)                           │
└─────────────────────────────────────────────────────────────┘
```

📚 **See [AGENTS.md](AGENTS.md) for architecture, layout, and conventions.**  
🔐 **Auth and permissions (design):** [docs/AUTH_AND_PERMISSIONS.md](docs/AUTH_AND_PERMISSIONS.md)

---

## Web UI

TierSum includes a modern Vue 3 CDN-based frontend with the following features. **Which screen calls which REST endpoint** is documented in **[cmd/web/FRONTEND.md](cmd/web/FRONTEND.md)** (“Web UI ↔ REST API”). **Sign-in, admin, and devices:** [Access control and permissions (user guide)](#access-control-and-permissions-user-guide).

### Query Page (`/#/`)

- Central search box with Progressive Query support
- Split-panel results: AI Answer (left) + Reference results (right)
- Displays both hot and cold document results (from `POST /api/v1/query/progressive`)
- Shows relevance scores and tier/status indicators

### Documents (`/#/docs`, `/#/docs/new`, `/#/docs/:id`)

- **List** (`/#/docs`): filter by title/tags; opens detail on row click
- **Create** (`/#/docs/new`): full-page Markdown editor + live preview
- **Detail** (`/#/docs/:id`): loads document, summaries, and chapters via parallel GETs; cold docs emphasize source view

### Tag Browser (`/#/tags`)

- Two-level tag navigation
- Left panel: L1 Tag Groups (categories)
- Right panel: L2 Tags (filtered by selected group; document counts from API)
- Regroup button triggers `POST /api/v1/tags/group`

### Observability (`/#/observability`)

Reachable from the top bar **Management → Observability** after sign-in (same URL as before; `/monitoring` still redirects here).

- **Monitoring** tab (`?tab=monitoring`): health, runtime, document counts, cold index stats, Prometheus text preview (same data as `GET /bff/v1/monitoring` and `/metrics`).
- **Cold probe** tab (`?tab=cold`): calls `GET /bff/v1/cold/doc_source` with keywords and `max_results` to inspect hybrid cold hits (`path`, `score`, `source`, full chapter text) without running progressive query.
- **Traces** tab (`?tab=traces`): stored OpenTelemetry traces for progressive-query debugging.

### Tech Stack

- **Framework**: Vue 3 (via CDN)
- **Router**: Vue Router 4 (via CDN)
- **Styling**: Tailwind CSS (via CDN)
- **Components**: DaisyUI (via CDN)
- **Markdown**: Marked.js (via CDN)
- **Theme**: Slate dark theme
- **Deployment**: Embedded in Go binary via `//go:embed`

---

## Cold document chapters (retrieval)

Cold markdown is split into **chapters** (heading tree + bottom-up token merge under `cold_index.markdown.chapter_max_tokens`). If a leaf is still too large, **sliding windows** apply (`cold_index.markdown.sliding_stride_tokens`, default 100 tokens between window starts; overlap ≈ budget − stride). Chapter paths are **parent heading path + numeric suffix** (e.g. `docId/Section/1`); with no headings, a synthetic `**__root__`** segment is used (e.g. `docId/__root__/1`).

Chunks are indexed in **Bleve (BM25)** and **HNSW** (optional text embeddings). `GET /api/v1/cold/doc_source` runs a hybrid search; each hit’s `context` is the **full chapter body** for that path (not a small arbitrary snippet).

### Compared to traditional RAG


| Aspect                    | Typical RAG                                                         | TierSum (cold path)                                                                                                                   |
| ------------------------- | ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| **Retrieval unit**        | Fixed-size chunks (chars/tokens), weakly tied to document structure | **Markdown-aware chapters** from the heading tree; oversized leaves use **controlled sliding windows** with stable, addressable paths |
| **Structure**             | Headings, lists, and code often split mid-block                     | Prefer cuts at **heading semantics**; sliding only when a single leaf still exceeds the token budget                                  |
| **Overlap**               | Fixed overlap between adjacent chunks (mainly anti-boundary)        | Overlap derived from **window size − stride** (both configurable); continuity without random re-chunking                              |
| **Indexing & fusion**     | Often vector-first (BM25 optional)                                  | **BM25 + vector hybrid**, merged with dedupe by chapter path                                                                          |
| **What the client sees**  | Short chunks stitched in the prompt                                 | **Full chapter text** per hit for the matched path                                                                                    |
| **Cost / explainability** | Chunk + embed pipeline; similarity-only signals                     | No full LLM on ingest for cold; paths + optional `source` hint (**bm25** / **vector** / **hybrid**) aid debugging                     |


**Where classic RAG may still fit better**: unstructured prose without headings, workflows that rely on very fine-grained arbitrary spans, or teams already standardized on a single dense-chunk pipeline.

**Where TierSum’s cold model fits better**: Markdown-heavy technical docs, preserving **hierarchy and chapter boundaries**, lower cold-side LLM cost, and alignment with the **hot** path (layered summaries + tag navigation) as one coherent system.

**Algorithm deep-dive:** [docs/COLD_INDEX.md](docs/COLD_INDEX.md) (English) · [docs/COLD_INDEX_zh.md](docs/COLD_INDEX_zh.md) (中文)

---

## Project Structure

```
tiersum/
├── cmd/
│   ├── main.go                 # API server entrypoint
│   └── web/                    # Vue 3 CDN frontend (embedded in binary)
│       ├── index.html          # Shell + importmap; ESM entry `js/main.js`
│       ├── js/                 # Vue app modules (pages, api_client, …)
│       └── FRONTEND.md         # Stack, routes, UI ↔ REST mapping
├── configs/                    # Configuration files
│   ├── config.example.yaml
│   └── config.yaml
deployments/
│   └── docker/                 # Docker and docker-compose files
db/
│   └── migrations/             # Database migration files (7 versions)
├── internal/
│   ├── api/                    # Layer 1: API (REST + MCP handlers)
│   ├── service/                # Layer 2: Business logic
│   │   ├── interface.go        # I* interfaces
│   │   └── svcimpl/            # Implementations
│   │       ├── document.go     # Hot/cold tiering
│   │       ├── query.go        # Progressive query
│   │       ├── topic.go        # LLM topic regrouping + topics
│   │       ├── indexer.go      # Summary indexing
│   │       ├── summarizer.go   # LLM analysis
│   │       └── quota.go        # Rate limiting
│   ├── storage/                # Layer 3: Data persistence
│   │   ├── interface.go
│   │   ├── db/
│   │   │   ├── repository.go   # SQL repositories
│   │   │   ├── schema.go       # DB schemas
│   │   │   └── migrator.go     # Migrations
│   │   ├── cache/
│   │   │   └── cache.go        # In-memory cache
│   │   └── coldindex/          # Cold doc chapter index (Bleve + HNSW + embedders)
│   │       └── index.go        # storage.IColdIndex
│   ├── client/                 # Layer 4: External dependencies
│   │   ├── interface.go
│   │   └── llm/
│   │       └── openai.go       # OpenAI/Anthropic
│   ├── job/                    # Background tasks
│   │   ├── scheduler.go
│   │   ├── jobs.go             # Topic regroup, promote, hot score
│   │   ├── promote_job.go      # Cold→Hot promotion
│   │   └── hotscore_job.go     # Hot score calc
│   └── di/                     # Dependency injection
│       └── container.go
├── pkg/
│   └── types/                  # Public API types
├── go.mod
├── Makefile
├── README.md
└── LICENSE
```

---

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Format code
make fmt

# Run with hot reload (requires air)
make dev

# Build for multiple platforms
make build-all
```

---

## Roadmap

- Hot/Cold document tiering with auto-promotion
- BM25 + Vector hybrid search over cold chapters (full chapter text)
- 3-tier summarization engine (Document + Chapter + Source)
- Two-level tag hierarchy with auto-grouping
- Progressive query with LLM filtering at each step
- LLM auto-tagging for documents
- REST API + MCP Server
- SQLite/PostgreSQL + in-memory cache storage
- Vue 3 CDN frontend with Tailwind + DaisyUI
- OpenClaw skill pack (convert + update)
- Real-time collaborative editing
- Multi-modal support (images, diagrams)
- Enterprise SSO + audit logs

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Good first issues**:

- Additional document format parsers (LaTeX, AsciiDoc)
- Local LLM adapter (Ollama, vLLM)
- Enhanced Web UI features

---

## License

[MIT License](LICENSE) © 2026 TierSum Contributors

---

## Acknowledgments

- Inspired by [Anthropic's Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval)
- MCP Protocol by [Anthropic](https://modelcontextprotocol.io)
- Built with [Gin](https://gin-gonic.com), [Goldmark](https://github.com/yuin/goldmark), [Bleve](https://blevesearch.com), [HNSW](https://github.com/coder/hnsw)

