# Project Structure

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
├── internal/
│   ├── api/                    # Layer 1: API (REST + MCP handlers)
│   ├── service/                # Layer 2: Contracts + facade DTOs
│   │   ├── interface.go
│   │   ├── types.go
│   │   └── impl/               # Implementations (wired only from internal/di/container.go)
│   │       ├── auth/
│   │       ├── document/       # + analysis_contracts.go (composition-only capability interfaces)
│   │       ├── query/
│   │       ├── catalog/
│   │       ├── observability/
│   │       └── adminconfig/
│   ├── storage/                # Layer 3: Data persistence
│   │   ├── interface.go
│   │   ├── db/
│   │   │   ├── unit_of_work_impl.go # NewUnitOfWork (composition root)
│   │   │   ├── shared/         # SQLDB helpers + Baseline DDL (BaseSchema)
│   │   │   ├── document/       # Document, chapter, tag, topic repos
│   │   │   ├── auth/           # system_state, users, sessions, API keys, audit
│   │   │   └── observability/  # OpenTelemetry span rows
│   │   ├── cache/
│   │   │   └── cache_impl.go   # In-memory cache
│   │   └── coldindex/          # Cold doc chapter index (Bleve + HNSW + embedders)
│   │       └── cold_index_impl.go # storage.IColdIndex
│   ├── client/                 # Layer 4: External dependencies
│   │   ├── interface.go
│   │   └── llm/
│   │       ├── llm_provider_factory.go
│   │       └── *_provider_impl.go # OpenAI / Anthropic / Ollama
│   ├── job/                    # Background tasks
│   │   ├── scheduler.go
│   │   ├── jobs.go             # Topic regroup, etc.
│   │   ├── queues.go           # Global queue channels
│   │   ├── maintenance_delegate_job.go # Promote + hot score delegates
│   │   ├── promote_consumer.go
│   │   ├── queue_consumer.go   # Shared queue consumer skeleton
│   │   └── hot_ingest_consumer.go
│   └── di/                     # Dependency injection
│       └── container.go
├── pkg/
│   └── types/                  # Public API types
├── go.mod
├── Makefile
├── README.md
└── LICENSE
```

**Note:** `internal/service` top level keeps only `interface.go`, `types.go`, and other facade contracts. Document analysis capability interfaces (`IDocumentAnalysisGenerator` / `IDocumentAnalysisPersister`) live in `impl/document/analysis_contracts.go` for composition use; they are **not** referenced by API or Job layers.
