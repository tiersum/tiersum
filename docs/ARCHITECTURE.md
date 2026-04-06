# TierSum Architecture

## 5-Layer Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Layer 1: API Layer                                             │
│  ┌─────────────────┐    ┌─────────────────┐                    │
│  │   REST API      │    │  MCP Tools API  │                    │
│  │ internal/api    │    │ internal/mcp    │                    │
│  └────────┬────────┘    └────────┬────────┘                    │
└───────────┼──────────────────────┼──────────────────────────────┘
            │                      │
            ▼                      ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 2: Service Layer                                         │
│  Business logic implementation, called by API layer             │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  DocumentService  │  QueryService  │  TopicService          ││
│  │  - Ingest docs    │  - Search      │  - Topic mgmt          ││
│  │  - Auto tags      │  - Match       │  - Cross-doc summary   ││
│  │  internal/service │  internal/service│ internal/service      ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 3: Core Layer                                            │
│  Core domain logic, pure business rules                         │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Indexer          │  Summarizer       │  Parser            ││
│  │  - 5-tier summary │  - LLM summarize  │  - Parse markdown  ││
│  │  - Hierarchical   │  - Tag generation │  - Extract struct  ││
│  │  internal/core    │  internal/core    │  internal/core     ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 4: Storage Layer                                         │
│  Data persistence and caching                                   │
│  ┌─────────────────────────┐    ┌─────────────────────────────┐ │
│  │    DB (internal/storage/db)  │    Cache (internal/storage/cache)│
│  │  - Documents            │    │  - Query results            │ │
│  │  - Summaries            │    │  - Hot data                 │ │
│  │  - Topic Summaries      │    │                             │ │
│  └─────────────────────────┘    └─────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 5: Client Layer                                          │
│  Third-party system dependencies                                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  LLM Client (internal/client/llm)  │  Other External APIs   ││
│  │  - OpenAI provider          │                             ││
│  │  - Generate summary/tags    │                             ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Job Layer (Background) - internal/job                          │
│  Internal scheduled tasks, runs independently                   │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  IndexerJob │ TopicAggregatorJob │ CacheCleanupJob          ││
│  │  - Pending docs │ - Auto group │ - Expire cleanup           ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Package Structure

```
internal/
├── api/                    # Layer 1: API Layer
│   └── handler.go          # REST API handlers
│
├── mcp/                    # Layer 1: API Layer (MCP)
│   └── server.go           # MCP server & tools
│
├── service/                # Layer 2: Service Layer
│   └── document.go         # Business logic (DocumentSvc, QuerySvc, TopicSvc)
│
├── core/                   # Layer 3: Core Layer
│   └── services.go         # Domain logic (Indexer, Summarizer, Parser)
│
├── storage/                # Layer 4: Storage Layer
│   ├── db/                 # Database implementations
│   │   └── repository.go   # Repository implementations
│   └── cache/              # Cache implementations
│       └── cache.go        # In-memory cache with TTL
│
├── client/                 # Layer 5: Client Layer
│   └── llm/                # LLM clients
│       └── openai.go       # OpenAI provider
│
├── job/                    # Job Layer (Background tasks)
│   ├── scheduler.go        # Job scheduler
│   └── jobs.go            # Job implementations
│
├── ports/                  # Interface Definitions
│   └── interfaces.go       # All interface contracts
│
└── app/                    # Composition Root
    └── wire.go            # Dependency injection
```

## Layer Responsibilities

### Layer 1: API Layer (`internal/api/`, `internal/mcp/`)
**职责**: 对外暴露接口，处理协议细节
- **REST API**: HTTP 协议处理，路由，参数绑定，响应格式化
- **MCP Tools**: Model Control Protocol 处理，SSE 连接，工具注册
- **原则**: 薄层，只负责协议转换，业务逻辑委托给 Service 层

**依赖**: Service Layer (ports 接口)

### Layer 2: Service Layer (`internal/service/`)
**职责**: 业务逻辑实现
- **DocumentService**: 文档录入、自动标签生成、索引触发
- **QueryService**: 分层查询、标签匹配、结果聚合
- **TopicService**: 主题管理、跨文档摘要生成
- **原则**: 实现业务用例，协调 Core 层和 Storage 层

**依赖**: Core Layer, Storage Layer (ports 接口)

### Layer 3: Core Layer (`internal/core/`)
**职责**: 核心领域逻辑，纯业务规则
- **Indexer**: 5层摘要生成（Topic/Document/Chapter/Paragraph/Source）
- **Summarizer**: LLM 调用，摘要生成，标签生成
- **Parser**: 文档解析，结构提取
- **原则**: 实现核心业务算法，不依赖外部系统

**依赖**: Client Layer (通过 ports.LLMProvider 接口)

### Layer 4: Storage Layer (`internal/storage/`)
**职责**: 数据持久化和缓存
- **db/**: 数据库访问（PostgreSQL/SQLite）
- **cache/**: 内存缓存，热点数据
- **原则**: 实现 Ports 定义的接口，隔离具体存储技术

**依赖**: Ports (实现接口)

### Layer 5: Client Layer (`internal/client/`)
**职责**: 外部系统交互
- **llm/**: OpenAI/Claude API 调用
- **原则**: 适配器模式，隐藏外部 API 细节

**依赖**: 无（实现 ports.LLMProvider 接口）

### Job Layer (`internal/job/`)
**职责**: 后台定时任务
- **IndexerJob**: 处理待索引文档
- **TopicAggregatorJob**: 自动主题聚合
- **CacheCleanupJob**: 缓存清理
- **原则**: 独立运行，通过 Service Layer 执行业务逻辑

**依赖**: Service Layer, Storage Layer

## Dependency Direction

```
        API Layer (L1)
            │
            ▼
      Service Layer (L2)
            │
            ▼
       Core Layer (L3)
            │
            ▼
      Storage Layer (L4)
            │
            ▼
       Client Layer (L5)

Job Layer can call: Service Layer, Storage Layer

All layers depend on:
┌───────────────────────┐
│        Ports          │  ◄── Interface Definitions
│   (internal/ports)    │
└───────────────────────┘
```

所有依赖都向内指向 **Ports** 层（接口定义）。

## Key Interfaces (Ports)

```go
// Service Interfaces (Layer 2)
DocumentService  - 文档业务逻辑
QueryService     - 查询业务逻辑
TopicService     - 主题业务逻辑 (concrete, not in ports)

// Core Interfaces (Layer 3)
Indexer          - 文档索引
Summarizer       - 摘要生成
Parser           - 文档解析

// Storage Interfaces (Layer 4)
DocumentRepository      - 文档存储
SummaryRepository       - 摘要存储
TopicSummaryRepository  - 主题存储
Cache                   - 缓存操作

// Client Interfaces (Layer 5)
LLMProvider      - LLM 服务
```

## Data Flow Example: Document Ingestion

```
1. API Layer (REST) - internal/api/handler.go
   POST /api/v1/documents
   ↓
2. Service Layer - internal/service/document.go
   DocumentService.Ingest()
   - Validate input
   - Call LLM to generate tags (if missing)
   - Save document (via Storage Layer)
   - Trigger async indexing
   ↓
3. Core Layer - internal/core/services.go
   Indexer.Index()
   - Parse document structure (Parser)
   - Generate 5-tier summaries (Summarizer → LLM)
   ↓
4. Storage Layer - internal/storage/db/repository.go
   - Save to documents table
   - Save to summaries table
   ↓
5. Client Layer - internal/client/llm/openai.go
   - LLM API calls for summarization
```

## Adding New Features

1. **New API Endpoint**: Add to `internal/api/handler.go` or `internal/mcp/server.go`
2. **New Business Logic**: Add to `internal/service/`
3. **New Core Algorithm**: Add to `internal/core/`
4. **New Storage**: Add to `internal/storage/db/`, update `internal/ports/`
5. **New External Client**: Add to `internal/client/`, implement port interface
6. **New Background Job**: Add to `internal/job/jobs.go`, register in `internal/app/wire.go`

## Naming Conventions

- **Service Layer**: `*Svc` (e.g., `DocumentSvc`, `QuerySvc`)
- **Core Layer**: `*Svc` (e.g., `IndexerSvc`, `SummarizerSvc`)
- **Storage Layer**: `*Repo` (e.g., `DocumentRepo`, `SummaryRepo`)
- **Client Layer**: `*Provider` (e.g., `OpenAIProvider`)
