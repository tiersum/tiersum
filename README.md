# TierSum

> **Hierarchical Summary Knowledge Base** — A RAG-free document retrieval system powered by multi-layer abstraction.

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | [中文](README_zh.md)

---

## Why TierSum?

Traditional RAG systems chop documents into arbitrary chunks, losing hierarchical context and semantic structure. **TierSum preserves knowledge architecture** through 5-layer summarization:

```
┌─────────────────────────────────────┐
│  Topic Summary (Cross-document)     │  ← Theme across multiple docs
├─────────────────────────────────────┤
│  Document Summary (Bird's-eye view) │  ← 30,000ft perspective
├─────────────────────────────────────┤
│  Chapter Summary (Structural map)   │  ← 10,000ft perspective  
├─────────────────────────────────────┤
│  Paragraph Summary (Key concepts)   │  ← 1,000ft perspective
├─────────────────────────────────────┤
│  Source Text (Ground truth)         │  ← Original content
└─────────────────────────────────────┘
```

**Query flows top-down**: Start with high-level summaries, drill down to source only when needed. No vector similarity guessing — **precise hierarchical navigation**.

---

## Core Features

| Feature | Description |
|:--------|:------------|
| **5-Tier Summarization** | Topic → Document → Chapter → Paragraph → Source, auto-generated via LLM |
| **LLM Auto-Tagging** | Documents automatically tagged if tags not provided |
| **Topic Synthesis** | Cross-document theme summaries from multiple sources |
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

# Create topic from multiple documents
curl -X POST http://localhost:8080/api/v1/topics \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Cloud Native Concepts",
    "document_ids": ["doc-1", "doc-2", "doc-3"]
  }'

# Query hierarchical summary
curl "http://localhost:8080/api/v1/query?question=How does kube-scheduler work?&depth=chapter"
# depth: topic | document | chapter | paragraph | source

# List topics
curl "http://localhost:8080/api/v1/topics"

# Drill down
curl "http://localhost:8080/api/v1/documents/{id}/hierarchy?path=1.2.3"
```

### MCP Tools (for Agents)

```json
{
  "tools": [
    {
      "name": "tiersum_query",
      "description": "Query knowledge base with hierarchical precision",
      "inputSchema": {
        "question": "string",
        "depth": "topic|document|chapter|paragraph|source",
        "filters": {"tags": ["kubernetes"]}
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
      "name": "tiersum_list_topics",
      "description": "List all topic summaries",
      "inputSchema": {}
    },
    {
      "name": "tiersum_get_topic",
      "description": "Retrieve a topic summary by ID",
      "inputSchema": {
        "topic_id": "string"
      }
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
      - tiersum_get_document
      - tiersum_list_topics
      - tiersum_get_topic
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
│  │  impl/: Implementations (DocumentSvc, QuerySvc, etc.)   ││
│  │  Includes: Indexer, Summarizer, Parser                ││
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
Input (Markdown/PDF/HTML)
    │
    ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Parser    │───▶│  Structurer │───▶│ Summarizer  │
│ (Goldmark)  │    │ (Heading    │    │  (LLM API)  │
│             │    │  Hierarchy) │    │             │
└─────────────┘    └─────────────┘    └──────┬──────┘
                                             │
                   ┌─────────────────────────┼─────────────────────────────────┐
                   ▼                         ▼                                 ▼
           ┌─────────────┐          ┌─────────────┐          ┌─────────────┐ ┌─────────────┐
           │Topic Summary│          │Doc Summary  │          │Chapter Sum. │ │Para Summary │
           │(Cross-doc)  │          │(Abstract)   │          │(Outline)    │ │(Key points) │
           └─────────────┘          └─────────────┘          └─────────────┘ └─────────────┘
```

---

## Project Structure

```
tiersum/
├── cmd/
│   ├── server/          # API server entrypoint
│   ├── worker/          # Background job processor
│   └── cli/             # CLI tools
├── configs/             # Configuration files
│   ├── config.example.yaml
│   └── config.yaml
├── deployments/
│   └── docker/          # Docker and docker-compose files
├── internal/
│   ├── api/             # Layer 1: API (REST + MCP handlers)
│   ├── service/         # Layer 2: Business logic
│   │   ├── interface.go # I* interfaces (IDocumentService, etc.)
│   │   └── impl/        # Implementations
│   │       ├── document.go
│   │       ├── query.go
│   │       ├── topic.go
│   │       └── indexer.go  # Indexer, Summarizer, Parser
│   ├── storage/         # Layer 3: Data persistence
│   │   ├── interface.go # I* interfaces
│   │   ├── db/
│   │   │   └── repository.go
│   │   └── cache/
│   │       └── cache.go
│   ├── client/          # Layer 4: External dependencies
│   │   ├── interface.go # ILLMProvider
│   │   └── llm/
│   │       └── openai.go
│   ├── job/             # Background tasks
│   │   ├── scheduler.go
│   │   └── jobs.go
│   └── di/              # Dependency injection
│       └── container.go
├── pkg/
│   └── types/           # Public API types
├── migrations/          # Database migrations
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

- [x] 5-tier summarization engine (Topic + Document + Chapter + Paragraph + Source)
- [x] LLM auto-tagging for documents
- [x] Topic synthesis from multiple documents
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
