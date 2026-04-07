# TierSum Architecture

## Table of Contents

1. [5-Layer Architecture](#5-layer-architecture)
2. [Hot/Cold Document Tiering](#hotcold-document-tiering)
3. [Progressive Query Flow](#progressive-query-flow)
4. [Snippet Extraction Algorithm](#snippet-extraction-algorithm)
5. [Job Layer](#job-layer)
6. [Data Flow Diagrams](#data-flow-diagrams)
7. [Package Structure](#package-structure)
8. [Interface Naming Convention](#interface-naming-convention)
9. [Adding New Features](#adding-new-features)

---

## 5-Layer Architecture

TierSum uses a **5-Layer Architecture with Interface+Impl Pattern** for clear separation of concerns and testability.

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
│  │  - DocumentSvc (hot/cold tiering)                          ││
│  │  - QuerySvc (progressive query)                            ││
│  │  - TagGroupSvc (tag clustering)                            ││
│  │  - IndexerSvc (summary indexing)                           ││
│  │  - SummarizerSvc (LLM analysis)                            ││
│  │  - QuotaManager (rate limiting)                            ││
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
│  │  IInMemoryIndex (BM25 + Vector for cold docs)              ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────┐    ┌─────────────────────────────┐│
│  │  db/repository.go       │    │  cache/cache.go             ││
│  │  - DocumentRepo         │    │  - Cache                    ││
│  │  - SummaryRepo          │    │                             ││
│  │  - TagRepo              │    │                             ││
│  │  - TagGroupRepo         │    │                             ││
│  └─────────────────────────┘    └─────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  memory/index.go - BM25 + HNSW hybrid search               ││
│  │  - Bleve for BM25 text search                              ││
│  │  - HNSW for vector similarity (384-dim)                    ││
│  │  - Snippet extraction for cold documents                   ││
│  └─────────────────────────────────────────────────────────────┘│
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
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  scheduler.go - Job scheduler with timeout support         ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │  jobs.go:                                                   ││
│  │  - IndexerJob: Document indexing (1 min)                   ││
│  │  - TagGroupJob: Auto-tag grouping (30 min)                 ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │  promote_job.go:                                            ││
│  │  - PromoteJob: Cold→Hot promotion (5 min)                  ││
│  ├─────────────────────────────────────────────────────────────┤│
│  │  hotscore_job.go:                                           ││
│  │  - HotScoreJob: Score calculation (1 hour)                 ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Dependency Direction

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
  memory/index.go
```

---

## Hot/Cold Document Tiering

TierSum implements a two-tier document storage strategy to balance LLM cost and query performance.

### Document Status States

```go
type DocumentStatus string

const (
    DocStatusHot     DocumentStatus = "hot"      // Full LLM analysis
    DocStatusCold    DocumentStatus = "cold"     // Minimal processing
    DocStatusWarming DocumentStatus = "warming"  // Being promoted
)
```

### Hot Documents

**Characteristics:**
- Full LLM analysis with document + chapter summaries
- Up to 10 auto-generated tags
- LLM-based filtering during queries
- Stored in database with tiered summaries
- Requires quota (default: 100/hour)

**Criteria for Hot Tier:**
```go
// 1. Quota available
// 2. AND one of:
//    - force_hot=true in request
//    - Has pre-built summary/chapters
//    - Content length > 5000 characters
```

**Processing Flow:**
```
Document Ingest
    │
    ▼
┌─────────────────────────────┐
│  Check Quota (QuotaManager) │
└──────────────┬──────────────┘
               │ Available?
      ┌────────┴────────┐
      │ Yes             │ No
      ▼                 ▼
┌──────────────┐  ┌──────────────┐
│ Hot Path     │  │ Cold Path    │
├──────────────┤  ├──────────────┤
│ • LLM Analyze│  │ • Embedding  │
│ • Summarize  │  │ • BM25 Index │
│ • Tag Extraction      │
│ • Chapter Split       │
└──────┬───────┘  └──────┬───────┘
       │                 │
       └────────┬────────┘
                ▼
       ┌────────────────┐
       │  Database      │
       │  (SQLite/PG)   │
       └────────────────┘
```

### Cold Documents

**Characteristics:**
- Minimal processing, no LLM analysis
- BM25 + Vector hybrid search
- Keyword-based snippet extraction
- Automatic promotion after 3+ queries
- No quota consumption

**Storage:**
- In-memory index (Bleve + HNSW)
- 384-dimensional embeddings
- Simple n-gram hash-based embeddings

**Processing Flow:**
```
Document Ingest (Cold)
    │
    ▼
Generate Simple Embedding
    │ (char n-gram hash)
    ▼
Add to Memory Index
    ├─ Bleve (BM25 text search)
    └─ HNSW (384-dim vector search)
    │
    ▼
On Query: Hybrid Search
    ├─ BM25: Keyword-based ranking
    ├─ Vector: Cosine similarity
    └─ Merge: 50/50 weighting
    │
    ▼
Extract Snippets
    ├─ Keyword positioning
    ├─ Context window (±200 chars)
    └─ Merge overlapping
    │
    ▼
Track Access
    ├─ Increment query_count
    └─ If count >= 3 → PromoteQueue
```

### Cold Document Promotion

**PromoteJob** (runs every 5 minutes):
```
Check PromoteQueue
    │
    ▼
For each cold doc with query_count >= 3:
    ├─ Change status to "warming"
    ├─ Run full LLM analysis
    ├─ Generate summaries + tags
    ├─ Store in database
    └─ Change status to "hot"
```

**Hot Score Calculation** (HotScoreJob, runs hourly):
```go
hot_score = query_count / (1 + hours_since_last_query)
```

---

## Progressive Query Flow

The progressive query combines hot path (tag-based) and cold path (hybrid search) for comprehensive retrieval.

### Complete Query Flow

```
User Query
    │
    ▼
Step 1: Adaptive L2 Tag Filtering
    ├─ Get all L2 tags count
    ├─ If count < 200:
    │   └─ Direct LLM filter on all L2 tags
    └─ If count >= 200:
        └─ L1 → L2 two-level filter:
            ├─ LLM selects 1-3 relevant L1 groups
            ├─ Collect L2 tags from selected groups
            └─ LLM filters L2 tags (relevance >= 0.5)
    │
    ▼
Step 2: Document Retrieval & Filtering
    ├─ Query docs by filtered L2 tags (OR logic)
    ├─ Separate hot vs cold documents
    ├─ Hot docs: LLM filter (relevance >= 0.5)
    └─ Cold docs: Keyword matching
    │
    ▼
Step 3: Chapter Retrieval & Filtering
    ├─ Hot docs: Get chapters from summary repo
    ├─ Cold docs: Create pseudo-chapters from snippets
    └─ LLM filter chapters (relevance >= 0.5)
    │
    ▼
Step 4: Cold Path (Parallel)
    ├─ BM25 + Vector hybrid search
    ├─ Extract keyword-based snippets
    └─ Convert to QueryItem format
    │
    ▼
Step 5: Result Merging
    ├─ Combine hot and cold results
    ├─ Deduplicate by document ID
    ├─ Boost relevance if found in both paths
    └─ Sort by relevance, return top K
    │
    ▼
Step 6: Access Tracking
    ├─ Increment query_count for each doc
    └─ If cold doc queried 3+ times → PromoteQueue
    │
    ▼
Return ProgressiveQueryResponse
    ├─ Question (original)
    ├─ Steps (execution trace with timings)
    └─ Results (QueryItem list)
```

### Two-Level Tag Hierarchy

```
Level 1: Tag Groups (created by LLM clustering)
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

**Tag Grouping Process:**

The `TagGroupJob` runs every 30 minutes:

1. **Check Need**: Compare current tag count with last refresh
2. **LLM Clustering**: Group all L2 tags into 3-10 L1 categories
3. **Target**: ~10 tags per group
4. **Update**: Clear old groups, create new ones
5. **Log**: Record refresh metrics

---

## Snippet Extraction Algorithm

For cold documents, TierSum extracts keyword-based snippets to provide relevant context.

### Algorithm Steps

```
Input: content (document text), query (user question)
    │
    ▼
1. EXTRACT KEYWORDS
    ├─ Extract top 10 keywords from query
    │  (by frequency, length > 4)
    └─ Example: ["kube-scheduler", "work", "schedule", "node"]
    │
    ▼
2. LOCATE KEYWORDS
    ├─ Find all occurrences of each keyword
    └─ Record position (start, end) for each match
    │
    ▼
3. CREATE CONTEXT WINDOWS
    ├─ For each keyword match:
    │   ├─ start = match_pos - 200 (ContextWindowSize)
    │   ├─ end = match_pos + len(keyword) + 200
    │   └─ Clip to document bounds
    └─ Example windows: [0-450], [380-830], [1200-1650]
    │
    ▼
4. SORT & MERGE
    ├─ Sort windows by start position
    ├─ Merge overlapping/adjacent windows:
    │   └─ If next.start <= current.end + 50 (MergeDistance)
    │       └─ Extend current.end to next.end
    └─ Re-extract merged text from original content
    │
    ▼
5. LIMIT RESULTS
    ├─ Keep top 3 merged snippets (MaxSnippetsPerDoc)
    └─ Format with ellipsis indicators
    │
    ▼
Output: []Snippet{Text, StartPos, EndPos, Keyword}
```

### Configuration Constants

```go
const (
    ContextWindowSize = 200   // Characters before/after keyword
    MaxSnippetLength  = 500   // Maximum snippet length
    MaxSnippetsPerDoc = 3     // Maximum snippets per document
    MergeDistance     = 50    // Distance threshold for merging
)
```

### Example Output

**Query**: "How does kube-scheduler work?"

**Document**: "Kubernetes Architecture" (10,000+ chars)

**Extracted Snippets**:
```
Snippet 1 (chars 450-950):
"... The kube-scheduler is the control plane component that
assigns pods to nodes. It works by watching for newly created
pods and selecting the best node for them to run on ..."

Snippet 2 (chars 2100-2700):
"... Scheduling decisions consider resource requirements,
affinity rules, and taints/tolerations. The scheduler
uses a scoring algorithm to rank nodes ..."

Snippet 3 (chars 4200-4700):
"... If a pod cannot be scheduled, the scheduler will
retry with backoff. Events are emitted to help debug
scheduling failures ..."
```

---

## Job Layer

Background tasks run on configurable intervals using the internal scheduler.

### Job Scheduler

```go
// Scheduler ticks at shortest job interval
// Each job tracks last execution time
// 5-minute timeout per job
```

### Available Jobs

| Job | File | Interval | Purpose |
|-----|------|----------|---------|
| **IndexerJob** | `jobs.go` | 1 minute | Processes pending document indexing |
| **TagGroupJob** | `jobs.go` | 30 minutes | LLM-based tag clustering into L1 groups |
| **PromoteJob** | `promote_job.go` | 5 minutes | Promotes cold docs (query_count > 3) to hot |
| **HotScoreJob** | `hotscore_job.go` | 1 hour | Updates hot scores: `query_count / (1 + hours_since_last_query)` |

### Job Execution Flow

```
Scheduler Start
    │
    ▼
Tick (every minute)
    │
    ▼
For each job:
    ├─ Check if interval elapsed
    ├─ If yes:
    │   ├─ Create context with 5-min timeout
    │   ├─ Execute job
    │   └─ Update last execution time
    └─ If no: skip
    │
    ▼
Repeat
```

---

## Data Flow Diagrams

### Document Ingestion Flow

```
1. API Layer
   POST /api/v1/documents
   │
   ▼
2. Service Layer (DocumentSvc.Ingest)
   ├─ Check Quota (QuotaManager)
   ├─ Determine hot vs cold:
   │   ├─ Hot: quota && (force_hot || has_summary || len > 5000)
   │   └─ Cold: otherwise
   │
   ▼
3a. HOT Path:
   ├─ SummarizerSvc.AnalyzeDocument()
   │   ├─ LLM generates summary
   │   ├─ LLM extracts up to 10 tags
   │   └─ LLM identifies chapters
   ├─ IndexerSvc.Index()
   │   ├─ Store document summary
   │   ├─ Store chapter summaries
   │   └─ Store source content
   └─ TagRepo.Create() [update global tags]
   │
3b. COLD Path:
   ├─ GenerateSimpleEmbedding() [n-gram hash]
   ├─ MemoryIndex.AddDocument()
   │   ├─ Bleve index (BM25)
   │   └─ HNSW index (vector)
   │
   ▼
4. Storage Layer
   DocumentRepo.Create()
   │
   ▼
5. Database
   INSERT INTO documents (...)
```

### Query Flow (Detailed)

```
1. API Layer
   POST /api/v1/query/progressive
   │
   ▼
2. Service Layer (QuerySvc.ProgressiveQuery)
   │
   ├─ Step 1: Tag Filtering ──────────────────────────────┐
   │   ├─ Get all L2 tags                                 │
   │   ├─ If < 200 tags:                                  │
   │   │   └─ Summarizer.FilterL2TagsByQuery()            │
   │   └─ If >= 200 tags:                                 │
   │       ├─ Get L1 groups                               │
   │       ├─ LLM filter L1 groups                        │
   │       ├─ Get L2 tags from selected groups            │
   │       └─ LLM filter L2 tags                          │
   │                                                      │
   ├─ Step 2: Document Retrieval ─────────────────────────┤
   │   ├─ DocumentRepo.ListByTags() [OR logic]            │
   │   ├─ Separate hot/cold                               │
   │   ├─ Hot: LLM filter (relevance >= 0.5)              │
   │   └─ Cold: Keyword matching                          │
   │                                                      │
   ├─ Step 3: Chapter Retrieval ──────────────────────────┤
   │   ├─ Hot: SummaryRepo.GetByDocument()                │
   │   ├─ Cold: Create pseudo-chapters from snippets      │
   │   └─ LLM filter chapters                             │
   │                                                      │
   ├─ Step 4: Cold Path (Parallel) ───────────────────────┤
   │   ├─ MemoryIndex.HybridSearch()                      │
   │   │   ├─ BM25 search                                 │
   │   │   ├─ Vector search                               │
   │   │   └─ Merge results (50/50)                       │
   │   └─ Extract snippets                                │
   │                                                      │
   ├─ Step 5: Merge Results ──────────────────────────────┤
   │   ├─ Combine hot and cold                            │
   │   ├─ Deduplicate by doc ID                           │
   │   ├─ Boost if found in both paths                    │
   │   └─ Sort by relevance                               │
   │                                                      │
   └─ Step 6: Track Access ───────────────────────────────┘
       ├─ Increment query_count
       └─ Promote if >= 3 queries
   │
   ▼
3. Build Response
   ProgressiveQueryResponse{
       Question: "...",
       Steps: [...],      // Execution trace
       Results: [...]     // QueryItem list
   }
```

---

## Package Structure

```
internal/
├── api/                       # Layer 1: API Layer
│   ├── handler.go            # REST API handlers
│   │   ├─ CreateDocument
│   │   ├─ GetDocument
│   │   ├─ ProgressiveQuery
│   │   ├─ ListTagGroups
│   │   └─ ... (all REST endpoints)
│   └── mcp.go               # MCP protocol handlers
│       ├─ Tools registration
│       └─ Tool implementations
│
├── service/                   # Layer 2: Service Layer
│   ├── interface.go         # Service interfaces (I* prefix)
│   │   ├─ IDocumentService
│   │   ├─ IQueryService
│   │   ├─ ITagGroupService
│   │   ├─ IIndexer
│   │   └─ ISummarizer
│   └── svcimpl/             # Implementation subpackage
│       ├── document.go      # DocumentSvc: hot/cold logic
│       ├── query.go         # QuerySvc: progressive query
│       ├── tag_grouping.go  # TagGroupSvc: LLM clustering
│       ├── indexer.go       # IndexerSvc: summary storage
│       ├── summarizer.go    # SummarizerSvc: LLM prompts
│       └── quota.go         # QuotaManager: rate limiting
│
├── storage/                   # Layer 3: Storage Layer
│   ├── interface.go         # Storage interfaces (I* prefix)
│   │   ├─ IDocumentRepository
│   │   ├─ ISummaryRepository
│   │   ├─ ITagRepository
│   │   ├─ ICache
│   │   └─ IInMemoryIndex
│   ├── db/
│   │   ├── repository.go    # Repository implementations
│   │   │   ├─ DocumentRepo
│   │   │   ├─ SummaryRepo
│   │   │   ├─ TagRepo
│   │   │   └─ TagGroupRepo
│   │   ├── schema.go        # SQL schema definitions
│   │   └── migrator.go      # Migration manager
│   ├── cache/
│   │   └── cache.go         # TTL-based in-memory cache
│   └── memory/
│       └── index.go         # BM25 + HNSW hybrid index
│           ├─ Bleve (BM25)
│           ├─ HNSW (vector)
│           └─ Snippet extraction
│
├── client/                    # Layer 4: Client Layer
│   ├── interface.go         # ILLMProvider
│   └── llm/
│       └── openai.go        # OpenAI/Anthropic client
│
├── job/                       # Job Layer
│   ├── scheduler.go         # Job scheduler
│   ├── jobs.go              # IndexerJob, TagGroupJob
│   ├── promote_job.go       # Cold→Hot promotion
│   └── hotscore_job.go      # Hot score calculation
│
└── di/                        # Dependency Injection
    └── container.go         # Wires all layers together
```

---

## Interface Naming Convention

All interfaces use **I-prefix** naming for clear identification:

### Service Layer

```go
IDocumentService      // Document service interface
IQueryService         // Query service interface
ITagGroupService      // Tag grouping service interface
IIndexer              // Indexer interface
ISummarizer           // Summarizer interface
```

### Storage Layer

```go
IDocumentRepository        // Document repository interface
ISummaryRepository         // Summary repository interface
ITagRepository             // Tag repository interface
ITagGroupRepository        // Tag group repository interface
ITagGroupRefreshLogRepository  // Refresh log interface
ICache                     // Cache interface
IInMemoryIndex            // Memory index interface (BM25 + vector)
```

### Client Layer

```go
ILLMProvider          // LLM provider interface
```

### Key Design Principles

1. **Interface+Impl Pattern**: Each layer defines interfaces in `interface.go`, implementations in subpackage
2. **I-prefix Naming**: All interfaces start with I (IDocumentService, not DocumentService)
3. **No Central ports Package**: Each layer owns its interfaces
4. **No core Package**: Domain logic merged into service/svcimpl

---

## Adding New Features

### Example: Adding a New Service

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
type AnalyzerSvc struct {
    llm client.ILLMProvider
}

func NewAnalyzerSvc(llm client.ILLMProvider) *AnalyzerSvc {
    return &AnalyzerSvc{llm: llm}
}

func (a *AnalyzerSvc) Analyze(ctx context.Context, doc *types.Document) error {
    // Implementation
    return nil
}

// Compile-time check
var _ service.IAnalyzer = (*AnalyzerSvc)(nil)
```

3. **Wire in** di/container.go:

```go
// internal/di/container.go
func NewDependencies(...) (*Dependencies, error) {
    analyzer := svcimpl.NewAnalyzerSvc(llmProvider)
    
    deps := &Dependencies{
        Analyzer: analyzer,
        // ... other dependencies
    }
    return deps, nil
}
```

4. **Use in** API handler:

```go
// internal/api/handler.go
func (h *Handler) SomeEndpoint(c *gin.Context) {
    result, err := h.Analyzer.Analyze(c.Request.Context(), doc)
    // ...
}
```

---

## Database Schema Versions

| Version | File | Description |
|---------|------|-------------|
| 1 | 001_initial_schema.sql | documents, summaries tables |
| 2 | 002_topic_summaries.sql | topic_summaries, topic_documents |
| 3 | 003_topic_source.sql | source column for topics |
| 4 | 004_add_is_source.sql | is_source flag for hierarchy |
| 5 | 005_tag_clustering.sql | global_tags, tag_clusters tables |
| 6 | 006_hot_cold_tier.sql | status, hot_score, query_count columns |
| 7 | 007_add_embedding.sql | embedding column for vector search |

---

## Configuration

Key configuration options in `configs/config.yaml`:

```yaml
server:
  port: 8080
  web_dir: "./web/dist"  # Frontend static files

llm:
  provider: openai
  openai:
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    model: gpt-4o-mini
    temperature: 0.3

quota:
  per_hour: 100  # Hot documents per hour

documents:
  tiering:
    hot_content_threshold: 5000  # Min chars for hot tier
    cold_promotion_threshold: 3  # Query count for promotion

memory_index:
  vector_dimension: 384
  hnsw_m: 16
  hnsw_ef_construction: 200
  hnsw_ef_search: 100

jobs:
  indexer_interval: 1m
  tag_group_interval: 30m
  promote_interval: 5m
  hotscore_interval: 1h
```
