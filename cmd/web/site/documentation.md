# Documentation

Complete guide to using TierSum.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Ingesting Documents](#ingesting-documents)
- [Querying](#querying)
- [Hot / Cold Tiering](#hot--cold-tiering)
- [Authentication](#authentication)
- [Documents API](#documents-api)
- [Query API](#query-api)
- [MCP Protocol](#mcp-protocol)

---

## Quick Start

Get TierSum running locally in a few minutes.

### Prerequisites

- Go 1.23 or later
- Make
- OpenAI API key (or Anthropic, or local Ollama)

### Installation

```bash
# Clone the repository
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# Copy and edit configuration
cp configs/config.example.yaml configs/config.yaml
# Edit configs/config.yaml and set your LLM API key

# Build
make build

# Run
make run
```

### Bootstrap

Open http://localhost:8080 in your browser. Complete the bootstrap wizard to create the first admin user.

> The bootstrap endpoint is only accessible from localhost by default for security.

### First Document

Navigate to the Library page and click "Add Document". Paste Markdown content and choose an ingest mode:

- **Auto** — Let TierSum decide based on content length and quota
- **Hot** — Force full LLM analysis (better queries, uses quota)
- **Cold** — Index only (faster ingest, BM25 + vector search)

---

## Ingesting Documents

TierSum ingests Markdown documents and processes them according to the chosen mode.

### Supported Formats

Currently, TierSum supports Markdown (`.md`, `.markdown`) documents. The parser recognizes ATX headings (`#`, `##`, etc.) to split chapters.

### Ingest Modes

<div class="p-4 rounded-lg bg-amber-500/10 border border-amber-500/20">
<strong class="text-amber-200">Hot — Full LLM Analysis</strong><br>
Generates document summary, chapter summaries, and catalog tags. Best for frequently queried documents. Counts against hourly quota.
</div>

<div class="p-4 rounded-lg bg-sky-500/10 border border-sky-500/20 mt-4">
<strong class="text-sky-200">Cold — Index Only</strong><br>
Splits into chapters and indexes with BM25 + vector search. No LLM calls on ingest. Best for large archives and cost-sensitive deployments.
</div>

<div class="p-4 rounded-lg bg-slate-700/30 border border-slate-600/30 mt-4">
<strong class="text-slate-200">Auto — Smart Selection</strong><br>
Chooses hot if content length > 5000 chars and quota allows; otherwise cold. Recommended for most use cases.
</div>

### API Example

```http
POST /api/v1/documents
Content-Type: application/json
X-API-Key: tsk_live_xxx

{
  "title": "Architecture Decision Records",
  "content": "# Why TierSum...",
  "format": "markdown",
  "tags": ["architecture", "adr"],
  "ingest_mode": "auto"
}
```

---

## Querying

TierSum offers progressive query for intelligent retrieval and direct cold search for raw chapter hits.

### Progressive Query

The recommended query method. Walks through three stages:

1. **Tag Filter** — Find relevant catalog tags from the query
2. **Document Rank** — Score matching documents with LLM relevance
3. **Chapter Select** — Pick top chapters from ranked documents

```http
POST /api/v1/query/progressive
{
  "question": "How does authentication work?",
  "max_results": 10
}

// Returns: answer, steps, references
```

### Cold Search

Direct BM25 + vector hybrid search over cold chapter index. Returns raw chapter text without LLM synthesis.

```http
GET /api/v1/cold/chapter_hits?q=auth,login&max_results=20
```

---

## Hot / Cold Tiering

TierSum's core cost optimization mechanism. Documents can be hot (fully analyzed) or cold (indexed only).

### Promotion

Cold documents with `query_count >= 3` are automatically queued for promotion. A background job runs every 5 minutes to promote queued documents to hot, running full LLM analysis.

### Quota Management

Hot ingest is rate-limited to control LLM costs. Default: 100 documents per hour. Check current quota at `GET /api/v1/quota`.

---

## Authentication

TierSum uses dual-track authentication: API keys for programs, browser sessions for humans.

### API Keys

Include in every request via header:

```http
X-API-Key: tsk_live_xxx
# or
Authorization: Bearer tsk_live_xxx
```

Scopes: `read` (GET + query), `write` (+ ingest), `admin` (full access).

### Browser Sessions

HttpOnly cookies for web UI. Supports passkeys (WebAuthn) for passwordless authentication.

---

## Documents API

### Create Document

```http
POST /api/v1/documents
Content-Type: application/json
X-API-Key: tsk_live_xxx

{
  "title": "string",
  "content": "string",
  "format": "markdown",
  "tags": ["string"],
  "ingest_mode": "auto"
}
```

### List Documents

```http
GET /api/v1/documents?max_results=100
```

### Get Document

```http
GET /api/v1/documents/:id
```

### Get Document Chapters

```http
GET /api/v1/documents/:id/chapters
```

---

## Query API

### Progressive Query

```http
POST /api/v1/query/progressive
Content-Type: application/json
X-API-Key: tsk_live_xxx

{
  "question": "string",
  "max_results": 100
}
```

### Cold Chapter Hits

```http
GET /api/v1/cold/chapter_hits?q=keywords&max_results=20
```

---

## MCP Protocol

TierSum implements the Model Context Protocol for AI agent integration.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/mcp/sse` | SSE stream |
| POST | `/mcp/message` | JSON-RPC messages |

### Configuration

```yaml
mcp:
  enabled: true
  api_key: ${TIERSUM_API_KEY}  # Optional override
```