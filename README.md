# TierSum

> **Hierarchical Summary Knowledge Base** — A RAG-free document retrieval system powered by multi-layer abstraction.

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | [中文](README_zh.md)

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
| **3-Tier Summarization** | Document → Chapter → Source, auto-generated via LLM |
| **Two-Level Tag Hierarchy** | L1 Tag Groups (clusters) → L2 Tags (auto-generated) |
| **Progressive Query** | LLM filters tags → documents → chapters at each step |
| **Auto Tag Clustering** | LLM automatically groups related tags into categories |
| **RAG Alternative** | Zero chunk fragmentation; full context preservation |
| **Dual API** | REST API + MCP Tools for seamless agent integration |
| **Markdown-Native** | Optimized for `.md`; extensible skills for PDF/HTML/Docs |
| **Incremental Updates** | Smart diffing — re-summarize only changed sections |

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

# Build binary
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
# Run locally
make run

# Or run binary directly
./build/tiersum --config configs/config.yaml

# Server ready
# REST API: http://localhost:8080/api/v1
# MCP SSE: http://localhost:8080/mcp/sse
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
    "format": "markdown"
  }'

# Progressive query (recommended)
curl -X POST http://localhost:8080/api/v1/query/progressive \
  -H "Content-Type: application/json" \
  -d '{
    "question": "How does kube-scheduler work?",
    "max_results": 100
  }'

# Legacy hierarchical query
curl "http://localhost:8080/api/v1/query?question=How does kube-scheduler work?&depth=chapter"
# depth: document | chapter | source

# List tag clusters (Level 1)
curl "http://localhost:8080/api/v1/tags/clusters"

# Trigger tag clustering manually
curl -X POST http://localhost:8080/api/v1/tags/cluster

# Get document
curl "http://localhost:8080/api/v1/documents/{id}"
```

### MCP Tools (for Agents)

```json
{
  "tools": [
    {
      "name": "tiersum_query",
      "description": "Query knowledge base for relevant content",
      "inputSchema": {
        "question": "string",
        "depth": "document|chapter|source"
      }
    },
    {
      "name": "tiersum_progressive_query",
      "description": "Perform progressive query using two-level tag hierarchy (recommended)",
      "inputSchema": {
        "question": "string",
        "max_results": "number (default: 100)"
      }
    },
    {
      "name": "tiersum_get_document",
      "description": "Retrieve a document by ID",
      "inputSchema": {
        "document_id": "string"
      }
    },
    {
      "name": "tiersum_list_tag_clusters",
      "description": "List all tag clusters (Level 1 categories)",
      "inputSchema": {}
    },
    {
      "name": "tiersum_get_tags_by_cluster",
      "description": "Get all tags (Level 2) belonging to a specific cluster",
      "inputSchema": {
        "cluster_id": "string"
      }
    },
    {
      "name": "tiersum_trigger_tag_clustering",
      "description": "Manually trigger tag clustering (runs automatically every 30 minutes)",
      "inputSchema": {}
    }
  ]
}
```

**OpenClaw Integration**:
```yaml
# openclaw-skill/skill.yaml
mcpServers:
  tiersum:
    type: sse
    url: http://localhost:8080/mcp/sse
    tools:
      - tiersum_query
      - tiersum_progressive_query
      - tiersum_get_document
      - tiersum_list_tag_clusters
      - tiersum_get_tags_by_cluster
      - tiersum_trigger_tag_clustering
```

---

## Architecture

### 5-Layer Design with Interface+Impl Pattern

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
│  (OpenClaw / Claude Desktop / Custom Agents / REST Clients) │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                      API Layer (internal/api)                │
│  ┌──────────────┐  ┌──────────────┐                         │
│  │   REST API   │  │  MCP Server  │                         │
│  └──────────────┘  └──────────────┘                         │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Service Layer (internal/service)           │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  interface.go: I* interfaces (IDocumentService, etc.)  ││
│  └─────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────┐│
│  │  svcimpl/: Implementations (DocumentSvc, QuerySvc, etc)││
│  │  Includes: Indexer, Summarizer, TagGroupSvc           ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Storage Layer (internal/storage)           │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  interface.go: I* interfaces (IDocumentRepository, etc)││
│  └─────────────────────────────────────────────────────────┘│
│  ┌──────────────────┐    ┌──────────────────┐              ││
│  │  db/repository.go│    │  cache/cache.go  │              ││
│  │  (SQLite/PG)     │    │  (In-memory)     │              ││
│  └──────────────────┘    └──────────────────┘              ││
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    Client Layer (internal/client)            │
│  ┌─────────────────────────────────────────────────────────┐│
│  │  interface.go: ILLMProvider                            ││
│  └─────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────┐│
│  │  llm/openai.go: OpenAIProvider implementation          ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### Design Principles

1. **Interface+Impl Pattern**: Each layer defines `interface.go` with I-prefix interfaces, implementations in subpackages
2. **Layer Ownership**: No central `ports/` package — each layer manages its own interfaces
3. **Dependency Injection**: All wiring in `internal/di/container.go`
4. **Unified API**: REST and MCP handlers coexist in `internal/api/`

---

## Document Processing Pipeline

```
Input (Markdown)
    │
    ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Parser    │───▶│  Structurer │───▶│ Summarizer  │
│ (Goldmark)  │    │ (Heading    │    │  (LLM API)  │
│             │    │  Hierarchy) │    │             │
└─────────────┘    └─────────────┘    └──────┬──────┘
                                             │
                   ┌─────────────────────────┼─────────────────┐
                   ▼                         ▼                 ▼
           ┌─────────────┐          ┌─────────────┐    ┌─────────────┐
           │Doc Summary  │          │Chapter Sum. │    │Source Text  │
           │(Abstract)   │          │(Outline)    │    │(Original)   │
           └─────────────┘          └─────────────┘    └─────────────┘
```

---

## Project Structure

```
tiersum/
├── cmd/
│   └── main.go            # API server entrypoint
├── configs/               # Configuration files
│   ├── config.example.yaml
│   └── config.yaml
deployments/
│   └── docker/            # Docker and docker-compose files
db/
│   └── migrations/        # Database migration files
├── internal/
│   ├── api/               # Layer 1: API (REST + MCP handlers)
│   ├── service/           # Layer 2: Business logic
│   │   ├── interface.go   # I* interfaces (IDocumentService, etc.)
│   │   └── svcimpl/       # Implementations
│   │       ├── document.go
│   │       ├── query.go
│   │       ├── tag_clustering.go
│   │       ├── indexer.go
│   │       └── summarizer.go
│   ├── storage/           # Layer 3: Data persistence
│   │   ├── interface.go   # I* interfaces
│   │   ├── db/
│   │   │   ├── repository.go
│   │   │   ├── schema.go
│   │   │   └── migrator.go
│   │   └── cache/
│   │       └── cache.go
│   ├── client/            # Layer 4: External dependencies
│   │   ├── interface.go   # ILLMProvider
│   │   └── llm/
│   │       └── openai.go
│   ├── job/               # Background tasks
│   │   ├── scheduler.go
│   │   └── jobs.go
│   └── di/                # Dependency injection
│       └── container.go
├── pkg/
│   └── types/             # Public API types
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

- [x] 3-tier summarization engine (Document + Chapter + Source)
- [x] Two-level tag hierarchy with auto-clustering
- [x] Progressive query with LLM filtering at each step
- [x] LLM auto-tagging for documents
- [x] REST API + MCP Server
- [x] SQLite/PostgreSQL + in-memory cache storage
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
- Web UI for document exploration

---

## License

[MIT License](LICENSE) © 2026 TierSum Contributors

---

## Acknowledgments

- Inspired by [Anthropic's Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval)
- MCP Protocol by [Anthropic](https://modelcontextprotocol.io)
- Built with [Gin](https://gin-gonic.com), [Goldmark](https://github.com/yuin/goldmark)
