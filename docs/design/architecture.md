# Architecture

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

## Key Principles

1. **Interface+Impl Pattern**: Each layer defines interfaces in `interface.go`; implementations are wired from `internal/di/container.go`
2. **I-prefix Naming**: All interfaces start with I (e.g., `IDocumentService`, `ICache`, `ILLMProvider`)
3. **Layer owns interfaces**: No central ports package; each layer manages its own contracts
4. **Strict dependency direction**:
   - `internal/api` → `internal/service`
   - `internal/job` → `internal/service`
   - `internal/service` → `internal/storage` + `internal/client`
5. **DI in `di/`**: All wiring happens in `internal/di/container.go`
6. **API unified**: REST and MCP handlers share the same package (`internal/api`)

## Layer Details

### Layer 1: API Layer (`internal/api/`)
- REST handlers (`handler.go`, `handler_execute.go`, `handler_catalog.go`)
- MCP protocol handlers (`mcp.go`)
- Authentication middleware (`program_auth_middleware.go`, `bff_session_middleware.go`, `bff_human_rbac_middleware.go`)
- Depends on: `internal/service/interface.go` only

### Layer 2: Service Layer (`internal/service/`)
- Facade interfaces in `interface.go`
- Shared DTOs and sentinel errors in `types.go`
- Implementations under `impl/` subpackages (auth, document, query, catalog, observability, adminconfig)
- Document analysis capability interfaces in `impl/document/analysis_contracts.go` (composition-only, not for API/Job)

### Layer 3: Storage Layer (`internal/storage/`)
- Storage interfaces in `interface.go`
- SQL repositories under `db/` (shared, document, auth, observability)
- In-memory cache (`cache/cache_impl.go`)
- Cold document index (`coldindex/`) — Bleve BM25 + HNSW vectors

### Layer 4: Client Layer (`internal/client/`)
- Third-party client interfaces in `interface.go`
- LLM provider implementations under `llm/` (OpenAI, Anthropic, Ollama)
- Factory pattern for provider selection

### Job Layer (`internal/job/`)
- Background scheduled tasks
- Depends only on `internal/service` facade contracts
- Jobs: TopicRegroupJob, PromoteJob, HotScoreJob

## Security

- **Dual-track auth**: Browser session cookies (`/bff/v1`) vs scoped API keys (`/api/v1` + MCP)
- **Human roles**: `admin`, `user`, `viewer` (browser)
- **API scopes**: `read`, `write`, `admin` (programmatic)
- See [auth-and-permissions.md](auth-and-permissions.md) for design details

---

*For full conventions, layout, and build commands, see [AGENTS.md](../AGENTS.md).*
