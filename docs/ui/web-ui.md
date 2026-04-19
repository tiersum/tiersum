# Web UI Guide

TierSum includes a modern Vue 3 CDN-based frontend embedded in the Go binary.

**Which screen calls which REST endpoint** is documented in [cmd/web/FRONTEND.md](../cmd/web/FRONTEND.md).

**Sign-in, admin, and devices:** see [Auth and Permissions](../design/auth-and-permissions.md) and [Installation Guide](../getting-started/installation.md).

## Pages

### Library (`/library`)

The unified document library and catalog browser:

- **Document list**: all documents with bucket filters (**All**, **Cold**, **Untagged**)
- **Topics & Tags** (sidebar): themes from `GET /api/v1/topics`, catalog tags from `GET /api/v1/tags?topic_ids=…`
- **Filtering**: client-side filter by topic/tag (see `js/pages/LibraryPage.js`)
- **Actions**: **Add document** button, **Regroup into topics** button (non-viewer roles only)

> **Note:** `/docs` and `/tags` redirect to `/library`.

### Query Page (`/`)

- Central search box with Progressive Query support
- Split-panel results: AI Answer (left) + Reference results (right)
- Displays both hot and cold document results (from `POST /api/v1/query/progressive`)
- Shows relevance scores and tier/status indicators

### Documents (`/docs/new`, `/docs/:id`)

- **Create** (`/docs/new`): full-page Markdown editor + live preview
- **Detail** (`/docs/:id`): loads the document and chapter list via GETs; cold docs emphasize source view

### About (`/about`)

- Bilingual product overview (English, then Chinese): use cases, hot/cold in plain language, who TierSum is for
- No API calls; available without signing in once the system has been bootstrapped
- Public access after system initialization (router guard in `main.js`)

### Observability (`/observability`)

Reachable from the top bar **Management → Observability** after sign-in (admin role only).

- **Monitoring** tab (`?tab=monitoring`): health, runtime, document counts, cold index stats, Prometheus text preview (same data as `GET /bff/v1/monitoring` and `/metrics`).
- **Cold probe** tab (`?tab=cold`): calls `GET /bff/v1/cold/chapter_hits` with keywords and `max_results` to inspect hybrid cold hits (`path`, `score`, `source`, full chapter text) without running progressive query.
- **Traces** tab (`?tab=traces`): stored OpenTelemetry traces for progressive-query debugging.

### Management Pages

After login, the top bar shows a **Management** dropdown:

- **Devices & sessions** (`/settings`) — all signed-in roles; manage devices, aliases, sign-out
- **Users & API keys** (`/admin`) — admin role only; user management, API key CRUD
- **Configuration** (`/admin/config`) — admin role only; read-only redacted effective config snapshot

## Permissions

| Role | Access |
|------|--------|
| **viewer** | Read-only: search, library, documents. Cannot ingest or regroup topics. |
| **user** | Full product features except observability admin pages. |
| **admin** | Full access including observability, user management, and configuration. |

## Tech Stack

- **Framework**: Vue 3 (vendored ESM in `js/vendor/`, importmap → `vue`)
- **Router**: Vue Router 4 (vendored ESM in `js/vendor/`, importmap → `vue-router`)
- **Styling**: Tailwind CSS (via CDN)
- **Components**: DaisyUI (via CDN)
- **Markdown**: Marked.js (vendored ESM)
- **Theme**: Slate dark theme
- **Build**: No bundler — native ES modules under `js/`, entry `js/main.js`
- **Deployment**: Embedded in Go binary via `//go:embed`

## Route Mapping

| Route | Description | Redirect |
|-------|-------------|----------|
| `/` | Search page | — |
| `/library` | Document library + topics/tags | — |
| `/docs` | — | → `/library` |
| `/tags` | — | → `/library` |
| `/docs/new` | Create document | — |
| `/docs/:id` | Document detail | — |
| `/about` | Product introduction | — |
| `/observability` | Monitoring & traces | `/monitoring` → here |
| `/settings` | Devices & sessions | — |
| `/admin` | Users & API keys | — |
| `/admin/config` | Configuration snapshot | — |

Vue Router uses **HTML5 history** mode (`createWebHistory`). The API server serves `index.html` for unknown non-API paths so direct URLs and refresh work.
