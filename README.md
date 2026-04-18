# TierSum

> **Hierarchical Summary Knowledge Base** — A RAG-free document retrieval system powered by multi-layer abstraction and hot/cold document tiering.

[Go Version](https://golang.org) · [MCP Protocol](https://modelcontextprotocol.io) · [License](LICENSE)

[English](README.md) | [简体中文](README_zh.md)

---

## What is TierSum?

Traditional RAG systems chop documents into arbitrary chunks, losing hierarchical context and semantic structure. **TierSum preserves knowledge architecture** through layered summarization with intelligent tag-based navigation:

```
┌─────────────────────────────────────────────────────────────┐
│  Topics (themes; refreshed via /topics/regroup)            │
│  ├── Cloud Native                                           │
│  │      └── Catalog tags: kubernetes, docker, helm           │
│  └── Programming Languages                                  │
│         └── Catalog tags: golang, python, rust                │
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

**Query flows through intelligent filtering**: **Tags → documents → chapters** (via topics when the catalog is large), with LLM relevance scoring at each step. No vector similarity guessing — **precise hierarchical navigation**.

---

## Key Features

| Feature | Description |
|---------|-------------|
| **Hot/Cold Tiering** | Smart storage: Hot (full LLM analysis) vs Cold (BM25 + vector search) |
| **3-Tier Summarization** | Document → Chapter → Source, auto-generated via LLM |
| **Progressive Query** | LLM filters tags → documents → chapters with relevance scoring |
| **BM25 + Vector Hybrid** | Keyword + semantic search over cold markdown chapters |
| **Dual API** | REST API + MCP Tools for agent integration |
| **Modern Web UI** | Vue 3 CDN frontend with Tailwind + DaisyUI |
| **Markdown-Native** | Optimized for `.md`; extensible to other formats |

---

## Quick Start

```bash
# 1. Clone and setup
git clone https://github.com/tiersum/tiersum.git && cd tiersum
cp configs/config.example.yaml configs/config.yaml

# 2. Set your LLM API key
export OPENAI_API_KEY="your-api-key"

# 3. Build and run
make build && make run
```

The server starts at `http://localhost:8080/` with:
- **Web UI** — `http://localhost:8080/`
- **REST API** — `http://localhost:8080/api/v1`
- **MCP SSE** — `http://localhost:8080/mcp/sse`

On first launch, visit `/init` to create the admin user. The response shows your **access token** and **API key** once — store them safely.

For detailed installation (Docker, PostgreSQL, MiniLM embeddings), see [docs/getting-started/installation.md](docs/getting-started/installation.md).
For configuration options (LLM providers, quotas, tiering), see [docs/getting-started/configuration.md](docs/getting-started/configuration.md).

---

## Documentation

| Topic | Document |
|-------|----------|
| **Installation** | [docs/getting-started/installation.md](docs/getting-started/installation.md) |
| **Configuration** | [docs/getting-started/configuration.md](docs/getting-started/configuration.md) |
| **Project Structure** | [docs/getting-started/project-structure.md](docs/getting-started/project-structure.md) |
| **Development** | [docs/getting-started/development.md](docs/getting-started/development.md) |
| **Architecture** | [docs/design/architecture.md](docs/design/architecture.md) |
| **Auth & Permissions** | [docs/design/auth-and-permissions.md](docs/design/auth-and-permissions.md) |
| **Core API Flows** | [docs/algorithms/core-api-flows.md](docs/algorithms/core-api-flows.md) |
| **Cold Document Index** | [docs/algorithms/cold-index/cold-index.md](docs/algorithms/cold-index/cold-index.md) |
| **Web UI** | [docs/ui/web-ui.md](docs/ui/web-ui.md) |
| **Roadmap** | [docs/planning/roadmap.md](docs/planning/roadmap.md) |

---

## Architecture

TierSum uses a **5-Layer Architecture** with Interface+Impl Pattern:

```
Client → API → Service → Storage → Client (LLM)
```

**Key principles:** Interface+Impl pattern, strict layer boundaries, dependency injection in `internal/di/container.go`.

📚 **Full architecture:** [docs/design/architecture.md](docs/design/architecture.md)  
📚 **Code conventions:** [AGENTS.md](AGENTS.md)

---

## Quick API Example

```bash
# Set your API key (shown once at bootstrap)
export TIERSUM_API_KEY='tsk_live_...'

# Ingest a document
curl -X POST http://localhost:8080/api/v1/documents \
  -H "X-API-Key: $TIERSUM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"title":"Kubernetes","content":"# Kubernetes...","format":"markdown"}'

# Progressive query (searches both hot and cold docs)
curl -X POST http://localhost:8080/api/v1/query/progressive \
  -H "X-API-Key: $TIERSUM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"question":"How does kube-scheduler work?","max_results":10}'
```

For the full API reference and MCP tools, see [docs/algorithms/core-api-flows.md](docs/algorithms/core-api-flows.md).

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Good first issues:**
- Additional document format parsers (LaTeX, AsciiDoc)
- Local LLM adapter improvements
- Enhanced Web UI features

---

## License

[MIT License](LICENSE) © 2026 TierSum Contributors

---

## Acknowledgments

- Inspired by [Anthropic's Contextual Retrieval](https://www.anthropic.com/news/contextual-retrieval)
- MCP Protocol by [Anthropic](https://modelcontextprotocol.io)
- Built with [Gin](https://gin-gonic.com), [Goldmark](https://github.com/yuin/goldmark), [Bleve](https://blevesearch.com), [HNSW](https://github.com/coder/hnsw)
