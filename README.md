# TierSum

> **Hierarchical Summary Knowledge Base** — A RAG-free document retrieval system powered by multi-layer abstraction.

[![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0-6E49CB)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**English** | [中文](#中文文档)

---

## Why TierSum?

Traditional RAG systems chop documents into arbitrary chunks, losing hierarchical context and semantic structure. **TierSum preserves knowledge architecture** through 4-layer summarization:

```
┌─────────────────────────────────────┐
│  Document Summary (Bird's-eye view)   │  ← 30,000ft perspective
├─────────────────────────────────────┤
│  Chapter Summary (Structural map)   │  ← 10,000ft perspective  
├─────────────────────────────────────┤
│  Paragraph Summary (Key concepts)   │  ← 1,000ft perspective
├─────────────────────────────────────┤
│  Source Text (Ground truth)           │  ← Original content
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

### Installation

```bash
# Clone repository
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# Build binary
go build -o tiersum ./cmd/server

# Or use Docker
docker run -p 8080:8080 -v ./data:/data tiersum/tiersum:latest
```

### Configuration

```yaml
# config.yaml
server:
  port: 8080

llm:
  provider: openai  # or anthropic, local
  model: gpt-4o-mini
  api_key: ${OPENAI_API_KEY}

storage:
  type: postgres
  dsn: postgres://user:pass@localhost/tiersum

index:
  type: meilisearch  # optional, for hybrid search
  host: http://localhost:7700
```

### Start Server

```bash
./tiersum --config config.yaml

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
│  PostgreSQL (docs + hierarchy) │  Redis (cache) │  Meilisearch │
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
                    ┌────────────────────────┼────────────────────────┐
                    ▼                        ▼                        ▼
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
│   ├── server/          # API server entry
│   ├── worker/          # Background job processor
│   └── cli/             # CLI tools
├── internal/
│   ├── api/             # REST handlers + MCP server
│   ├── core/
│   │   ├── parser/      # Markdown/document parsers
│   │   ├── summarizer/  # LLM abstraction layer
│   │   └── indexer/     # Hierarchical index builder
│   ├── storage/         # PostgreSQL + Redis + Meilisearch
│   └── mcp/             # MCP protocol implementation
├── pkg/
│   └── types/           # Public API types
├── skills/              # OpenClaw skill definitions
│   ├── convert/         # PDF/HTML → Markdown converters
│   └── update/          # Incremental summary updaters
├── migrations/            # Database migrations
└── docs/                  # Documentation
```

---

## Roadmap

- [x] Core 4-tier summarization engine
- [x] REST API + MCP Server
- [x] PostgreSQL + Meilisearch storage
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
- Built with [Gin](https://gin-gonic.com), [Goldmark](https://github.com/yuin/goldmark), [Meilisearch](https://meilisearch.com)

---

# 中文文档

TierSum 是基于**分层摘要**的知识库管理系统，用分层抽象替代传统 RAG 的碎片切块，实现精准的知识检索。

**核心特点**：文档摘要 → 章节摘要 → 段落摘要 → 段落原文，四层结构由 LLM 自动生成，查询时自顶向下逐层穿透，无需向量相似度猜测。

详见上方英文文档获取完整 API 说明和架构设计。
