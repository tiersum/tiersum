# Core REST API flows and algorithms

This document traces **non-trivial** REST endpoints: anything beyond simple list/get of stored rows. It follows the call chain from `internal/api` into `internal/service/svcimpl` and related storage.

**Full auth design:** roles, scopes, dual-track model, DB tables, and config are in **[docs/AUTH_AND_PERMISSIONS.md](AUTH_AND_PERMISSIONS.md)**. End-user steps are in the root **[README.md](../README.md#access-control-and-permissions-user-guide)**.

**Mount points and auth:** The same `Handler.RegisterRoutes` surface is mounted at **`/api/v1`** (program track: **`api.ProgramAuthMiddleware`** → `service.IProgramAuth` / `svcimpl.AuthSvc`: DB API keys with scopes `read` | `write` | `admin`, `X-API-Key` or `Authorization: Bearer`) and at **`/bff/v1`** for the embedded UI (human track: **`api.BFFSessionMiddleware`** + HttpOnly `tiersum_session` cookie after `POST /bff/v1/auth/login`). Until bootstrap, **`IsSystemInitialized`** is false: **`/api/v1/*`** returns **403** JSON `{ "code": "SYSTEM_NOT_INITIALIZED" }`; protected **`/bff/v1/*`** (everything except small public auth paths) returns the same or **401** when unauthenticated. Paths below use **`/api/v1`**; use **`/bff/v1`** for the same handlers behind the session cookie. **Probes / metrics:** **`GET /health`** and **`GET /metrics`** stay at the **server root** and are not gated by either track.

**Bootstrap (first boot):** `GET /bff/v1/system/status` → `{ initialized, version }`. `POST /bff/v1/system/bootstrap` `{ "username" }` (only when `initialized=false`) → `AuthSvc.Bootstrap`: creates first admin user (hashed `ts_u_*` access token), one read-scoped `tsk_live_*` API key, sets `system_state.initialized_at`, returns plaintext secrets once. Initialization cannot be repeated.

**Browser login:** `POST /bff/v1/auth/login` `{ "access_token", "fingerprint": { "timezone", "client_signal?" } }` → `AuthSvc.LoginWithAccessToken` validates hashed access token, enforces per-user **max_devices** against active distinct fingerprints, stores **`browser_sessions`** (hashed opaque session cookie value), sets **`Set-Cookie`**. Subsequent requests use **`ValidateBrowserSession`** (loose IP/UA consistency + sliding session and user token TTL when configured).

**Admin (browser, admin role only):** under **`/bff/v1/admin/*`** with `BFFRequireAdmin`: users CRUD tokens, **`GET /bff/v1/admin/devices`** (all users’ browser sessions with usernames), per-user devices at **`GET /bff/v1/admin/users/:id/devices`**, API keys list/create/revoke, usage snapshot `GET /bff/v1/admin/api_keys/usage`, **`GET /bff/v1/admin/config/snapshot`** (read-only merged `viper` settings with `api_key` / `dsn` / `password` / … leaves redacted — **Management → Configuration** in the UI at **`/admin/config`**). **`/bff/v1/me/*`**: profile, devices, alias, revoke sessions (admins may PATCH/DELETE another user’s session id here for support). **Management → Devices & sessions** (`/settings`) uses **`GET /bff/v1/admin/devices`** when the browser profile role is admin so the list covers every user’s bound browsers; otherwise it uses **`GET /bff/v1/me/devices`** (own sessions only).

**MCP:** each tool calls **`MCPServer.mcpProgramGate`** with the same scope rules as REST; API key is read from **`TIERSUM_API_KEY`** or `mcp.api_key` in config.

**Simple CRUD / pass-through (not detailed here)**  
`GET /api/v1/documents`, `GET /api/v1/documents/:id`, `GET /api/v1/documents/:id/summaries`, `GET /api/v1/topics`, `GET /api/v1/quota`, **`GET /health`** (root JSON liveness), and **`GET /metrics`** (root Prometheus text) — mostly read from DB or Prometheus without multi-step domain logic. **`/health`** and **`/metrics`** remain public.

`GET /api/v1/documents/:id/chapters` is detailed below (markdown fallback when DB has no chapter-tier rows).

---

## 0. `GET /api/v1/documents/:id/chapters` — sections for detail UI

**Handler:** `ExecuteGetDocumentChapters` → `**ListChapterSummariesForDocument**` (chapter tier, skip `IsSource`). If the result is empty, **`Retrieval.MarkdownChaptersForDocument`** → **`IColdIndex.MarkdownChapters`** (same splitter and `coldChapterMaxTokens` as cold ingest), or `**coldindex.SplitMarkdown**` when no index is wired — so cold documents (and hot documents before summaries exist) still get `path` / `title` / `summary` (section body) for chapter navigation.

---

## 1. `POST /api/v1/documents` — Ingest (hot / cold)

**Handler:** `Handler.CreateDocument` → `ExecuteIngestDocument` → `DocumentSvc.Ingest` (`internal/service/svcimpl/document.go`).

### 1.0 Ingest validation (configurable)

Before hot/cold routing, **`DocumentSvc.Ingest`** enforces **`documents.max_size`** (UTF-8 byte length of `content`), **`documents.supported_formats`** (when non-empty), and optional **`documents.chunking`** (`enabled` + **`max_chunk_size`** as Unicode code points). Violations return **`service.ErrIngestValidation`**, mapped to **HTTP 400** in **`ExecuteIngestDocument`**.

### 1.1 Hot vs cold decision (`shouldBeHot`)

Resolved mode: `**req.EffectiveIngestMode()**` (`ingest_mode` JSON field: `auto` | `hot` | `cold`; legacy `force_hot=true` maps to `hot`).

1. **`hot`** → always hot.
2. **`cold`** → always cold.
3. **`auto`** → if **prebuilt summary and chapters** (`req.Summary != ""` and `len(req.Chapters) > 0`) → hot; else **quota** `QuotaManager.CheckAndConsume()` — if it fails → cold; else if `**len(content) > HotContentThreshold()`** (config, default 5000) → hot; else cold.

### 1.2 Hot path

- Build `types.Document` with `status = hot`.  
- **Branches:**  
  - Prebuilt summary + tags: merge into `DocumentAnalysisResult`, call `**IndexerSvc.Index`** only.  
  - Prebuilt tags only: `**SummarizerSvc.AnalyzeDocument`**, merge tags, then `**Index`**.  
  - Neither: full `**AnalyzeDocument**`, then `**Index**`.
- `**Index**` (`internal/service/svcimpl/indexer.go`): delete old summaries; write document-tier summary; for each chapter write chapter summary + `path/source` row for raw chapter text.  
- For each tag: `**TagRepo.Create**` + `**IncrementDocumentCount**` (catalog tag rows: deduplicated names with document counts).

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

1. `**filterCatalogTags(question)`** — adaptive (`**CatalogTagThreshold**` = 200 in `query.go`):
  - If **catalog tag** count **< threshold**: `**filterTagsDirect`** — LLM `**Summarizer.FilterTagsByQuery**` on all catalog tags.  
  - Else: `**filterTagsViaTopics`** — `**filterTopics**` (`**Summarizer.FilterTopicsByQuery`** on topics, relevance **≥ 0.5**, up to **3**) → `**getTagsFromTopics**` → `**filterTagsDirect**` on that tag subset.  
  - Relevant tag names: filter results with relevance **≥ 0.5** (`**extractRelevantTags**`). Fallbacks if LLM or repos fail.
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

## 3. `POST /api/v1/topics/regroup` — Topic regrouping (catalog tags → themes)

**Handler:** `ExecuteTriggerTopicRegroup` → `TopicSvc.RegroupTags` (`internal/service/svcimpl/topic.go`).

1. `**TagRepo.List`** all catalog tags.
2. `**performGrouping`**: LLM returns JSON topics → `[]Topic` (name, description, member tag names).
3. `**TopicRepo.DeleteAll`** then create each topic row.
4. For each tag name in a topic: `**TagRepo.GetByName**`, set `TopicID`, `**TagRepo.Create**` (implementation note: relies on create path for assignment).
5. Updates in-memory refresh bookkeeping for `**ShouldRefresh**`.

`GET /api/v1/topics` lists persisted topics (`**TopicSvc.ListTopics**` → `**TopicRepo.List**`).

Scheduled `**TopicRegroupJob**` (`internal/job/jobs.go`) runs the same regroup path on an interval when `**ShouldRefresh**` is true.

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

- `**IRetrievalService.ListTags**`: if `**topic_ids**` non-empty, `**TagRepo.ListByTopicIDs**` with `max_results` (defaults/clamps per handler); else `**TagRepo.List**` with optional cap from `max_results` (implemented in `**RetrievalSvc**`).

No LLM; included because behavior differs from a single-table dump when `topic_ids` is set.

---

## Related code map


| Concern                   | Primary files                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------- |
| HTTP + shared REST bodies | `internal/api/handler.go`, `internal/api/handler_execute.go` (read paths via `**IRetrievalService**`) |
| Ingest + tiering          | `internal/service/svcimpl/document.go`, `internal/config/tiering.go`                   |
| Progressive query         | `internal/service/svcimpl/query.go`, `progressive_answer.go`                           |
| Summaries persistence     | `internal/service/svcimpl/indexer.go`, `internal/storage/db/repository.go`             |
| Topic regroup + list      | `internal/service/svcimpl/topic.go`, `internal/job/jobs.go` (`TopicRegroupJob`)       |
| Cold index (Bleve + HNSW)   | `internal/storage/coldindex/index.go`                                                     |
| Cold index algorithms     | [COLD_INDEX.md](COLD_INDEX.md), [COLD_INDEX_zh.md](COLD_INDEX_zh.md)                    |
| Cold embeddings           | `coldindex.NewTextEmbedderFromViper` + `**coldindex.Index.SetTextEmbedder**` in `cmd/main.go`; `**storage.IColdIndex**` exposes only documents + text `**Search**` / `**ColdIndexHit**` |
| Promotion side effect     | `job.PromoteQueue` → `IDocumentMaintenanceService.PromoteColdDocumentByID`; sweep in `svcimpl/document_maintenance.go` |


---

*Last aligned with service implementation in `internal/service/svcimpl` and `internal/api`; if behavior diverges, treat source code as authoritative.*