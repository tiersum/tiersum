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
│   ├── api_client.js     # REST helpers (`/api/v1/...`)
│   ├── markdown.js       # Marked wrappers (vendored `vendor/marked.esm.js`)
│   ├── vendor/           # Vue, Vue Router, @vue/devtools-api, Marked ESM (importmap; no CDN for core UI)
│   ├── components/       # Shared Vue SFC-style objects (e.g. AppHeader)
│   └── pages/            # Route views: Search, Documents, Tags, Monitoring, …
├── FRONTEND.md   # This file — stack, routes, UI ↔ REST mapping
└── api.js        # Standalone client snippet (not used by index.html)
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
- **Monitoring** (`/monitoring`): Health, document counts, quota, Prometheus preview
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

## Web UI ↔ REST API

The embedded UI calls TierSum REST under `**/api/v1`** (same origin as the server). Below: **route / feature** → **HTTP** (request shape and main JSON keys). Endpoints not listed are **not** used by the current UI (`js/`) today (e.g. hot/cold retrieval family).


| UI area                              | REST                                  | Notes                                                                                                                                                                                                                                                                                                     |
| ------------------------------------ | ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Search** — run query               | `POST /api/v1/query/progressive`      | Body: `{ "question": string, "max_results": 100 }`. Response: `question`, `answer`, `steps`, `results` (`results[].id`, `title`, `content`, `path`, `relevance`, `tier`, `status`, …).                                                                                                                    |
| **Documents** — list                 | `GET /api/v1/documents`               | Response: `{ "documents": [ ... ] }` — each item is a document object (`id`, `title`, `content`, `format`, `tags`, `status`, `hot_score`, …).                                                                                                                                                             |
| **Documents** — open one             | `GET /api/v1/documents/:id`           | Single document JSON (same fields as list item).                                                                                                                                                                                                                                                          |
| **Documents** — summaries tab / data | `GET /api/v1/documents/:id/summaries` | Response: `{ "summaries": [ ... ] }` — `tier`, `path`, `content`, `is_source`, …                                                                                                                                                                                                                          |
| **Documents** — chapter nav / data   | `GET /api/v1/documents/:id/chapters`  | Response: `{ "document_id", "chapters": [ { "path", "title", "summary" } ] }`.                                                                                                                                                                                                                            |
| **Documents** — create               | `POST /api/v1/documents`              | Body: `CreateDocumentRequest` — `title`, `content`, `format` (`markdown` | `md`), optional `tags`, optional `ingest_mode` (`auto` \| `hot` \| `cold`; default auto), optional prebuilt `summary` / `chapters` / `embedding`. Response: created document summary payload (`id`, `title`, `format`, `tags`, `summary`, `chapter_count`, `status`, `created_at`). |
| **Tags** — L1 groups                 | `GET /api/v1/tags/groups`             | Response: `{ "groups": [ ... ] }`.                                                                                                                                                                                                                                                                        |
| **Tags** — L2 list                   | `GET /api/v1/tags`                    | Response: `{ "tags": [ ... ] }` — each tag includes `group_id`. UI loads all tags then filters by selected group; the API also supports `?group_ids=id1,id2&max_results=N` for server-side filtering.                                                                                                     |
| **Tags** — regroup                   | `POST /api/v1/tags/group`             | Response: `{ "message": "..." }`.                                                                                                                                                                                                                                                                         |
| **Monitoring** — snapshot            | `GET /api/v1/monitoring`              | JSON: `server.version`, `documents` (counts by status), `cold_index.approx_chapters`, `quota`, `prometheus_metrics_path`. Also `GET /health` (outside `/api/v1`) for `status`, `version`, `cold_doc_count`.                                                                                                  |
| **Monitoring** — Prometheus text   | `GET /api/v1/metrics`                 | Plain-text exposition; loaded in-page as preview.                                                                                                                                                                                                                                                        |


**Not wired in the current UI** (REST exists; use curl, MCP, or extend `js/api_client.js` / pages):

- `GET /api/v1/hot/doc_summaries`, `.../hot/doc_chapters`, `.../hot/doc_source`
- `GET /api/v1/cold/doc_source`
- `GET /api/v1/quota`, `GET /api/v1/metrics`, `GET /health`

**Auth:** If `security.api_key` is set, the REST middleware expects `X-API-Key` or `Authorization: Bearer <key>`; the CDN client in `js/api_client.js` does not attach these by default — configure or extend `apiClient.request` if needed.

**Errors:** Failed responses typically use `{ "error": "..." }`; the client surfaces `error` or `message` when present.