# TierSum Web Frontend

Next.js 14 frontend for TierSum - Hierarchical Summary Knowledge Base.

## Tech Stack

- **Framework**: Next.js 14 (App Router)
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **UI Components**: shadcn/ui
- **Icons**: lucide-react
- **Theme**: Slate dark theme (Notion/Linear style)

## Project Structure

```
web/
├── app/                    # Next.js App Router
│   ├── page.tsx           # Home - Progressive Query search
│   ├── layout.tsx         # Root layout with dark theme
│   ├── globals.css        # Global styles
│   ├── docs/
│   │   ├── page.tsx       # Document list page
│   │   └── [id]/
│   │       ├── page.tsx   # Document detail (SSR)
│   │       └── client.tsx # Document detail (client component)
│   └── tags/
│       └── page.tsx       # Two-level tag browser
├── components/
│   └── ui/                # shadcn/ui components
├── lib/
│   ├── api.ts            # API client for backend
│   └── utils.ts          # Utility functions
├── public/               # Static assets
├── next.config.js        # Next.js config (static export)
├── tailwind.config.ts    # Tailwind config
└── .env.local           # Environment variables
```

## Pages

### 1. Home (`/`)
- Progressive Query search interface
- Two-pane layout: results list + detail view
- Shows tier level, relevance score
- Quick navigation to full document

### 2. Documents (`/docs`)
- List all documents in knowledge base
- Search/filter by title or tags
- Shows hot score, query count, status
- Link to document detail

### 3. Document Detail (`/docs/[id]`)
- Full document content view
- Document statistics sidebar
- Summary tier selector (topic/document/chapter/paragraph)
- Tags display

### 4. Tags (`/tags`)
- Two-level tag navigation
- L1 Groups (categories) on left
- L2 Tags on right
- Click tag to search

## Development

```bash
# Install dependencies
npm install

# Run development server
npm run dev

# Build for production
npm run build

# The build output goes to `dist/` directory
```

## API Integration

The frontend communicates with the Go backend at `http://localhost:8080`:

- `POST /api/v1/query/progressive` - Progressive search
- `GET /api/v1/documents/:id` - Get document
- `GET /api/v1/documents/:id/summaries` - Get document summaries
- `GET /api/v1/tags` - List all tags
- `GET /api/v1/tags/groups` - List tag groups
- `POST /api/v1/tags/group` - Trigger tag grouping

## Configuration

Edit `.env.local`:

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

## Static Export

The project is configured for static export (`output: 'export'`), which generates static HTML files that can be served by the Go backend:

```javascript
// next.config.js
module.exports = {
  output: 'export',
  distDir: 'dist',
  trailingSlash: true,
}
```

The Go backend serves these files using:

```go
router.Static("/", "./web/dist")
```

## Design System

### Colors
- Background: `slate-950` (#020617)
- Card: `slate-900/50` with border `slate-800`
- Primary: `blue-500/600`
- Text Primary: `slate-100`
- Text Secondary: `slate-400`
- Text Muted: `slate-500`

### Components
- Cards with subtle borders and hover effects
- Badges for status/tags with outline variant
- Buttons with ghost variant for navigation
- Scroll areas for long content
- Skeleton loaders for loading states
