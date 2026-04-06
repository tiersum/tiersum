# TierSum Architecture

## Layer Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Layer                                │
│  ┌─────────────────┐    ┌─────────────────┐                    │
│  │   REST API      │    │  MCP Tools API  │                    │
│  │ /api/v1/*       │    │ /mcp/sse        │                    │
│  └────────┬────────┘    └────────┬────────┘                    │
└───────────┼──────────────────────┼──────────────────────────────┘
            │                      │
            ▼                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Service Layer                              │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  DocumentService  │  QueryService  │  TopicService          ││
│  │  - Ingest docs    │  - Search      │  - Topic mgmt          ││
│  │  - Auto tags      │  - Match       │  - Cross-doc summary   ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Core Domain Layer                          │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Indexer          │  Summarizer       │  Parser            ││
│  │  - 5-tier summary │  - LLM summarize  │  - Parse markdown  ││
│  │  - Hierarchical   │  - Tag generation │  - Extract struct  ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Storage Layer                               │
│  ┌─────────────────────────┐    ┌─────────────────────────────┐ │
│  │    Repository (DB)      │    │       Cache (Memory)        │ │
│  │  - Documents            │    │  - Query results            │ │
│  │  - Summaries            │    │  - Hot data                 │ │
│  │  - Topic Summaries      │    │                             │ │
│  └─────────────────────────┘    └─────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Client Layer                                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  LLM Client (OpenAI/Claude)  │  Other External APIs        ││
│  │  - Generate summary          │                             ││
│  │  - Generate tags             │                             ││
│  │  - Topic analysis            │                             ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      Job Layer (Background)                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  IndexerJob         │  TopicAggregatorJob  │  CacheCleanup ││
│  │  - Pending docs     │  - Auto group        │  - Expire     ││
│  │  - Batch process    │  - Generate topics   │  - Cleanup    ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Layer Responsibilities

### 1. API Layer (`internal/api/`, `internal/mcp/`)
**职责**: 对外暴露接口，处理协议细节
- **REST API**: HTTP 协议处理，路由，参数绑定，响应格式化
- **MCP Tools**: Model Control Protocol 处理，SSE 连接，工具注册
- **原则**: 薄层，只负责协议转换，业务逻辑委托给 Service 层

**依赖方向**: API → Service

### 2. Service Layer (`internal/domain/service/`)
**职责**: 业务逻辑实现
- **DocumentService**: 文档录入、自动标签生成、索引触发
- **QueryService**: 分层查询、标签匹配、结果聚合
- **TopicService**: 主题管理、跨文档摘要生成
- **原则**: 实现业务用例，协调多个领域服务

**依赖方向**: Service → Core, Service → Storage

### 3. Core Domain Layer (`internal/domain/core/`)
**职责**: 核心领域逻辑，纯业务规则
- **Indexer**: 5层摘要生成（Topic/Document/Chapter/Paragraph/Source）
- **Summarizer**: LLM 调用，摘要生成，标签生成
- **Parser**: 文档解析，结构提取
- **原则**: 不依赖外部系统，纯粹的业务逻辑

**依赖方向**: Core → Ports (接口)

### 4. Storage Layer (`internal/adapters/repository/`, `internal/storage/`)
**职责**: 数据持久化和缓存
- **Repository**: 数据库访问（PostgreSQL/SQLite）
- **Cache**: 内存缓存，热点数据
- **原则**: 实现 Ports 定义的接口，隔离具体存储技术

**依赖方向**: Storage → Ports (实现接口)

### 5. Client Layer (`internal/adapters/llm/`)
**职责**: 外部系统交互
- **LLM Client**: OpenAI/Claude API 调用
- **原则**: 适配器模式，隐藏外部 API 细节

**依赖方向**: Client → Ports (实现接口)

### 6. Job Layer (`internal/job/`)
**职责**: 后台定时任务
- **IndexerJob**: 处理待索引文档
- **TopicAggregatorJob**: 自动主题聚合
- **CacheCleanupJob**: 缓存清理
- **原则**: 独立运行，通过 Service 层执行业务逻辑

**依赖方向**: Job → Service, Job → Storage

## Dependency Direction

```
        API Layer
            │
            ▼
      Service Layer
            │
            ▼
    Core Domain Layer
            │
            ▼
┌───────────────────────┐
│        Ports          │  ◄── Interface Definitions
│   (internal/ports)    │
└───────────────────────┘
            ▲
            │
    ┌───────┴───────┐
    ▼               ▼
Storage Layer   Client Layer
                    ▲
                    │
                Job Layer
```

所有依赖都向内指向 **Ports** 层（接口定义）。

## Key Interfaces (Ports)

```go
// Service Interfaces
DocumentService  - 文档业务逻辑
QueryService     - 查询业务逻辑
TopicService     - 主题业务逻辑

// Core Interfaces
Indexer          - 文档索引
Summarizer       - 摘要生成
Parser           - 文档解析

// Storage Interfaces
DocumentRepository      - 文档存储
SummaryRepository       - 摘要存储
TopicSummaryRepository  - 主题存储
Cache                   - 缓存操作

// Client Interfaces
LLMProvider      - LLM 服务
```

## Data Flow Example: Document Ingestion

```
1. API Layer (REST)
   POST /api/v1/documents
   ↓
2. Service Layer (DocumentService)
   - Validate input
   - Call LLM to generate tags (if missing)
   - Save document
   - Trigger async indexing
   ↓
3. Core Layer (Indexer)
   - Parse document structure
   - Generate 5-tier summaries
   ↓
4. Storage Layer
   - Save to documents table
   - Save to summaries table
   ↓
5. Client Layer
   - LLM API calls for summarization
```

## Module Structure

```
internal/
├── api/                    # API Layer
│   └── handler.go         # REST handlers
├── mcp/                    # API Layer (MCP)
│   └── server.go          # MCP server & tools
├── domain/
│   ├── service/           # Service Layer
│   │   └── document.go    # Business logic
│   └── core/              # Core Domain Layer
│       └── services.go    # Indexer, Summarizer, Parser
├── adapters/
│   ├── repository/        # Storage Layer
│   │   └── document.go    # DB implementations
│   └── llm/               # Client Layer
│       └── openai.go      # LLM client
├── storage/               # Storage Layer (Cache)
│   └── cache.go
├── job/                   # Job Layer
│   ├── scheduler.go       # Job scheduler
│   └── jobs.go           # Job implementations
├── ports/                 # Interface Definitions
│   └── interfaces.go      # All interfaces
└── app/                   # Composition Root
    └── wire.go           # Dependency injection
```

## Adding New Features

1. **New API Endpoint**: Add to `api/handler.go` or `mcp/server.go`
2. **New Business Logic**: Add to `domain/service/`
3. **New Core Algorithm**: Add to `domain/core/`
4. **New Storage**: Add repository to `adapters/repository/`, update `ports/`
5. **New External Client**: Add to `adapters/`, implement port interface
6. **New Background Job**: Add to `job/jobs.go`, register in `app/wire.go`
