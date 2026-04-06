# TierSum Architecture

## 5-Layer Architecture with Interface+Impl Pattern

```
┌─────────────────────────────────────────────────────────────────┐
│  Layer 1: API Layer (internal/api/)                             │
│  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │  REST API           │  │  MCP Server         │              │
│  │  - handler.go       │  │  - mcp.go           │              │
│  │  - Uses service.I*  │  │  - Uses service.I*  │              │
│  └─────────────────────┘  └─────────────────────┘              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 2: Service Layer (internal/service/)                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  interface.go - I-prefixed interfaces                       ││
│  │  IDocumentService, IQueryService, ITagGroupService          ││
│  │  IIndexer, ISummarizer                                     ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  svcimpl/ - Implementation subpackage                       ││
│  │  - DocumentSvc, QuerySvc, TagGroupSvc                      ││
│  │  - IndexerSvc, SummarizerSvc                               ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 3: Storage Layer (internal/storage/)                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  interface.go - I-prefixed interfaces                       ││
│  │  IDocumentRepository, ISummaryRepository, etc.             ││
│  │  ITagRepository, ITagGroupRepository, ICache               ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────┐    ┌─────────────────────────────┐│
│  │  db/repository.go       │    │  cache/cache.go             ││
│  │  - DocumentRepo         │    │  - Cache                    ││
│  │  - SummaryRepo          │    │                             ││
│  │  - TagRepo              │    │                             ││
│  │  - TagGroupRepo         │    │                             ││
│  └─────────────────────────┘    └─────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 4: Client Layer (internal/client/)                       │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  interface.go                                               ││
│  │  ILLMProvider                                               ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  llm/openai.go                                              ││
│  │  OpenAIProvider implements ILLMProvider                    ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Job Layer (internal/job/)                                      │
│  Background tasks using layer interfaces                        │
│  - TagGroupJob: Auto-tag clustering (every 30 min)             │
│  - IndexerJob: Document indexing (every 1 min)                 │
└─────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
internal/
├── api/                       # Layer 1: API Layer
│   ├── handler.go            # REST API handlers
│   └── mcp.go               # MCP protocol handlers
│
├── service/                   # Layer 2: Service Layer
│   ├── interface.go         # Service interfaces (I* prefix)
│   └── svcimpl/             # Implementation subpackage
│       ├── document.go      # DocumentSvc
│       ├── query.go         # QuerySvc
│       ├── tag_clustering.go # TagGroupSvc
│       ├── indexer.go       # IndexerSvc
│       └── summarizer.go    # SummarizerSvc
│
├── storage/                   # Layer 3: Storage Layer
│   ├── interface.go         # Storage interfaces (I* prefix)
│   ├── db/
│   │   ├── repository.go    # DB implementations
│   │   ├── schema.go        # Database schemas
│   │   └── migrator.go      # Migration manager
│   └── cache/
│       └── cache.go         # Cache implementation
│
├── client/                    # Layer 4: Client Layer
│   ├── interface.go         # Client interfaces (I* prefix)
│   └── llm/
│       └── openai.go        # LLM implementation
│
├── job/                       # Job Layer
│   ├── scheduler.go         # Job scheduler
│   └── jobs.go              # Background jobs
│
└── di/                        # Dependency Injection
    └── container.go         # Wires all layers together
```

## Interface Naming Convention

All interfaces use **I-prefix** naming:

```go
// Service Layer
IDocumentService      // Document service interface
IQueryService         // Query service interface
ITagGroupService      // Tag clustering service interface
IIndexer              // Indexer interface
ISummarizer           // Summarizer interface

// Storage Layer
IDocumentRepository      // Document repository interface
ISummaryRepository       // Summary repository interface
ITagRepository           // Tag repository interface
ITagGroupRepository      // Tag group repository interface
ICache                   // Cache interface

// Client Layer
ILLMProvider          // LLM provider interface
```

## Dependency Direction

```
        API Layer (handler.go, mcp.go)
            │ Uses: service.I*
            ▼
      Service Layer (interface.go)
            │ Uses: storage.I*, client.I*
            ▼
    ┌───────────────────────┬───────────────────────┐
    ▼                       ▼                       ▼
Storage Layer          Client Layer           Service/svcimpl
(interface.go)         (interface.go)         (concrete implementations)
    ▲                       ▲
    │ Implements            │ Implements
    ▼                       ▼
 db/repository.go       llm/openai.go
 cache/cache.go
```

## Key Design Principles

1. **Interface+Impl Pattern**: Each layer defines interfaces in `interface.go`, implementations in subpackage
2. **I-prefix Naming**: All interfaces start with I (IDocumentService, not DocumentService)
3. **No Central ports Package**: Each layer owns its interfaces
4. **No core Package**: Domain logic merged into service/svcimpl

## Two-Level Tag Hierarchy

The system organizes documents using a two-level tag hierarchy:

```
Level 1: Tag Groups (Clusters)
    ├── "Cloud Native"
    │       ├── Level 2: kubernetes
    │       ├── Level 2: docker
    │       └── Level 2: helm
    ├── "Programming Languages"
    │       ├── Level 2: golang
    │       ├── Level 2: python
    │       └── Level 2: rust
    └── ...
```

### Tag Clustering Process

1. Documents are ingested with auto-generated tags (L2)
2. `TagGroupJob` runs every 30 minutes
3. LLM clusters L2 tags into L1 groups
4. Cluster assignments are stored in database

## Progressive Query Flow

```
User Query
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 1: Filter L2 Tags (via LLM)                            │
│ Input: Query + All L2 Tags                                  │
│ Output: Relevant L2 Tags (relevance >= 0.5)                │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 2: Query & Filter Documents (via LLM)                  │
│ Input: Query + Documents matching L2 Tags (OR logic)        │
│ Output: Relevant Documents (relevance >= 0.5)              │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 3: Query & Filter Chapters (via LLM)                   │
│ Input: Query + Chapters from filtered docs                  │
│ Output: Relevant Chapters (relevance >= 0.5)               │
└─────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 4: Build Results                                       │
│ Output: QueryItem list with paths and content              │
└─────────────────────────────────────────────────────────────┘
```

## Example: Adding a New Feature

1. **Define interface** in layer's `interface.go`:
```go
// internal/service/interface.go
type IAnalyzer interface {
    Analyze(ctx context.Context, doc *types.Document) error
}
```

2. **Implement** in svcimpl subpackage:
```go
// internal/service/svcimpl/analyzer.go
type AnalyzerSvc struct{}

func (a *AnalyzerSvc) Analyze(ctx context.Context, doc *types.Document) error {
    // implementation
}

var _ service.IAnalyzer = (*AnalyzerSvc)(nil)
```

3. **Wire in** di/container.go:
```go
analyzer := svcimpl.NewAnalyzerSvc()
deps := &Dependencies{
    Analyzer: analyzer,
}
```
