# Core REST API flows and algorithms

This document traces **non-trivial** REST endpoints: anything beyond simple list/get of stored rows. It follows the call chain from `internal/api` into `internal/service/impl` and related storage.

**Full auth design:** roles, scopes, dual-track model, DB tables, and config are in **[../design/auth-and-permissions.md](../design/auth-and-permissions.md)**. End-user steps are in **[../getting-started/installation.md](../getting-started/installation.md)** and the root README.

**Mount points and auth:** The same `Handler.RegisterRoutes` surface is mounted at **`/api/v1`** (program track: **`api.ProgramAuthMiddleware`** → `service.IProgramAuth` / `authimpl.NewProgramAuth`: DB API keys with scopes `read` | `write` | `admin`, `X-API-Key` or `Authorization: Bearer`) and at **`/bff/v1`** for the embedded UI (human track: **`api.BFFSessionMiddleware`** + HttpOnly **`tiersum_session`** cookie after `POST /bff/v1/auth/login` or `POST /bff/v1/auth/device_login`, then **`api.BFFHumanRBAC`**: `viewer` is read-only except **`POST /bff/v1/query/progressive`**; **`GET /bff/v1/monitoring`** and **`GET /bff/v1/traces*`** require human **`admin`**). Optional HttpOnly **`tiersum_device`** cookie stores a persistent **device token** (`ts_d_*`, DB row in `device_tokens`) for “keep me signed in” / quick re-login. Until bootstrap, **`IsSystemInitialized`** is false: **`/api/v1/*`** returns **403** JSON `{ "code": "SYSTEM_NOT_INITIALIZED" }`; protected **`/bff/v1/*`** (everything except small public auth paths) returns the same or **401** when unauthenticated. Paths below use **`/api/v1`**; use **`/bff/v1`** for the same handlers behind the session cookie. **Probes / metrics:** **`GET /health`** and **`GET /metrics`** stay at the **server root** and are not gated by either track.

**Bootstrap (first boot):** `GET /bff/v1/system/status` → `{ initialized, version }`. `POST /bff/v1/system/bootstrap` `{ "username" }` (only when `initialized=false`) → `IAuthService.Bootstrap` (wired by `authimpl.NewAuthService`): creates first admin user (hashed `ts_u_*` access token), one read-scoped `tsk_live_*` API key, sets `system_state.initialized_at`, returns plaintext secrets once. Initialization cannot be repeated.

**Browser login:** `POST /bff/v1/auth/login` `{ "access_token", "fingerprint": { "timezone", "client_signal?" }, "remember_me?": bool, "device_name?" }` → `IAuthService.LoginWithAccessToken` validates hashed access token, enforces per-user **max_devices** against active distinct fingerprints, stores **`browser_sessions`** (hashed opaque session cookie value), sets **`Set-Cookie`**. When `remember_me=true`, the server mints a **device token** (`IAuthService.CreateDeviceTokenForSession`) and sets HttpOnly **`tiersum_device`** (`auth.browser.device_token_ttl`).

**Device login (persistent token):** `POST /bff/v1/auth/device_login` `{ "device_token?", "fingerprint" }` (or omit `device_token` and rely on the **`tiersum_device`** cookie) → `IAuthService.DeviceLogin` validates `device_tokens` row (not revoked / not expired + same loose IP/UA binding rules as sessions), then creates a fresh **`browser_sessions`** row and sets **`tiersum_session`**.

**WebAuthn passkeys (browser):** under **`/bff/v1/me/security/passkeys/*`** (session required) the UI can register and verify passkeys via `IAuthService.Begin/FinishPasskeyRegistration` and `Begin/FinishPasskeyVerification` (implemented with `github.com/go-webauthn/webauthn`, in-memory WebAuthn ceremony sessions, persisted credentials in `passkey_credentials`, and “recently verified” state in `passkey_session_verifications`). Successful registration or verification upserts a short-lived verification row keyed by **browser session id** (`auth.passkey.session_verification_ttl`).

**Passkey-gated admin APIs:** `POST /bff/v1/admin/*` additionally runs `api.BFFRequireAdminPasskey`: when `auth.passkey.admin_required=true` **and** the admin has at least one passkey, admin routes require a **non-expired** `passkey_session_verifications` row for the current session (use **Verify passkey** in `/settings`, or register a passkey which also marks the session verified).

Subsequent authenticated browser requests use **`IAuthService.ValidateBrowserSession`** (loose IP/UA consistency + sliding session and user token TTL when configured).

**BFF request hardening (browser track):**

- **Human RBAC** (`api.BFFHumanRBAC`): after the session cookie is validated, **`users.role`** gates write-style traffic for **`viewer`** and gates observability reads for non-**`admin`** (see **[../design/auth-and-permissions.md](../design/auth-and-permissions.md)**).
- **CSRF**: all non-GET/HEAD/OPTIONS requests under `/bff/v1` are protected by a same-origin check (`api.BFFSameOriginMiddleware`). The server requires `Origin` or `Referer` to match the current request host (or be allowlisted via `auth.browser.csrf.allowed_origins`). This defends cookie-authenticated endpoints from cross-site form / fetch abuse.
- **Rate limiting**: `POST /bff/v1/system/bootstrap`, `POST /bff/v1/auth/login`, and `POST /bff/v1/auth/device_login` are IP rate-limited in-process. Login additionally applies an exponential cooldown after repeated failures (`try_later` with `retry_after`).
- **Secure cookie behind proxies**: session cookie `Secure` attribute is controlled by `auth.browser.cookie_secure_mode` (`auto|always|never`). In `auto`, the server sets `Secure` when the request is TLS, or when `auth.browser.trust_proxy_headers=true` and `X-Forwarded-Proto=https` / `X-Forwarded-Ssl=on` is present (reverse-proxy TLS termination).
- **Recommended public deployment**: run Nginx and TierSum on the same host, terminate TLS at Nginx, and bind TierSum to `127.0.0.1` (`server.host: "127.0.0.1"`). This makes the Go server unreachable from the public internet except via the proxy.

**Admin (browser, admin role only):** under **`/bff/v1/admin/*`** with `BFFRequireAdmin` **and** `BFFRequireAdminPasskey` (see passkey gating above): users CRUD tokens, **`GET /bff/v1/admin/devices`** (all users’ browser sessions with usernames), per-user devices at **`GET /bff/v1/admin/users/:id/devices`**, API keys list/create/revoke, usage snapshot `GET /bff/v1/admin/api_keys/usage`, **`GET /bff/v1/admin/config/snapshot`** (read-only merged `viper` settings with `api_key` / `dsn` / `password` / … leaves redacted — **Management → Configuration** in the UI at **`/admin/config`**). **`/bff/v1/me/*`**: profile, devices, alias, revoke sessions (admins may PATCH/DELETE another user’s session id here for support). **Management → Security** (`/settings`) uses **`GET /bff/v1/admin/devices`** when the browser profile role is admin so the list covers every user’s bound browsers; otherwise it uses **`GET /bff/v1/me/devices`** (own sessions only), and includes passkey + device-token management calls under **`/bff/v1/me/security/*`**.

**MCP:** each tool calls **`MCPServer.mcpProgramGate`** with the same scope rules as REST; API key is read from **`TIERSUM_API_KEY`** or `mcp.api_key` in config.

**Simple CRUD / pass-through (not detailed here)**  
`GET /api/v1/documents`, `GET /api/v1/documents/:id`, `GET /api/v1/topics`, `GET /api/v1/quota`, **`GET /health`** (root JSON liveness), and **`GET /metrics`** (root Prometheus text) — mostly read from DB or Prometheus without multi-step domain logic. **`/health`** and **`/metrics`** remain public.

`GET /api/v1/documents/:id/chapters` is detailed below (cold docs always markdown-derived; hot docs use DB rows with markdown fallback when empty).

---

## How to read the call trees

Each endpoint is rendered as an **ASCII call tree** (Jaeger-style span hierarchy).

```text
FunctionName (file/path.go:line)
├─ ChildCall (file/path.go:line)          ← synchronous, sequential
├─ [conditional: expr]
│  └─ BranchA (file/path.go:line)
└─ [else]
   └─ BranchB (file/path.go:line)
```

Annotations:
- **`[conditional: …]`** — runtime branch predicate.
- **`[goroutine]`** — runs concurrently with siblings at the same tree level.
- **`[async]`** — fire-and-forget (queued or background goroutine, caller does not wait).
- **`[loop]`** — iterated over a collection.
- **File paths** are relative to repository root and point to the primary implementation file.

---

## 0. `GET /api/v1/documents/:id/chapters` — sections for detail UI

```text
Handler.GetDocumentChapters
└─ Handler.ExecuteListDocumentChaptersByDocumentID (internal/api/handler_execute.go:79)
   ├─ DocService.GetDocument (internal/service/impl/document/document_service_impl.go:269)
   │  └─ docs.GetByID (internal/storage/db/document/document_repository_impl.go)
   ├─ [conditional: doc.Status == cold]
   │  └─ ChaptersService.ExtractChaptersFromMarkdown (internal/service/impl/catalog/chapter_service_impl.go:57)
   │     ├─ [conditional: coldIndex != nil]
   │     │  └─ coldIndex.MarkdownChapters (internal/storage/coldindex/cold_index_impl.go:672)
   │     │     └─ MarkdownChaptersFromSplit (internal/storage/coldindex/markdown_chapter_splitter_impl.go)
   │     └─ [else: coldIndex == nil]
   │        └─ pkg/markdown.ChapterDisplayTitle (pkg/markdown/...)
   └─ [else: hot / warming]
      ├─ ChaptersService.ListChaptersByDocumentID (internal/service/impl/catalog/chapter_service_impl.go:53)
      │  └─ chapterRepo.ListByDocument (internal/storage/db/document/chapter_repository_impl.go)
      └─ [conditional: len(chapters) == 0]
         └─ ExtractChaptersFromMarkdown (fallback, same as cold path above)
```

**Notes:**
- **Cold** documents always use `ExtractChaptersFromMarkdown` so the UI sees the same merged sections as the cold index (avoids stale partial persisted rows).
- **Hot/warming** loads persisted DB chapter rows first; if none, falls back to markdown extraction.

---

## 1. `POST /api/v1/documents` — Ingest (hot / cold)

```text
Handler.CreateDocument (internal/api/handler.go:106)
└─ Handler.ExecuteCreateDocument (internal/api/handler_execute.go:38)
   └─ DocService.CreateDocument (internal/service/impl/document/document_service_impl.go:53)
      ├─ validateCreateIngest (internal/service/impl/document/document_service_impl.go:149)
      │  ├─ config.DocumentMaxBodyBytes (internal/config/documents_ingest.go)
      │  ├─ config.DocumentFormatAllowed
      │  └─ config.DocumentChunkingMaxChars
      ├─ resolveHotIngest (internal/service/impl/document/document_service_impl.go:166)
      │  ├─ req.EffectiveIngestMode
      │  ├─ [conditional: mode == auto && no prebuilt summary+chapters]
      │  │  └─ quota.CheckAndConsume (internal/service/impl/document/hot_ingest_quota_impl.go)
      │  └─ config.HotContentThreshold (internal/config/tiering.go)
      ├─ [conditional: hot == true] ───────── HOT PATH ─────────
      │  ├─ docs.Create (internal/storage/db/document/document_repository_impl.go)
      │  │  └─ [DB] INSERT documents (status = hot)
      │  ├─ [conditional: len(req.Chapters) > 0 && chapters != nil]
      │  │  ├─ materializePrebuiltChapters
      │  │  └─ chapters.ReplaceByDocument (internal/storage/db/document/chapter_repository_impl.go)
      │  ├─ syncCatalogTags (internal/service/impl/document/document_service_impl.go:248)
      │  │  └─ [loop: tagNames]
      │  │     ├─ tags.GetByName
      │  │     ├─ [conditional: missing] tags.Create
      │  │     └─ tags.IncrementDocumentCount
      │  └─ [conditional: hot && len(req.Chapters) == 0 && hotIngestSink != nil]
      │     └─ hotIngestSink.SubmitHotIngest [async]
      │        └─ (consumed by job.HotIngestQueue)
      │           └─ HotIngestQueueConsumer (internal/job/hot_ingest_consumer.go)
      │              └─ IHotIngestProcessor.ProcessHotIngest (internal/service/impl/document/hot_ingest_processor_impl.go:47)
      │                 ├─ docRepo.GetByID
      │                 ├─ [conditional: doc.Status != hot] → skip
      │                 ├─ IDocumentAnalysisGenerator.GenerateAnalysis (LLM call)
      │                 │  └─ [on failure] analysisFailureResult → virtual failure chapter
      │                 └─ IDocumentAnalysisPersister.PersistAnalysis
      │                    ├─ docRepo.UpdateSummary
      │                    └─ chapters.ReplaceByDocument
      ├─ [conditional: hot == false && coldIndex != nil] ── COLD PATH ──
      │  ├─ coldIndex.AddDocument (internal/storage/coldindex/cold_index_impl.go:404)
      │  │  ├─ removeChaptersForDocLocked (dedupe existing doc chapters)
      │  │  ├─ coldSplitter.Split (internal/storage/coldindex/markdown_chapter_splitter_impl.go)
      │  │  └─ [loop: chapters]
      │  │     ├─ [conditional: textEmbedder != nil]
      │  │     │  └─ FallbackColdTextEmbedding
      │  │     └─ [else] GenerateSimpleEmbedding (coldvec)
      │  │     ├─ inverted.indexChapter (Bleve)
      │  │     └─ vector.add (HNSW)
      │  │  └─ docChapterPaths[doc.ID] = paths
      │  └─ docs.Create
      └─ [conditional: hot == false && coldIndex == nil]
         └─ docs.Create
      └─ docs.GetByID (reload for response)
```

**Key decisions:**
- `resolveHotIngest` decides the path: `hot` always, `cold` always, `auto` uses prebuilt analysis → quota + content length threshold.
- **Cold path order:** index first, then DB; if DB fails, index is rolled back via `RemoveDocument`.
- **Hot ingest queue:** bounded capacity (100); when full, work is dropped with a warning (manual retry needed).

---

## 2. `POST /api/v1/query/progressive` — Progressive query

```text
Handler.ProgressiveQuery (internal/api/handler.go:139)
└─ Handler.ExecuteProgressiveQuery (internal/api/handler_execute.go:144)
   └─ QueryService.ProgressiveQuery (internal/service/impl/query/query_service_impl.go:256)
      ├─ [goroutine] queryHotPath (internal/service/impl/query/query_service_impl.go:417)
      │  └─ WithOptionalSpan("hot_chapter_search")
      │     └─ chapterSearch.SearchHotChapters (internal/service/impl/catalog/chapter_service_impl.go:118)
      │        └─ searchHotChaptersProgressive (internal/service/impl/catalog/search_hot_progressive.go:19)
      │           ├─ filterCatalogTags
      │           │  ├─ TagRepo.List
      │           │  ├─ [conditional: tagCount < 200]
      │           │  │  └─ FilterTagsByQuery (LLM via hotProgressiveLLMCore)
      │           │  └─ [else]
      │           │     ├─ TopicRepo.List
      │           │     ├─ FilterTopicsByQuery (LLM, max 3, ≥0.5)
      │           │     ├─ TagRepo.ListByTopic per selected topic
      │           │     └─ FilterTagsByQuery
      │           ├─ queryAndFilterDocumentsForHotSearch
      │           │  ├─ DocRepo.ListByTags (OR) OR DocRepo.ListAll fallback
      │           │  ├─ [split: hot/warming vs cold in candidate set]
      │           │  │  ├─ hot/warming candidates → FilterDocuments (LLM, ≥0.5)
      │           │  │  └─ cold candidates → ExtractKeywords substring match
      │           │  └─ merge + rank
      │           └─ queryAndFilterChaptersForHotSearch
      │              ├─ ChapterRepo.ListByDocument (hot/warming docs)
      │              ├─ cold candidates → pseudo-chapter (docId/full)
      │              └─ FilterChapters (LLM, ≥0.5)
      │  └─ trackDocumentAccess [async goroutine per distinct docID]
      │     ├─ docRepo.IncrementQueryCount
      │     └─ [conditional: status == cold && queryCount+1 >= threshold]
      │        └─ job.PromoteQueue <- docID
      ├─ [goroutine] queryColdPath (internal/service/impl/query/query_service_impl.go:534)
      │  └─ WithOptionalSpan("cold_index_search")
      │     └─ chapterSearch.SearchColdChapterHits (internal/service/impl/catalog/chapter_service_impl.go:94)
      │        └─ coldIndex.Search (internal/storage/coldindex/cold_index_impl.go:498)
      │           ├─ [conditional: textEmbedder != nil]
      │           │  └─ FallbackColdTextEmbedding(queryText)
      │           └─ hybridSearch
      │              ├─ searchWithBleve (BM25, recall = branchRecallSize(topK))
      │              │  └─ inverted.search (Bleve)
      │              ├─ [conditional: queryEmbedding valid dimension]
      │              │  └─ searchWithVector (HNSW cosine, recall = branchRecallSize(topK))
      │              │     └─ vector.search (hnsw.Graph)
      │              └─ mergeHybridResults
      │                 ├─ [loop: bm25] normalize score, weight 0.5
      │                 ├─ [loop: vector] weight 0.5, merge or add
      │                 └─ sort by score + topK trim
      ├─ mergeHotAndColdQueryItems (dedupe by docID+path, sort relevance, cap maxResults)
      └─ generateProgressiveAnswer
         ├─ buildProgressiveAnswerPrompt (up to 30 refs, ~6KB excerpts each)
         └─ [conditional: llm != nil && len(items) > 0]
            └─ llm.Generate
               └─ [on failure] answer = ""
```

**Notes:**
- Two goroutines run **concurrently**; results merge after both complete.
- **Hot path** uses LLM filtering at three hops (tags → documents → chapters) when `relCore` (LLM) is available; without it, hot returns empty.
- **Cold path** silently returns empty if the cold index is unavailable (`ErrColdIndexUnavailable`), without failing the overall request.
- **Track access** fires background goroutines per document to increment query counts and enqueue cold documents for promotion when threshold is reached.
- **Answer synthesis** is optional: if LLM fails or is absent, `answer` is empty and the UI may render raw results.

---

## 3. `POST /api/v1/topics/regroup` — Topic regrouping (catalog tags → themes)

```text
Handler.RegroupTagsIntoTopics (internal/api/handler.go:157)
└─ Handler.ExecuteRegroupTagsIntoTopics (internal/api/handler_execute.go:183)
   └─ TopicService.RegroupTags (internal/service/impl/catalog/topic_service_impl.go:46)
      ├─ TagRepo.List
      ├─ [conditional: len(tags) == 0]
      │  └─ no-op (update refresh bookkeeping only)
      └─ [deterministic impl: single catch-all topic]
         ├─ TopicRepo.DeleteAll
         ├─ TopicRepo.Create (topic "All tags")
         └─ [loop: tags]
            └─ TagRepo.Create (upsert with TopicID)
```

**Scheduled job:** `TopicRegroupJob` (`internal/job/jobs.go`) runs the same path on an interval when `ShouldRefresh` is true.

---

## 4. Hot retrieval family (`GET /api/v1/hot/...`)

### 4.1 `GET /api/v1/hot/doc_summaries`

```text
Handler.ListHotDocumentSummariesByTags (internal/api/handler_catalog.go)
└─ Handler.ExecuteListHotDocumentSummariesByTags (internal/api/handler_execute.go:222)
   └─ DocService.ListHotDocumentsWithSummariesByTags (internal/service/impl/document/document_service_impl.go:303)
      └─ docs.ListMetaByTagsAndStatuses (internal/storage/db/document/document_repository_impl.go)
         └─ [DB] SELECT hot + warming by tags (OR), cap max_results
```

### 4.2 `GET /api/v1/hot/doc_chapters`

```text
Handler.ListHotDocumentChaptersByDocumentIDs (internal/api/handler_catalog.go)
└─ Handler.ExecuteListHotDocumentChaptersByDocumentIDs (internal/api/handler_execute.go:250)
   └─ ChaptersService.ListChaptersByDocumentIDs (internal/service/impl/catalog/chapter_service_impl.go:79)
      └─ chapterRepo.ListByDocumentIDs (internal/storage/db/document/chapter_repository_impl.go)
         └─ [DB] SELECT chapters WHERE document_id IN (...)
```

**Note:** These are DB-only reads; no LLM or cold index is involved.

---

## 5. `GET /api/v1/cold/chapter_hits` — Cold hybrid search

**Design (algorithms, indexing, merge, config):** [cold-index/cold-index.md](cold-index/cold-index.md) · [cold-index/cold-index.zh.md](cold-index/cold-index.zh.md)（中文）

```text
Handler.SearchColdChapterHits (internal/api/handler.go)
└─ Handler.ExecuteSearchColdChapterHits (internal/api/handler_execute.go:283)
   └─ ChaptersService.SearchColdChapterHits (internal/service/impl/catalog/chapter_service_impl.go:94)
      ├─ [conditional: coldIndex == nil] → return ErrColdIndexUnavailable
      └─ coldIndex.Search (internal/storage/coldindex/cold_index_impl.go:498)
         ├─ [conditional: textEmbedder != nil]
         │  └─ FallbackColdTextEmbedding(queryText)
         └─ hybridSearch (internal/storage/coldindex/cold_index_impl.go:586)
            ├─ searchWithBleve (recall = branchRecallSize(topK))
            │  └─ inverted.search (Bleve BM25)
            ├─ [conditional: queryEmbedding valid dimension]
            │  └─ searchWithVector (recall = branchRecallSize(topK))
            │     └─ vector.search (hnsw.Graph cosine)
            └─ mergeHybridResults
               ├─ [loop: bm25 hits] normalize score (maxBM25), weight 0.5, source="bm25"
               ├─ [loop: vector hits] weight 0.5
               │  ├─ [conditional: path already in map]
               │  │  └─ blend scores (existing + vector*0.5), source="hybrid"
               │  └─ [else] add new, source="vector"
               └─ sort by blended score + topK trim
      └─ [map] ColdIndexHit → types.ColdSearchHit for JSON
```

**Key config:**
- `cold_index.search.branch_recall_multiplier` (default 2)
- `cold_index.search.branch_recall_floor` (default 20)
- `cold_index.search.branch_recall_ceiling` (default 200)

---

## 6. `GET /api/v1/tags` — Filtered tag list (lightweight core)

```text
Handler.ListTags (internal/api/handler.go)
└─ Handler.ExecuteListTags (internal/api/handler_execute.go:195)
   └─ TagsService.ListTags (internal/service/impl/catalog/tag_service_impl.go)
      ├─ [conditional: len(topicIDs) > 0]
      │  └─ TagRepo.ListByTopicIDs (internal/storage/db/document/tag_repository_impl.go)
      └─ [else]
         └─ TagRepo.List (internal/storage/db/document/tag_repository_impl.go)
```

No LLM; included because behavior differs from a single-table dump when `topic_ids` is set.

---

## Related code map

| Concern                   | Primary files                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------- |
| HTTP + shared REST bodies | `internal/api/handler.go`, `internal/api/handler_execute.go`, `internal/api/handler_catalog.go` |
| Ingest + tiering          | `internal/service/impl/document/document_service_impl.go`, `internal/service/impl/document/hot_ingest_quota_impl.go`, `internal/config/tiering.go`, `internal/config/documents_ingest.go`; hot-ingest async: `internal/service/impl/document/hot_ingest_processor_impl.go`, `internal/job/scheduler.go` (`HotIngestQueue`), `internal/job/hot_ingest_consumer.go`, `cmd/main.go` (`StartHotIngestQueueConsumer`) |
| Progressive query         | `internal/service/impl/query/query_service_impl.go`; hot/cold chapter reads `internal/service/impl/catalog/chapter_service_impl.go` |
| Document summary + chapter rows | Service persister (behind jobs/ingest via `internal/service/impl/document.IDocumentAnalysisPersister`), `internal/storage/db/document/document_repository_impl.go` / `chapter_repository_impl.go` (documents.summary + chapters table) |
| Topic regroup + list      | `internal/service/impl/catalog/topic_service_impl.go`, `internal/job/jobs.go` (`TopicRegroupJob`) |
| Cold index (Bleve + HNSW)   | `internal/storage/coldindex/cold_index_impl.go`                                           |
| Cold index algorithms     | [cold-index/cold-index.md](cold-index/cold-index.md), [cold-index/cold-index.zh.md](cold-index/cold-index.zh.md)                    |
| Cold embeddings           | `coldindex.NewTextEmbedderFromViper` + `**coldindex.Index.SetTextEmbedder**` in `cmd/main.go`; `**storage.IColdIndex**` exposes only documents + text `**Search**` / `**ColdIndexHit**` |
| Promotion side effect     | `job.PromoteQueue` → `IDocumentMaintenanceService.PromoteColdDocumentByID`; scheduled sweep `RunColdPromotionSweep` in `internal/service/impl/document/document_maintenance_service_impl.go` |

---

*Last aligned with service implementation under `internal/service/impl` and `internal/api`; if behavior diverges, treat source code as authoritative.*
