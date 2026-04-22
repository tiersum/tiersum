# Features

Everything you need for structured knowledge retrieval. TierSum combines hierarchical document processing, intelligent tiering, and hybrid search into a single, self-hostable platform.

---

## Chapter-First Document Processing

Traditional RAG systems split documents into arbitrary chunks, destroying structure and context. TierSum parses Markdown by headings, creating a natural chapter hierarchy that mirrors how humans write and read.

- Heading-aware splitting preserves document structure
- Configurable token budgets per chapter
- Sliding stride for long sections
- Stable path identifiers for every section

Example document structure:

```markdown
# Architecture Decision Records

## 01-why-tier-sum

### Context
We needed a system that preserves...

### Decision
Use chapter-first splitting...

### Consequences
- Structure preserved
- Queries return whole sections
```

---

## Hot / Cold Tiering

Not all documents need full LLM analysis. TierSum lets you choose the right ingest path for each document, balancing query quality against cost.

| Tier | Description | Best For |
|------|-------------|----------|
| **Hot** | Full LLM summaries + tags on ingest | Frequently queried documents |
| **Cold** | BM25 + vector hybrid search | Large archives, cost-sensitive |
| **Auto** | Smart selection based on content + quota | Most use cases |

- Hot: Full LLM summaries + tags on ingest
- Cold: BM25 + vector hybrid search
- Auto mode picks based on content + quota
- Auto-promotion from cold to hot on frequent queries

---

## Progressive Query

Instead of a single vector similarity search, TierSum uses a multi-stage pipeline that mimics how humans search: narrow by topic, then document, then section.

**Three stages:**

1. **Tag Filter** — Find relevant catalog tags from the query
2. **Document Rank** — Score matching documents with LLM relevance
3. **Chapter Select** — Pick top chapters from ranked documents
4. **Optional**: Synthesize answer with citations

API example:

```http
POST /api/v1/query/progressive
{
  "question": "How does hot/cold tiering work?",
  "max_results": 10
}

// Response
{
  "answer": "Hot/cold tiering balances...",
  "steps": [
    { "stage": "tags", "matches": 3 },
    { "stage": "documents", "matches": 12 },
    { "stage": "chapters", "matches": 8 }
  ],
  "results": [...]
}
```

---

## Topics & Tags

Auto-generated catalog tags grouped into LLM-curated topics. Navigate your knowledge the way humans organize it.

- Browse a shared catalog of tags
- Tags grouped into *topics* (themes)
- "Regroup" refreshes themes from your catalog
- Navigation stays meaningful as the library grows

---

## Dual-Track Authentication

Separate authentication paths for humans and programs. Humans use browser sessions with passkey support. Programs use scoped API keys.

**Authentication Methods:**

- **API Keys** — Programmatic access with scoped tokens (read / write / admin)
- **Browser Sessions** — HttpOnly cookies for web UI with role-based access
- **Passkeys** — WebAuthn support for passwordless authentication

**Access Control:**

- Role-based access: viewer, user, admin
- API key scopes: read, write, admin
- Device tokens for persistent sessions
- Rate limiting and brute-force protection

---

## REST + MCP

Full REST API for integrations. MCP (Model Context Protocol) support for AI agents and automation workflows.

- Complete REST API under `/api/v1`
- MCP protocol endpoints for AI agents
- OpenAPI-compatible JSON responses
- Prometheus metrics at `/metrics`