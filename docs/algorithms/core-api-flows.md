# Core REST API flows and algorithms

This document traces **non-trivial** REST endpoints: anything beyond simple list/get of stored rows. It follows the call chain from `internal/api` into `internal/service/impl` and related storage.

**Full auth design:** roles, scopes, dual-track model, DB tables, and config are in **[../design/auth-and-permissions.md](../design/auth-and-permissions.md)**. End-user steps are in the root **[README.md](../README.md#access-control-and-permissions-user-guide)**.

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

## 0. `GET /api/v1/documents/:id/chapters` — sections for detail UI

**Handler:** `ExecuteListDocumentChaptersByDocumentID`. **Cold** documents always use **`IChapterService.ExtractChaptersFromMarkdown`**: when a cold index is wired, **`IColdIndex.MarkdownChapters`** delegates to **`coldindex.MarkdownChaptersFromSplit`** (same splitter / **same `EstimateTokens` budget units** as cold ingest — sized for **vector embedder sequence length**, not LLM prompts; see **`cold-index/cold-index.md` §2.3**) and section titles use **`pkg/markdown.ChapterDisplayTitle`**; when no cold index is wired, the service returns a single synthetic `{doc_id}/body` chapter from raw markdown with the same title helper. **Hot/warming:** load persisted **`IChapterService.ListChaptersByDocumentID`** rows; if none, fall back to **`ExtractChaptersFromMarkdown`** as above. JSON `summary` is the section body for cold markdown fallback and the stored chapter summary for hot rows.

---

## 1. `POST /api/v1/documents` — Ingest (hot / cold)

**Handler:** `Handler.CreateDocument` → `ExecuteCreateDocument` → `IDocumentService.CreateDocument` (`internal/service/impl/document/document_service_impl.go`).

### 1.0 Ingest validation (configurable)

Before hot/cold routing, **`IDocumentService.CreateDocument`** enforces **`documents.max_size`** (UTF-8 byte length of `content`), **`documents.supported_formats`** (when non-empty), and optional **`documents.chunking`** (`enabled` + **`max_chunk_size`** as Unicode code points). Violations return **`service.ErrIngestValidation`**, mapped to **HTTP 400** in **`ExecuteCreateDocument`**.

### 1.1 Hot vs cold decision

Resolved mode: `**req.EffectiveIngestMode()**` (`ingest_mode` JSON field: `auto` | `hot` | `cold`; legacy `force_hot=true` maps to `hot`).

1. **`hot`** → always hot.
2. **`cold`** → always cold.
3. **`auto`** → hot if **prebuilt summary and chapters** (`req.Summary != ""` and `len(req.Chapters) > 0`); else hot only if the in-process hourly **`HotIngestQuota`** (`quota.per_hour`, wired in `internal/di`) still has capacity via **`CheckAndConsume()`** and **`len(content) > HotContentThreshold()`** (UTF-8 byte length vs `documents.tiering.hot_content_threshold`, default **5000**); else cold.

### 1.2 Hot path (implemented)

- Persist **`documents`** via `**DocRepo.Create**` with `**status = hot**`, request **tags** (deduplicated), **`summary`** from the request body, full **content**.
- Optional client **`chapters[]`**: map each `**ChapterInfo**` to `**types.Chapter**` (stable `path` under the document id), then `**IChapterRepository.ReplaceByDocument**`.
- Catalog tags: for each tag name, `**TagRepo.GetByName**` → `**TagRepo.Create**` when missing → `**IncrementDocumentCount**`.
- **Deferred analyze / materialize:** `**internal/di**` always wires `**IHotIngestProcessor**`, `**IHotIngestWorkSink**` (`**di.NewHotIngestQueueSink**` → `**job.HotIngestQueue**`, capacity 100), and `**cmd/main.go**` starts `**job.StartHotIngestQueueConsumer**`. Hot `**CreateDocument**` enqueues `**types.HotIngestWork**` when there are **no** client `**chapters[]**`. `**ProcessHotIngest**` is intentionally two-step: (1) `**internal/service/impl/document**.**IDocumentAnalysisGenerator**.**GenerateAnalysis**` — one LLM call, then JSON parse into summary/tags/chapters (length/chapter-count guidance is **prompt-only**; the server does **not** truncate or normalize LLM fields after parse; invalid chapter offsets leave empty `**content**` so gaps show in the document UI); (2) `**internal/service/impl/document**.**IDocumentAnalysisPersister**.**PersistAnalysis**` — write summary + replace chapter rows. **LLM/parse errors** materialize as a **single virtual failure chapter** (title prefixed with `[analysis failed]`) instead of silently inventing structure. **Without a configured LLM provider**, analysis errors the same way (no markdown-only substitute). **Cold→hot maintenance** (`**IDocumentMaintenanceService**`) remains disabled without an LLM. If the bounded queue is full, the sink drops work and logs a warning (no automatic retry; re-ingest or restart processing is manual).

### 1.3 Cold path (implemented)

- Persist **`documents`** with `**status = cold**` and **empty `tags`** (even if the client sent tags).
- Then `**IColdIndex.AddDocument(ctx, doc)**` (`**coldindex.Index**` from `cmd/main.go`): chapter split, Bleve + HNSW indexing, optional embedder — same behavior as cold index design (markdown windows / paths as in **`cold-index/cold-index.md`**). **Heading tree:** **goldmark** CommonMark AST; only **`ast.Heading`** nodes that are **direct children of the document** define chapters (prefer missed headings over false splits); see **`cold-index/cold-index.md` §2.1** and **`cold-index/chapter-tree-quality.md`**.
- **Order:** DB row first, then index; index failure surfaces as **HTTP 500** (row may already exist — operators can reindex or delete manually if needed).

### 1.4 Response

Returns `CreateDocumentResponse` (id, title, format, tags, summary preview fields, chapter count, status, timestamps).

---

## 2. `POST /api/v1/query/progressive` — Progressive query

**Handler:** `Handler.ProgressiveQuery` → `ExecuteProgressiveQuery` → `IQueryService.ProgressiveQuery` (`internal/service/impl/query/query_service_impl.go`).

### 2.1 Parallel paths

Two goroutines run concurrently:

| Path     | Purpose |
| -------- | ------- |
| **Hot**  | `**IChapterService.SearchHotChapters**` — legacy **progressive** pipeline: adaptive catalog **tags/topics** (LLM) → **documents** (LLM for hot/warming; keyword for cold in the candidate set) → **chapters** (LLM), returned as `**HotSearchHit**` rows (`content_source: hot_progressive`). |
| **Cold** | `**IChapterService.SearchColdChapterHits**` — hybrid cold index search over cold chapter chunks (`**IColdIndex.Search**`). |

Results are **merged** (`mergeHotAndColdQueryItems`): one `**QueryItem**` per **(document id, chapter path)**; if the same chapter appears on both paths, the higher **relevance** wins; sort by relevance; cap at `max_results`.

### 2.2 Hot path (`queryHotPath`)

1. `**SearchHotChapters(ctx, question, max_results/2)**` (`internal/service/impl/catalog/chapter_service_impl.go` + `search_hot_progressive.go` + `hot_progressive_llm_core.go`):  
   - **Tags**: `**TagRepo.List**`; if catalog tag count **< 200** → `**FilterTagsByQuery**` on all tags; else `**TopicRepo.List**` → `**FilterTopicsByQuery**` (up to **3**, relevance **≥ 0.5**) → `**TagRepo.ListByTopic**` per selected topic → `**FilterTagsByQuery**` on that subset.  
   - **Documents**: `**DocRepo.ListByTags**` (OR tags) or `**DocRepo.ListAll**` fallback when no tags; split **hot/warming** vs **cold** in the candidate set; hot/warming uses `**FilterDocuments**` (≥0.5); cold uses `**ExtractKeywords**` substring match on title/content/tags.  
   - **Chapters**: load persisted `**ChapterRepo.ListByDocument**` for hot/warming; cold candidates become one pseudo-chapter (`path` `docId/full`); `**FilterChapters**` (≥0.5). Each retained chapter maps to a `**HotSearchHit**` with `**Score**` from the chapter filter relevance.  
2. `**trackDocumentAccess**` (async per distinct document id from hot hits): `**DocRepo.IncrementQueryCount**`. Cold promotion via `**job.PromoteQueue**` applies only when the document status is **cold** (hot-path hits are hot/warming only).

### 2.3 Cold path (`queryColdPath`)

- If the cold index is unavailable → empty cold step (no error).  
- `**IChapterService.SearchColdChapterHits(ctx, question, max_results/2)**` → `**IColdIndex.Search**` — optional semantic branch when a text embedder was wired at startup (see §5).  
- Map each `**types.ColdSearchHit**` to a `**QueryItem**`; empty `path` falls back to `docId/full`. `status=cold`.

### 2.4 Answer field (`generateProgressiveAnswer`)

`**internal/service/impl/query/query_service_impl.go**` (answer synthesis): builds a prompt with up to 30 references, excerpts capped (~6KB UTF-8 each), instructs Markdown + `[^N^]` citations; `**ILLMProvider.Generate**` using configured `max_tokens`. On failure, `answer` is empty (UI may fall back).

---

## 3. `POST /api/v1/topics/regroup` — Topic regrouping (catalog tags → themes)

**Handler:** `ExecuteRegroupTagsIntoTopics` → `ITopicService.RegroupTags` (`internal/service/impl/catalog/topic_service_impl.go`).

1. `**TagRepo.List`** all catalog tags (empty list → no-op, refresh bookkeeping updated).
2. **Deterministic regroup (current implementation):** build one topic **"All tags"** containing every catalog tag name; `**TopicRepo.DeleteAll**` then `**TopicRepo.Create**` for that topic.
3. For each tag row: assign `**TopicID**` to the topic and `**TagRepo.Create**` (upsert path used by the implementation).
4. Updates in-memory refresh bookkeeping for `**ShouldRefresh**`.

`GET /api/v1/topics` lists persisted topics (`**ITopicService.ListTopics**` → `**TopicRepo.List**`).

Scheduled `**TopicRegroupJob**` (`internal/job/jobs.go`) runs the same regroup path on an interval when `**ShouldRefresh**` is true.

---

## 4. Hot retrieval family (`GET /api/v1/hot/...`)

**Handlers:** Gin entrypoints in `internal/api/handler_catalog.go` call `ExecuteListHotDocumentSummariesByTags` and `ExecuteListHotDocumentChaptersByDocumentIDs` in `internal/api/handler_execute.go`.  
**Data:** `**IDocumentService**` + `**IChapterService**` (DB reads only; no LLM in these endpoints).


| Endpoint                 | Algorithm                                                                                                                                                                                             |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `**/hot/doc_summaries`** | Require `tags`. `**IDocumentService.ListHotDocumentsWithSummariesByTags**` → `**DocRepo.ListMetaByTagsAndStatuses**` for `hot` + `warming`, cap `max_results`. Return `{ document_id, title, format, status, tags, summary }` from the document rows (body content not loaded). |
| `**/hot/doc_chapters`**  | Require `doc_ids` (trimmed to `max_results` doc cap). For each id: load persisted `chapters` rows, return path/title/summary/content.                                                                           |


---

## 5. `GET /api/v1/cold/chapter_hits` — Cold hybrid search

**Design (algorithms, indexing, merge, config):** [cold-index/cold-index.md](cold-index/cold-index.md) · [cold-index/cold-index.zh.md](cold-index/cold-index.zh.md)（中文）

**Handler:** `ExecuteSearchColdChapterHits` → `**IChapterService.SearchColdChapterHits**` → `**IColdIndex.Search**` (`internal/storage/coldindex/cold_index_impl.go`).

1. Parse `**q`** as comma-separated terms → single query string.
2. `**IChapterService.SearchColdChapterHits**` (`internal/service/impl/catalog/chapter_service_impl.go`) calls `**IColdIndex.Search**`, then maps each `**ColdIndexHit**` to `**types.ColdSearchHit**` for JSON. If `**coldindex.Index.SetTextEmbedder**` was wired at startup (`cmd/main.go` via `**NewTextEmbedderFromViper**`), the index may rank using additional signals internally; otherwise search is text-only.
3. Inside `**Search**`, the implementation may merge lexical and optional semantic indexes:
  - Each branch retrieves more candidates than the final **topK** (pool size from **cold_index.search**: `branch_recall_multiplier`, `branch_recall_floor`, `branch_recall_ceiling`), then merge so overlap can surface in the final cut.  
  - Text index branch — each hit is one **cold chapter**; `context` is the **full** chapter text (no keyword windowing).
  - Vector branch when embedding length matches `VectorDimension` — same chapter-level hits.
  - `**mergeHybridResults`**: dedupe by **`document_id` + `path`** (not by document alone); normalized score blend; combined rows get `source: hybrid`; sort by score; keep **`topK`**.
4. Handler maps `**types.ColdSearchHit**` rows to JSON (`document_id`, optional `path`, `title`, `score`, `context`, optional `source` for UI/debug trace only).

---

## 6. `GET /api/v1/tags` — Filtered tag list (lightweight core)

**Handler:** `ExecuteListTags`.

- `**ITagService.ListTags**`: if `**topic_ids**` non-empty, `**TagRepo.ListByTopicIDs**` with `max_results` (defaults/clamps per handler); else `**TagRepo.List**` with optional cap from `max_results`.

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