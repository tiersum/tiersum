# Web UI Guide

TierSum includes a modern Vue 3 CDN-based frontend embedded in the Go binary.

**Which screen calls which REST endpoint** is documented in [cmd/web/FRONTEND.md](../cmd/web/FRONTEND.md).

**Sign-in, admin, and devices:** see [Auth and Permissions](../design/auth-and-permissions.md) and [Installation Guide](../getting-started/installation.md).

## Pages

### About (`/about`)

- Bilingual product overview (English, then Chinese): use cases, hot/cold in plain language, who TierSum is for
- No API calls; available without signing in once the system has been bootstrapped

### Query Page (`/`)

- Central search box with Progressive Query support
- Split-panel results: AI Answer (left) + Reference results (right)
- Displays both hot and cold document results (from `POST /api/v1/query/progressive`)
- Shows relevance scores and tier/status indicators

### Documents (`/docs`, `/docs/new`, `/docs/:id`)

- **List** (`/docs`): filter by title/tags; opens detail on row click
- **Create** (`/docs/new`): full-page Markdown editor + live preview
- **Detail** (`/docs/:id`): loads the document and chapter list via GETs; cold docs emphasize source view

### Topics & Tags (`/tags`)

- **Topics** (left): themes from `GET /api/v1/topics`
- **Catalog tags** (right): `GET /api/v1/tags?topic_ids=<selected id>&max_results=…` (each row is a deduplicated name with document count and optional `topic_id`)
- Regroup button triggers `POST /api/v1/topics/regroup`

### Observability (`/observability`)

Reachable from the top bar **Management → Observability** after sign-in.

- **Monitoring** tab (`?tab=monitoring`): health, runtime, document counts, cold index stats, Prometheus text preview (same data as `GET /bff/v1/monitoring` and `/metrics`).
- **Cold probe** tab (`?tab=cold`): calls `GET /bff/v1/cold/chapter_hits` with keywords and `max_results` to inspect hybrid cold hits (`path`, `score`, `source`, full chapter text) without running progressive query.
- **Traces** tab (`?tab=traces`): stored OpenTelemetry traces for progressive-query debugging.

## Tech Stack

- **Framework**: Vue 3 (via CDN)
- **Router**: Vue Router 4 (via CDN)
- **Styling**: Tailwind CSS (via CDN)
- **Components**: DaisyUI (via CDN)
- **Markdown**: Marked.js (via CDN)
- **Theme**: Slate dark theme
- **Deployment**: Embedded in Go binary via `//go:embed`
