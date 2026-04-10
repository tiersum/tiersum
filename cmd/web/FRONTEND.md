# TierSum CDN frontend

Pure CDN frontend; no Node.js required.

## Stack

- **Vue 3** — via CDN (unpkg.com)
- **Vue Router 4** — via CDN (unpkg.com)
- **Tailwind CSS** — via CDN (cdn.tailwindcss.com)
- **DaisyUI** — via CDN (cdn.jsdelivr.net)
- **Marked.js** — via CDN (cdn.jsdelivr.net) for Markdown rendering

## Layout

```
cmd/web/
├── index.html    # HTML shell; loads all CDN assets
├── app.js        # Vue app: components and routes
└── (api client merged into app.js)
```

## Deployment

Assets are embedded into the Go binary:

1. Files live under `cmd/web/`.
2. `//go:embed web/*` in `cmd/static.go` (same `main` package as `cmd/main.go`).
3. `StaticFileServer()` serves them at runtime.

## Build

```bash
# Build the Go binary only
make build

# Frontend is embedded automatically; no separate frontend build step
```

## Features

- **Search**: Progressive query, AI answer + references side-by-side
- **Documents**: List, search, create, delete
- **Tags**: L1 groups + L2 tag browsing
- **Dark theme**: Slate-style palette
- **Responsive**: Mobile-friendly layout

## Routes

- `/` — Search
- `/docs` — Documents
- `/tags` — Tags

Vue Router uses hash mode: `/#/`, `/#/docs`, `/#/tags`.
