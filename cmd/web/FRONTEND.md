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
│   └── pages/            # Route views: Search, Documents, Tags, Monitoring, …
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
- **Documents** (`/docs`, `/docs/new`, `/docs/:id`): List/filter, full-page create, detail (summaries / chapters / source)
- **Tags** (`/tags`): L1 groups + L2 tag browsing, trigger regroup
- **Monitoring** (`/monitoring`): Health, Go runtime (`runtime` version / GOOS / GOARCH / CPU), document counts, quota, Prometheus preview
- **Dark theme**: Slate-style palette
- **Responsive**: Mobile-friendly layout

## Routes

- `/` — Search
- `/docs` — Document list
- `/docs/new` — Create document (Markdown + preview)
- `/docs/:id` — Document detail
- `/tags` — Tag browser
- `/monitoring` — Monitoring dashboard

Vue Router uses **HTML5 history** mode (`createWebHistory`): `/`, `/docs`, `/tags`, `/monitoring`, etc. The API server serves `index.html` for unknown non-API paths so direct URLs and refresh work.

---

## Web UI ↔ BFF REST

The embedded UI calls **`/bff/v1/*`** (same handlers as programmatic **`/api/v1/*`**, same origin). The BFF stack uses `api.BFFAuth()` (currently a no-op) so it can diverge from **`/api/v1`** key-based auth (`security.api_key`). Below: **route / feature** → **HTTP** (request shape and main JSON keys). Endpoints not listed are **not** used by the current UI (`js/`) today (e.g. hot/cold retrieval family).


| UI area                              | REST                                  | Notes                                                                                                                                                                                                                                                                                                     |
| ------------------------------------ | ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Search** — run query               | `POST /bff/v1/query/progressive`      | Body: `{ "question", "max_results" }`. Optional `?debug_trace=1` (UI “Trace sample”) force-samples the HTTP span for OpenTelemetry. Response adds `trace_id` when the request is part of a recording trace. Also `answer`, `steps`, `results`, …                                                                                                                    |
| **Documents** — list                 | `GET /bff/v1/documents`               | Response: `{ "documents": [ ... ] }` — each item is a document object (`id`, `title`, `content`, `format`, `tags`, `status`, `hot_score`, …).                                                                                                                                                             |
| **Documents** — open one             | `GET /bff/v1/documents/:id`           | Single document JSON (same fields as list item).                                                                                                                                                                                                                                                          |
| **Documents** — summaries tab / data | `GET /bff/v1/documents/:id/summaries` | Response: `{ "summaries": [ ... ] }` — `tier`, `path`, `content`, `is_source`, …                                                                                                                                                                                                                          |
| **Documents** — chapter nav / data   | `GET /bff/v1/documents/:id/chapters`  | Response: `{ "document_id", "chapters": [ { "path", "title", "summary" } ] }`.                                                                                                                                                                                                                            |
| **Documents** — create               | `POST /bff/v1/documents`              | Body: `CreateDocumentRequest` — `title`, `content`, `format` (`markdown` | `md`), optional `tags`, optional `ingest_mode` (`auto` \| `hot` \| `cold`; default auto), optional prebuilt `summary` / `chapters` / `embedding`. Response: created document summary payload (`id`, `title`, `format`, `tags`, `summary`, `chapter_count`, `status`, `created_at`). |
| **Tags** — L1 groups                 | `GET /bff/v1/tags/groups`             | Response: `{ "groups": [ ... ] }`.                                                                                                                                                                                                                                                                        |
| **Tags** — L2 list                   | `GET /bff/v1/tags`                    | Response: `{ "tags": [ ... ] }` — each tag includes `group_id`. UI loads all tags then filters by selected group; the API also supports `?group_ids=id1,id2&max_results=N` for server-side filtering.                                                                                                     |
| **Tags** — regroup                   | `POST /bff/v1/tags/group`             | Response: `{ "message": "..." }`.                                                                                                                                                                                                                                                                         |
| **Monitoring** — snapshot            | `GET /bff/v1/monitoring`              | JSON: `server.version`, `go` (`version`, `goos`, `goarch`, `compiler`, `num_cpu`, `gomaxprocs` from `runtime`), `documents` (counts by status), `cold_index.approx_chapters`, `cold_index.inverted` (`bleve_doc_count`, `storage_backend`, `text_analyzer`), `cold_index.vector` (`hnsw_nodes`, `vector_dim`, `hnsw_m`, `hnsw_ef_search`, `text_embedder_configured`), `telemetry`, `quota`, `prometheus_metrics_path` (always `/metrics`). Also `GET /health` for `status`, `version`, `cold_doc_count`. |
| **Monitoring** — Prometheus text   | `GET /metrics`                        | Plain-text exposition (Prometheus scrape path at server root; no API key); loaded in-page as preview.                                                                                                                                                                                                                                                                    |


**Not wired in the current UI** (REST exists under both `/bff/v1` and `/api/v1`; use curl/MCP against `/api/v1`, or extend `js/api_client.js` / pages):

- `GET /bff/v1/hot/doc_summaries`, `.../hot/doc_chapters`, `.../hot/doc_source`
- `GET /bff/v1/cold/doc_source`
- `GET /bff/v1/quota`, `GET /health`

**Auth:** Programmatic clients use **`/api/v1`** and, when `security.api_key` is set, send `X-API-Key` or `Authorization: Bearer <key>`. The embedded UI uses **`/bff/v1`** only; extend `api.BFFAuth()` in Go when you add browser-side auth.

**Errors:** Failed responses typically use `{ "error": "..." }`; the client surfaces `error` or `message` when present.