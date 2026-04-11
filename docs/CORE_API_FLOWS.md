# Core REST API flows and algorithms

This document traces **non-trivial** REST endpoints: anything beyond simple list/get of stored rows. It follows the call chain from `internal/api` into `internal/service/svcimpl` and related storage.

**Simple CRUD / pass-through (not detailed here)**  
`GET /api/v1/documents`, `GET /api/v1/documents/:id`, `GET /api/v1/documents/:id/summaries`, `GET /api/v1/documents/:id/chapters`, `GET /api/v1/tags/groups`, `GET /api/v1/quota`, `GET /api/v1/metrics`, `GET /health` — mostly read from DB or Prometheus without multi-step domain logic.

---

## 1. `POST /api/v1/documents` — Ingest (hot / cold)

**Handler:** `Handler.CreateDocument` → `ExecuteIngestDocument` → `DocumentSvc.Ingest` (`internal/service/svcimpl/document.go`).

### 1.1 Hot vs cold decision (`shouldBeHot`)

1. `**force_hot`** → always hot.
2. Else if **prebuilt summary and chapters** (`req.Summary != ""` and `len(req.Chapters) > 0`) → hot (external agent supplied structure).
3. Else **quota**: `QuotaManager.CheckAndConsume()`; if it fails → **cold**.
4. Else if `**len(content) > HotContentThreshold()`** (config, default 5000) → hot.
5. Otherwise → **cold**.

### 1.2 Hot path

- Build `types.Document` with `status = hot`.  
- **Branches:**  
  - Prebuilt summary + tags: merge into `DocumentAnalysisResult`, call `**IndexerSvc.Index`** only.  
  - Prebuilt tags only: `**SummarizerSvc.AnalyzeDocument`**, merge tags, then `**Index`**.  
  - Neither: full `**AnalyzeDocument**`, then `**Index**`.
- `**Index**` (`internal/service/svcimpl/indexer.go`): delete old summaries; write document-tier summary; for each chapter write chapter summary + `path/source` row for raw chapter text.  
- For each tag: `**TagRepo.Create**` + `**IncrementDocumentCount**` (global L2 tags).

### 1.3 Cold path

- `status = cold`, empty tags.  
- `**DocumentSvc**` (cold): `**coldIndex.AddDocument(ctx, doc)**` (`storage.IColdIndex`): implementation splits markdown (`**cold_index.markdown.chapter_max_tokens**` / optional `**coldindex.Index.SetColdChapterSplitter**`) and indexes content; optional `**coldindex.Index.SetTextEmbedder**` at startup supplies the same embedding stack as `cold_index.embedding`.  
- If a leaf body still exceeds the token budget, it is split with **sliding windows**: each window is up to the token budget wide; the next window starts **`cold_index.markdown.sliding_stride_tokens`** later (default **100** tokens, same rune/token estimate), so overlap ≈ budget − stride. Paths are parent heading path + **`1`**, **`2`**, …; with no heading path, synthetic **`__root__`** is used (e.g. `docId/__root__/1`).  
- Persist document via `**DocRepo.Create**`.

### 1.4 Response

Returns `CreateDocumentResponse` (id, title, format, tags, summary preview fields, chapter count, status, timestamps).

---

## 2. `POST /api/v1/query/progressive` — Progressive query

**Handler:** `Handler.ProgressiveQuery` → `ExecuteProgressiveQuery` → `QuerySvc.ProgressiveQuery` (`internal/service/svcimpl/query.go`).

### 2.1 Parallel paths

Two goroutines run concurrently:


| Path     | Purpose                                               |
| -------- | ----------------------------------------------------- |
| **Hot**  | Tag → document → chapter narrowing with LLM (and DB). |
| **Cold** | Cold index hybrid search over cold documents.       |


Results are **merged** (`mergeHotAndColdResults`): hot entries win by document id; duplicate ids boost relevance; sort by relevance; cap at `max_results`.

### 2.2 Hot path (`queryHotPath`)

1. `**filterL2Tags(question)`** — adaptive:
  - If global tag count **< `L2TagThreshold` (200)**: LLM `**FilterL2TagsByQuery`** on all L2 tags (`filterL2TagsDirect`).  
  - Else: LLM `**FilterL1GroupsByQuery`** → pick groups with relevance **≥ 0.5**, up to **3** → load L2 tags in those groups → `**FilterL2TagsByQuery`** on that subset (`filterL2TagsTwoLevel`).  
  - Relevant tag names: filter results with relevance **≥ 0.5**. Fallbacks if LLM or repos fail.
2. `**queryAndFilterDocuments`**
  - If no tag names: `**DocRepo.ListAll(limit)`** as fallback.  
  - Else: `**DocRepo.ListByTags`** (OR over tags).  
  - Split **hot** vs **cold** in the candidate set:  
    - Hot: `**Summarizer.FilterDocuments`**; keep docs with relevance **≥ 0.5**.  
    - Cold: `**filterColdDocuments`** — `ExtractKeywords` from query, substring match on title/content/tags.
3. `**trackDocumentAccess`** (async per doc): increment query count; if cold and count reaches `**ColdPromotionThreshold`**, enqueue `**job.PromoteQueue**`.
4. `**queryAndFilterChapters**`
  - Hot: load chapter-tier summaries per doc from `**SummaryRepo.QueryByTierAndPrefix**`.  
  - Cold: `**createColdDocumentChapter**` — returns the **full** cold document body as one pseudo-chapter (`path` `docId/full`) when cold docs appear on the hot path (no keyword snippet).  
  - `**Summarizer.FilterChapters**`; keep **≥ 0.5** relevance.
5. `**buildResults`** → `[]QueryItem` (chapter tier, paths, status from doc map).

### 2.3 Cold path (`queryColdPath`)

- If no cold index → empty step.  
- `**QuerySvc**`: `**coldIndex.Search(ctx, question, max_results/2)**` — the index applies optional semantic ranking internally when a text embedder was wired at startup (see §5).  
- Map each hit to a `**QueryItem**`: `path` and `content` come from the cold index hit (`ColdIndexHit` fields); legacy empty path falls back to `docId/full`. `status=cold`.

### 2.4 Answer field (`generateProgressiveAnswer`)

`**internal/service/svcimpl/progressive_answer.go**`: builds a prompt with up to 30 references, excerpts capped (~6KB UTF-8 each), instructs Markdown + `[^N^]` citations; `**ILLMProvider.Generate**` using configured `max_tokens`. On failure, `answer` is empty (UI may fall back).

---

## 3. `POST /api/v1/tags/group` — Tag grouping (L1)

**Handler:** `ExecuteTriggerTagGroup` → `TagGroupSvc.GroupTags` (`internal/service/svcimpl/tag_grouping.go`).

1. `**TagRepo.List`** all global tags.
2. `**performGrouping`**: LLM returns JSON groups → `[]TagGroup` (name, description, member tag names).
3. `**GroupRepo.DeleteAll`** then create each group.
4. For each tag name in a group: `**GetByName**`, set `GroupID`, `**TagRepo.Create**` (implementation note: relies on create path for assignment).
5. Updates in-memory refresh bookkeeping for `**ShouldRefresh**`.

Scheduled `**TagGroupJob**` runs the same service on an interval.

---

## 4. Hot retrieval family (`GET /api/v1/hot/...`)

**Handlers:** `ExecuteHotDocSummaries`, `ExecuteHotDocChapters`, `ExecuteHotDocSource` (`internal/api/handler_execute.go`).  
**Data:** `**IRetrievalService**` (`RetrievalSvc` → `DocRepo` + `SummaryRepo` under the hood; no LLM in these endpoints).


| Endpoint                 | Algorithm                                                                                                                                                                                             |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `**/hot/doc_summaries`** | Require `tags`. `**ListMetaByTagsAndStatuses`** for `hot` + `warming`, cap `max_results`. Load document-tier summaries by doc IDs; join into `{ document_id, title, format, status, tags, summary }`. |
| `**/hot/doc_chapters`**  | Require `doc_ids` (trimmed to `max_results` doc cap). For each id: `**QueryByTierAndPrefix**` chapter tier, skip `IsSource`, return path/title/summary.                                               |
| `**/hot/doc_source**`    | Require `chapter_paths`. `**ListSourcesByPaths**`, cap count; normalize `chapter_path` (strip `/source` suffix in JSON).                                                                              |


---

## 5. `GET /api/v1/cold/doc_source` — Cold hybrid search

**Design (algorithms, indexing, merge, config):** [COLD_INDEX.md](COLD_INDEX.md) · [COLD_INDEX_zh.md](COLD_INDEX_zh.md)（中文）

**Handler:** `ExecuteColdDocSource` → `**IRetrievalService.SearchColdByQuery**` → `**IColdIndex.Search**` (`internal/storage/coldindex/index.go`).

1. Parse `**q`** as comma-separated terms → single query string.
2. `**SearchColdByQuery**` maps `**ColdIndexHit**` rows to `**types.ColdSearchHit**` for the HTTP layer. Under the hood, `**IColdIndex.Search(ctx, queryText, topK)**` — if `**coldindex.Index.SetTextEmbedder**` was called at startup (`cmd/main.go` after `**NewTextEmbedderFromViper**`), the index may rank using additional signals internally; otherwise search is text-only.
3. Inside `**Search**`, the implementation may merge lexical and optional semantic indexes:
  - Each branch retrieves more candidates than the final **topK** (pool size from **cold_index.search**: `branch_recall_multiplier`, `branch_recall_floor`, `branch_recall_ceiling`), then merge so overlap can surface in the final cut.  
  - Text index branch — each hit is one **cold chapter**; `context` is the **full** chapter text (no keyword windowing).
  - Vector branch when embedding length matches `VectorDimension` — same chapter-level hits.
  - `**mergeHybridResults`**: dedupe by **`document_id` + `path`** (not by document alone); normalized score blend; combined rows get `source: hybrid`; sort by score; keep **`topK`**.
4. Handler maps `**types.ColdSearchHit**` rows to JSON (`document_id`, optional `path`, `title`, `score`, `context`, optional `source` for UI/debug trace only).

---

## 6. `GET /api/v1/tags` — Filtered tag list (lightweight core)

**Handler:** `ExecuteListTags`.

- `**IRetrievalService.ListTags**`: if `**group_ids**` non-empty, `**ListByGroupIDs**` with `max_results` (defaults/clamps per handler); else `**TagRepo.List**` with optional cap from `max_results` (implemented in `**RetrievalSvc**`).

No LLM; included because behavior differs from a single-table dump when `group_ids` is set.

---

## Related code map


| Concern                   | Primary files                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------- |
| HTTP + shared REST bodies | `internal/api/handler.go`, `internal/api/handler_execute.go` (read paths via `**IRetrievalService**`) |
| Ingest + tiering          | `internal/service/svcimpl/document.go`, `internal/config/tiering.go`                   |
| Progressive query         | `internal/service/svcimpl/query.go`, `progressive_answer.go`                           |
| Summaries persistence     | `internal/service/svcimpl/indexer.go`, `internal/storage/db/repository.go`             |
| Tag grouping              | `internal/service/svcimpl/tag_grouping.go`                                             |
| Cold index (Bleve + HNSW)   | `internal/storage/coldindex/index.go`                                                     |
| Cold index algorithms     | [COLD_INDEX.md](COLD_INDEX.md), [COLD_INDEX_zh.md](COLD_INDEX_zh.md)                    |
| Cold embeddings           | `coldindex.NewTextEmbedderFromViper` + `**coldindex.Index.SetTextEmbedder**` in `cmd/main.go`; `**storage.IColdIndex**` exposes only documents + text `**Search**` / `**ColdIndexHit**` |
| Promotion side effect     | `job.PromoteQueue` → `IDocumentMaintenanceService.PromoteColdDocumentByID`; sweep in `svcimpl/document_maintenance.go` |


---

*Last aligned with service implementation in `internal/service/svcimpl` and `internal/api`; if behavior diverges, treat source code as authoritative.*