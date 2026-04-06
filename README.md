# TierSum

> **Hierarchical Summary Knowledge Base** — A RAG-free document retrieval system powered by multi-layer abstraction.

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | [中文](README_zh.md)

---

## Why TierSum?

Traditional RAG systems chop documents into arbitrary chunks, losing hierarchical context and semantic structure. **TierSum preserves knowledge architecture** through 4-layer summarization:

```
┌─────────────────────────────────────┐
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
| **4-Tier Summarization** | Document → Chapter → Paragraph → Source, auto-generated via LLM |
| **RAG Alternative** | Zero chunk fragmentation; full context preservation |
| **Dual API** | REST API + MCP Tools for seamless agent integration |
| **Markdown-Native** | Optimized for `.md`; extensible skills for PDF/HTML/Docs conversion |
| **Incremental Updates** | Smart diffing — re-summarize only changed sections |

---

## Quick Start

### Prerequisites

- Go 1.23+
- PostgreSQL 16+

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

```yaml
# configs/config.yaml
server:
  port: 8080
  host: 0.0.0.0

llm:
  provider: openai  # or anthropic, local
  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4o-mini

storage:
  database:
    type: postgres
    dsn: postgres://tiersum:tiersum@localhost:5432/tiersum
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
# Ingest document
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Kubernetes Architecture",
    "content": "# Kubernetes Architecture\n\n## Control Plane...",
    "format": "markdown"
  }'

# Query hierarchical summary
curl "http://localhost:8080/api/v1/query?question=How does kube-scheduler work?&depth=chapter"
# depth: document | chapter | paragraph | source

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
        "depth": "document|chapter|paragraph|source",
        "filters": {"tags": ["kubernetes"]}
      }
    },
    {
      "name": "tiersum_explore",
      "description": "Navigate document structure interactively",
      "inputSchema": {
        "document_id": "string",
        "action": "list_chapters|get_summary|drill_down"
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
      - tiersum_explore
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
│  (OpenClaw / Claude Desktop / Custom Agents / REST Clients) │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                      API Gateway (Go)                        │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │   REST API   │  │  MCP Server  │  │  WebSocket (SSE) │  │
│  └──────────────┘  └──────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Core Engine (Go)                           │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │   Parser     │  │  Summarizer  │  │  Index Manager  │  │
│  │ (Markdown)   │  │   (LLM)      │  │  (Tree Struct)  │  │
│  └──────────────┘  └──────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    Storage Layer                             │
│  PostgreSQL (docs + hierarchy) │  In-memory cache           │
└─────────────────────────────────────────────────────────────┘
```

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
                     ┌───────────────────────┼────────────────────────┐
                     ▼                       ▼                        ▼
             ┌─────────────┐          ┌─────────────┐          ┌─────────────┐
             │Doc Summary  │          │Chapter Sum. │          │Para Summary │
             │(Abstract)   │          │(Outline)    │          │(Key points) │
             └─────────────┘          └─────────────┘          └─────────────┘
```

---

## Project Structure

```
tiersum/
├── cmd/
│   ├── server/          # API server entrypoint
│   ├── worker/          # Background job processor
│   ├── cli/             # CLI tools
│   ├── migrate/         # Database migrations
│   └── seed/            # Data seeding
├── configs/             # Configuration files
│   ├── config.example.yaml
│   └── config.yaml
├── deployments/
│   └── docker/          # Docker and docker-compose files
├── internal/
│   ├── api/             # REST handlers + MCP server
│   ├── core/
│   │   ├── parser/      # Markdown parser (Goldmark)
│   │   ├── summarizer/  # LLM abstraction layer
│   │   └── indexer/     # Hierarchical index builder
│   ├── storage/         # PostgreSQL + in-memory cache
│   └── mcp/             # MCP protocol implementation
├── pkg/
│   └── types/           # Public API types
├── skills/              # OpenClaw skill definitions
│   ├── convert/         # PDF/HTML → Markdown converters
│   └── update/          # Incremental summary updaters
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

- [x] Core 4-tier summarization engine
- [x] REST API + MCP Server
- [x] PostgreSQL + in-memory cache storage
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
