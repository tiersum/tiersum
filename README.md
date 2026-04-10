# TierSum

> **Hierarchical Summary Knowledge Base** — A RAG-free document retrieval system powered by multi-layer abstraction and hot/cold document tiering.

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

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

| Feature | Description |
|:--------|:------------|
| **Hot/Cold Tiering** | Smart document storage: Hot (full LLM analysis) vs Cold (BM25 + vector search) |
| **3-Tier Summarization** | Document → Chapter → Source, auto-generated via LLM |
| **Two-Level Tag Hierarchy** | L1 Tag Groups → L2 Tags (auto-generated) |
| **Progressive Query** | LLM filters tags → documents → chapters at each step |
| **Auto Tag Grouping** | LLM automatically groups related tags into categories |
| **BM25 + Vector Hybrid Search** | Keyword + semantic search with keyword-based snippet extraction |
| **RAG Alternative** | Zero chunk fragmentation; full context preservation |
| **Dual API** | REST API + MCP Tools for seamless agent integration |
| **Modern Web UI** | Vue 3 CDN frontend with Tailwind + DaisyUI dark theme |
| **Markdown-Native** | Optimized for `.md`; extensible skills for PDF/HTML/Docs |
| **Incremental Updates** | Smart diffing — re-summarize only changed sections (planned) |

---

## Hot/Cold Document Tiering

TierSum uses a two-tier system to balance LLM cost and retrieval performance:

### Hot Documents (Full Analysis)
- ✅ Full LLM analysis with document + chapter summaries
- ✅ Up to 10 auto-generated tags
- ✅ LLM-based filtering during queries
- ✅ Stored in database with tiered summaries
- ⚡ Requires quota (100/hour default)

**Criteria**: Quota available AND (force_hot OR has prebuilt summaries OR content > 5000 chars)

### Cold Documents (Efficient Storage)
- ✅ Minimal processing, no LLM analysis
- ✅ BM25 + Vector hybrid search (Bleve + HNSW)
- ✅ Keyword-based snippet extraction
- ✅ Automatic promotion after 3+ queries
- ⚡ No quota consumption

**Storage**: In-memory index with 384-dim embeddings

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
│  │  Keyword-based Snippet Extraction                     │  │
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

# Run database migrations
make migrate-up

# Build backend (includes embedded frontend)
make build

# Or use Docker Compose (includes all services)
cd deployments/docker && docker-compose up -d
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

### Start Server

```bash
# Run locally (backend + frontend)
make run

# Or run binary directly
./build/tiersum --config configs/config.yaml

# Server ready
# Web UI:   http://localhost:8080/
# REST API: http://localhost:8080/api/v1
# MCP SSE:  http://localhost:8080/mcp/sse
```

---

## API Usage

### REST API

```bash
# Ingest document (auto-tags via LLM if not provided)
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Kubernetes Architecture",
    "content": "# Kubernetes Architecture\n\n## Control Plane...",
    "format": "markdown",
    "force_hot": true  # Force full LLM analysis
  }'

# Progressive query (recommended) - searches both hot and cold docs
curl -X POST http://localhost:8080/api/v1/query/progressive \
  -H "Content-Type: application/json" \
  -d '{
    "question": "How does kube-scheduler work?",
    "max_results": 100
  }'

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

MCP tool names and JSON bodies align with the REST API under `/api/v1` (see `internal/api/mcp.go`).

| Tool | REST equivalent |
|------|-----------------|
| `api_v1_documents_post` | `POST /documents` |
| `api_v1_documents_list` | `GET /documents` |
| `api_v1_documents_get` | `GET /documents/:id` (`id`) |
| `api_v1_documents_chapters_get` | `GET /documents/:id/chapters` (`id`) |
| `api_v1_documents_summaries_get` | `GET /documents/:id/summaries` (`id`) |
| `api_v1_query_progressive_post` | `POST /query/progressive` (`question`, `max_results`) |
| `api_v1_tags_get` | `GET /tags` (`group_ids`, `max_results` optional) |
| `api_v1_tags_groups_get` | `GET /tags/groups` |
| `api_v1_tags_group_post` | `POST /tags/group` |
| `api_v1_hot_doc_summaries_get` | `GET /hot/doc_summaries` (`tags`, `max_results`) |
| `api_v1_hot_doc_chapters_get` | `GET /hot/doc_chapters` (`doc_ids`, `max_results`) |
| `api_v1_hot_doc_source_get` | `GET /hot/doc_source` (`chapter_paths`, `max_results`) |
| `api_v1_cold_doc_source_get` | `GET /cold/doc_source` (`q`, `max_results`) |
| `api_v1_quota_get` | `GET /quota` |
| `api_v1_metrics_get` | `GET /metrics` |

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
│  Storage Layer    (DB repositories + Cache + Memory Index)  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│  Client Layer     (LLM providers)                           │
└─────────────────────────────────────────────────────────────┘
```

📚 **See [AGENTS.md](AGENTS.md) for architecture, layout, and conventions.**

---

## Web UI

TierSum includes a modern Vue 3 CDN-based frontend with the following features:

### Query Page (`/#/`)
- Central search box with Progressive Query support
- Split-panel results: AI Answer (left) + Reference results (right)
- Displays both hot and cold document results
- Shows relevance scores and source indicators

### Documents Page (`/#/docs`)
- Document list with search/filter
- Create new document with modal form
- Document metadata (title, tags, format, status)
- Hot score and query count statistics

### Tag Browser (`/#/tags`)
- Two-level tag navigation
- Left panel: L1 Tag Groups (categories)
- Right panel: L2 Tags with document counts
- Click tags to filter documents

### Tech Stack
- **Framework**: Vue 3 (via CDN)
- **Router**: Vue Router 4 (via CDN)
- **Styling**: Tailwind CSS (via CDN)
- **Components**: DaisyUI (via CDN)
- **Markdown**: Marked.js (via CDN)
- **Theme**: Slate dark theme
- **Deployment**: Embedded in Go binary via `//go:embed`

---

## Snippet Extraction Algorithm

For cold documents, TierSum extracts keyword-based snippets to provide relevant context without loading full documents:

```
Query: "How does kube-scheduler work?"
    │
    ▼
Extract Keywords: ["kube-scheduler", "work", "schedule", ...]
    │
    ▼
Locate in Document: Find all keyword positions
    │
    ▼
Context Windows: Extract 200 chars before/after each match
    │
    ▼
Merge Overlapping: Combine snippets within 50 chars
    │
    ▼
Return Top 3: Most relevant merged snippets
```

**Example Output:**
```
... The kube-scheduler is the control plane component that
assigns pods to nodes. It works by watching for newly created
pods and selecting the best node for them to run on ...

...

... Scheduling decisions consider resource requirements,
affinity rules, and taints/tolerations. The scheduler
uses a scoring algorithm to rank nodes ...
```

---

## Project Structure

```
tiersum/
├── cmd/
│   ├── main.go                 # API server entrypoint
│   └── web/                    # Vue 3 CDN frontend (embedded in binary)
│       ├── index.html          # HTML entry with CDN imports
│       └── app.js              # Vue app with all components
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
│   │       ├── tag_grouping.go # Auto clustering
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
│   │   └── memory/
│   │       └── index.go        # BM25 + HNSW index
│   ├── client/                 # Layer 4: External dependencies
│   │   ├── interface.go
│   │   └── llm/
│   │       └── openai.go       # OpenAI/Anthropic
│   ├── job/                    # Background tasks
│   │   ├── scheduler.go
│   │   ├── jobs.go             # Indexer, TagGroup
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

- [x] Hot/Cold document tiering with auto-promotion
- [x] BM25 + Vector hybrid search with snippet extraction
- [x] 3-tier summarization engine (Document + Chapter + Source)
- [x] Two-level tag hierarchy with auto-grouping
- [x] Progressive query with LLM filtering at each step
- [x] LLM auto-tagging for documents
- [x] REST API + MCP Server
- [x] SQLite/PostgreSQL + in-memory cache storage
- [x] Vue 3 CDN frontend with Tailwind + DaisyUI
- [ ] OpenClaw skill pack (convert + update)
- [ ] Real-time collaborative editing
- [ ] Multi-modal support (images, diagrams)
- [ ] Enterprise SSO + audit logs

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
