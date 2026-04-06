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
│  │  IDocumentService, IQueryService, ITopicService             ││
│  │  IIndexer, ISummarizer, IParser                            ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  impl/ - Implementation subpackage                          ││
│  │  - DocumentSvc, QuerySvc, TopicSvc                         ││
│  │  - IndexerSvc, SummarizerSvc, ParserSvc                    ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 3: Storage Layer (internal/storage/)                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  interface.go - I-prefixed interfaces                       ││
│  │  IDocumentRepository, ISummaryRepository, etc.             ││
│  │  ICache                                                     ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────┐    ┌─────────────────────────────┐│
│  │  db/repository.go       │    │  cache/cache.go             ││
│  │  - DocumentRepo         │    │  - Cache                    ││
│  │  - SummaryRepo          │    │                             ││
│  │  - TopicSummaryRepo     │    │                             ││
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
│   └── impl/                # Implementation subpackage
│       ├── document.go      # DocumentSvc
│       ├── query.go         # QuerySvc
│       ├── topic.go         # TopicSvc
│       └── indexer.go       # IndexerSvc, SummarizerSvc, ParserSvc
│
├── storage/                   # Layer 3: Storage Layer
│   ├── interface.go         # Storage interfaces (I* prefix)
│   ├── db/
│   │   └── repository.go    # DB implementations
│   └── cache/
│       └── cache.go         # Cache implementation
│
├── client/                    # Layer 4: Client Layer
│   ├── interface.go         # Client interfaces (I* prefix)
│   └── llm/
│       └── openai.go        # LLM implementation
│
├── job/                       # Job Layer
│   ├── scheduler.go
│   └── jobs.go
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
ITopicService         // Topic service interface
IIndexer              // Indexer interface
ISummarizer           // Summarizer interface
IParser               // Parser interface

// Storage Layer
IDocumentRepository      // Document repository interface
ISummaryRepository       // Summary repository interface
ITopicSummaryRepository  // Topic repository interface
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
Storage Layer          Client Layer           Service/impl
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
4. **No core Package**: Domain logic merged into service/impl

## Example: Adding a New Feature

1. **Define interface** in layer's `interface.go`:
```go
// internal/service/interface.go
type IAnalyzer interface {
    Analyze(ctx context.Context, doc *types.Document) error
}
```

2. **Implement** in impl subpackage:
```go
// internal/service/impl/analyzer.go
type AnalyzerSvc struct{}

func (a *AnalyzerSvc) Analyze(ctx context.Context, doc *types.Document) error {
    // implementation
}

var _ service.IAnalyzer = (*AnalyzerSvc)(nil)
```

3. **Wire in** di/container.go:
```go
analyzer := impl.NewAnalyzerSvc()
deps := &Dependencies{
    Analyzer: analyzer,
}
```
