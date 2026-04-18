# TierSum CDN frontend

Pure CDN frontend; no Node.js required.

## Stack

- **Vue 3** — vendored ESM in `js/vendor/vue.esm-browser.prod.js` (importmap → `vue`)
- **Vue Router 4** — vendored ESM in `js/vendor/vue-router.esm-browser.js` (importmap → `vue-router`)
- **Marked** — vendored ESM in `js/vendor/marked.esm.js` (import from `markdown.js`)
- **Tailwind CSS** — via CDN (cdn.tailwindcss.com)
- **DaisyUI** — via CDN (cdn.jsdelivr.net)
- **Marked.js** — ESM (`+esm`) in `js/markdown.js` (jsDelivr)
- **No bundler** — native ES modules under `js/`; entry `js/main.js`

## Layout

```
cmd/web/
├── index.html    # Shell: importmap + Tailwind/DaisyUI + `type="module"` → `js/main.js`
├── js/
│   ├── main.js           # createApp, router, root shell
│   ├── api_client.js     # BFF REST helpers (`/bff/v1/...`)
│   ├── markdown.js       # Marked wrappers (vendored `vendor/marked.esm.js`)
│   ├── vendor/           # Vue, Vue Router, @vue/devtools-api, Marked ESM (importmap; no CDN for core UI)
│   ├── components/       # Shared Vue SFC-style objects (e.g. AppHeader)
│   └── pages/            # Route views: Search, Library (/library), Monitoring, …
└── FRONTEND.md   # This file — stack, routes, UI ↔ REST mapping
```

## Deployment

Assets are embedded into the Go binary:

1. Files live under `cmd/web/`.
2. `//go:embed web/*` in `cmd/main.go` (same `main` package).
3. `StaticFileServer()` serves them at runtime.

## Build

```bash
# Build the Go binary only
make build

# Frontend is embedded automatically; no separate frontend build step
```

## Features

- **Search** (`/`): Progressive query, server `answer` when available + reference list
- **Library** (`/library`; `/docs` and `/tags` redirect here): document list with buckets (**All**, **Cold**, **Untagged**) and **Topics** → catalog **Tags** → filter documents (client-side); **Regroup into topics** and **Add document** (non-viewer); create/detail remain **`/docs/new`** and **`/docs/:id`**
- **About** (`/about`): bilingual (English then Chinese) end-user product overview — static copy only, no BFF calls; reachable **without a browser session** once the system is initialized (`main.js` router guard).
- **Management** (top bar dropdown after login, `js/components/AppHeader.js`): **Observability** (`/observability`, `/monitoring` redirects here) — **admin** human role only (BFF: `GET /bff/v1/monitoring`, `GET /bff/v1/traces*`). **Devices & sessions** (`/settings`) — all signed-in roles. **Users & API keys** (`/admin`) — **admin** only; **Configuration** (`/admin/config`) — **admin** only, redacted `GET /bff/v1/admin/config/snapshot`. Human **`viewer`** is read-only in the UI (no ingest / regroup; `BFFHumanRBAC` allows `GET` + `POST /bff/v1/query/progressive` only). Observability: **Monitoring** tab (health, runtime, cold index stats, Prometheus preview), **Cold probe** (`GET /bff/v1/cold/chapter_hits`, `?tab=cold`), **Traces** (`?tab=traces`).
- **Dark theme**: Slate-style palette
- **Responsive**: Mobile-friendly layout
- **Entry points**: the home hero links **Add a document** / **Browse library**; management pages are reachable via the **Management** dropdown after sign-in.

## Routes

- `/` — Search
- `/library` — Library (documents + topics/catalog tags + regroup)
- `/docs` — Redirects to `/library`
- `/tags` — Redirects to `/library`
- `/docs/new` — Create document (Markdown + preview)
- `/docs/:id` — Document detail
- `/about` — Product introduction (bilingual); public after bootstrap (no login required)
- `/observability` — Monitoring + cold probe + traces (`/monitoring` redirects here); linked from **Management → Observability** (not a top-level nav button).

Vue Router uses **HTML5 history** mode (`createWebHistory`): `/`, `/library`, `/docs`, `/docs/new`, `/docs/:id`, `/about`, `/observability`, etc. The API server serves `index.html` for unknown non-API paths so direct URLs and refresh work.

**Permission / management UI entry:** After login, the top bar shows a **Management** dropdown (`js/components/AppHeader.js`): **Observability** (`/observability`, **admin** only), **Devices & sessions** (`/settings`, every role), **Users & API keys** (`/admin`, **admin** only), **Configuration** (`/admin/config`, **admin** only). Direct URLs: `/observability`, `/settings`, `/admin`, `/admin/config` (non-admins hitting `/observability` are redirected by `main.js` guards; BFF still enforces `ADMIN_ROLE_REQUIRED`).

---

## Web UI ↔ BFF REST

The embedded UI calls **`/bff/v1/*`** (same handlers as programmatic **`/api/v1/*`**, same origin). **Human track:** `fetch` uses **`credentials: 'include'`**; `api.BFFSessionMiddleware` requires an HttpOnly **`tiersum_session`** cookie issued by **`POST /bff/v1/auth/login`** (or **`POST /bff/v1/auth/device_login`**) after bootstrap. Optional HttpOnly **`tiersum_device`** stores a persistent device token for quick re-login. **Program track:** integrations use **`/api/v1`** with DB-backed API keys (`X-API-Key` or `Authorization: Bearer`). Full permission model: **[../../docs/design/auth-and-permissions.md](../../docs/design/auth-and-permissions.md)**; end-user steps: see [Auth and Permissions](../../docs/design/auth-and-permissions.md) design doc.

Below: **route / feature** → **HTTP** (request shape and main JSON keys). Endpoints not listed are **not** used by the current UI (`js/`) today (e.g. hot/cold retrieval family).

| Auth / setup | REST | Notes |
| --- | --- | --- |
| Bootstrap wizard | `GET /bff/v1/system/status`, `POST /bff/v1/system/bootstrap` | First boot only; returns admin access token + initial read API key once. |
| Login | `POST /bff/v1/auth/login` | Body: `{ "access_token", "fingerprint": { "timezone", "client_signal?" }, "remember_me?": bool, "device_name?" }`. Sets `tiersum_session`. When `remember_me=true`, also sets `tiersum_device`. |
| Device login | `POST /bff/v1/auth/device_login` | Body: `{ "device_token?", "fingerprint" }` (or rely on `tiersum_device` cookie). Sets `tiersum_session`. |
| Logout | `POST /bff/v1/auth/logout` | Clears session cookie. |
| Profile | `GET /bff/v1/me/profile` | `{ user_id, username, role }` for header / guards. |
| Settings — security | `GET /bff/v1/me/profile` then `GET /bff/v1/me/devices` **or** (admin role) `GET /bff/v1/admin/devices`; `PATCH/DELETE /bff/v1/me/devices…`; `POST /bff/v1/me/sessions/revoke_all`; passkeys + device tokens under `/bff/v1/me/security/*` | Non-admins: own sessions only. Admins: all users’ sessions on the same screen; per-device sign-out still uses `/me/devices/:id`. Admins additionally need a fresh passkey verification for `/bff/v1/admin/*` when passkeys exist (`auth.passkey.admin_required`). |
| Admin | `/bff/v1/admin/*` | Admin role only: users, **`GET /admin/devices`** (all browser sessions), API keys, usage, **`GET /admin/config/snapshot`** (redacted effective config; UI **Management → Configuration** at `/admin/config`). |


| UI area                              | REST                                  | Notes                                                                                                                                                                                                                                                                                                     |
| ------------------------------------ | ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Search** — run query               | `POST /bff/v1/query/progressive`      | Body: `{ "question", "max_results" }`. Optional `?debug_trace=1` (UI “Trace sample”) force-samples the HTTP span for OpenTelemetry. Response adds `trace_id` when the request is part of a recording trace. Also `answer`, `steps`, `results`, …                                                                                                                    |
| **Library** — document list          | `GET /bff/v1/documents`               | Response: `{ "documents": [ ... ] }` — each item is a document object (`id`, `title`, `content`, `format`, `tags`, `status`, `hot_score`, …). Filtering by bucket/topic/tag is **client-side** in `js/pages/LibraryPage.js`.                                                                                                                                              |
| **Documents** — open one             | `GET /bff/v1/documents/:id`           | Single document JSON (same fields as list item).                                                                                                                                                                                                                                                          |
| **Documents** — chapter nav / data   | `GET /bff/v1/documents/:id/chapters`  | Response: `{ "document_id", "chapters": [ { "path", "title", "summary" } ] }`.                                                                                                                                                                                                                            |
| **Documents** — create               | `POST /bff/v1/documents`              | Body: `CreateDocumentRequest` — `title`, `content`, `format` (`markdown` | `md`), optional `tags`, optional `ingest_mode` (`auto` \| `hot` \| `cold`; default auto), optional prebuilt `summary` / `chapters` / `embedding`. Response: created document summary payload (`id`, `title`, `format`, `tags`, `summary`, `chapter_count`, `status`, `created_at`). |
| **Library** — topic list             | `GET /bff/v1/topics`                  | Response: `{ "topics": [ ... ] }` — each topic has `id`, `name`, `description`, …                                                                                                                                                                                                                         |
| **Library** — catalog tags in topic | `GET /bff/v1/tags?topic_ids=…`       | Response: `{ "tags": [ ... ] }` — each tag includes `topic_id` when assigned. UI passes the selected topic’s `id` as `topic_ids` (comma-separated; supports multiple) and `max_results`.                                                                                                                |
| **Library** — topic regroup          | `POST /bff/v1/topics/regroup`         | Response: `{ "message": "..." }`.                                                                                                                                                                                                                                                                         |
| **Observability / Monitoring** — snapshot | `GET /bff/v1/monitoring`              | JSON: `server.version`, `go` (`version`, `goos`, `goarch`, `compiler`, `num_cpu`, `gomaxprocs` from `runtime`), `documents` (counts by status), `cold_index.approx_chapters`, `cold_index.inverted` (`bleve_doc_count`, `storage_backend`, `text_analyzer`), `cold_index.vector` (`hnsw_nodes`, `vector_dim`, `hnsw_m`, `hnsw_ef_search`, `text_embedder_configured`), `telemetry`, `quota`, `prometheus_metrics_path` (always `/metrics`). Also `GET /health` for `status`, `version`, `cold_doc_count`. |
| **Observability — Cold probe** tab | `GET /bff/v1/cold/chapter_hits?q=…&max_results=` | Own tab (`?tab=cold`) in `js/pages/ObservabilityPage.js`, sibling to Monitoring and Traces. Query `q`: comma- or space-separated keywords. Response `{ "items": [ { "document_id", "title", "path?", "score", "context", "source?" } ] }` — `source` when present indicates branch (e.g. bm25 / vector / hybrid). **503** if cold index unavailable. |
| **Observability / Monitoring** — Prometheus text | `GET /metrics`                        | Plain-text exposition (Prometheus scrape path at server root; no API key); loaded in-page as preview.                                                                                                                                                                                                                                                                    |


**Not wired in the current UI** (REST exists under both `/bff/v1` and `/api/v1`; use curl/MCP against `/api/v1`, or extend `js/api_client.js` / pages):

- `GET /bff/v1/hot/doc_summaries`, `.../hot/doc_chapters`
- `GET /bff/v1/quota`, `GET /health`

**Auth:** **`/api/v1`** requires a valid DB API key on every request (scopes `read` \| `write` \| `admin`) unless the system is not yet initialized (**403** `{ "code": "SYSTEM_NOT_INITIALIZED" }`). **`/bff/v1`** document routes require a browser session; public paths are **`/bff/v1/system/status`**, **`/bff/v1/system/bootstrap`**, **`/bff/v1/auth/login`**, **`/bff/v1/auth/device_login`**, **`/bff/v1/auth/logout`**. **`GET /health`** and **`GET /metrics`** at the server root stay public. MCP tools use **`TIERSUM_API_KEY`** (or `mcp.api_key`) with the same scope rules as REST.

**Errors:** Failed responses typically use `{ "error": "..." }`; the client surfaces `error` or `message` when present.