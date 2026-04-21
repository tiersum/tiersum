/** Features page: detailed capability overview. Static, no backend calls. */

export const FeaturesPage = {
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10 pb-16">
                <p class="text-xs uppercase tracking-widest text-slate-500 mb-2">Features</p>
                <h1 class="text-3xl sm:text-4xl font-bold text-slate-100 mb-4">Everything you need for structured knowledge retrieval</h1>
                <p class="text-slate-400 text-lg mb-12 max-w-3xl">
                    TierSum combines hierarchical document processing, intelligent tiering, and hybrid search
                    into a single, self-hostable platform.
                </p>

                <div class="space-y-16">
                    <!-- Feature 1 -->
                    <section class="grid grid-cols-1 lg:grid-cols-2 gap-8 items-center">
                        <div class="order-2 lg:order-1">
                            <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-blue-500/10 border border-blue-500/20 text-blue-300 text-xs font-medium mb-4">
                                Core
                            </div>
                            <h2 class="text-2xl font-bold text-slate-100 mb-4">Chapter-First Document Processing</h2>
                            <p class="text-slate-400 leading-relaxed mb-4">
                                Traditional RAG systems split documents into arbitrary chunks, destroying structure and context.
                                TierSum parses Markdown by headings, creating a natural chapter hierarchy that mirrors how humans write and read.
                            </p>
                            <ul class="space-y-2 text-slate-400">
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-blue-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Heading-aware splitting preserves document structure
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-blue-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Configurable token budgets per chapter
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-blue-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Sliding stride for long sections
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-blue-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Stable path identifiers for every section
                                </li>
                            </ul>
                        </div>
                        <div class="order-1 lg:order-2 card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <pre class="text-sm text-slate-300 font-mono overflow-x-auto"><code># Architecture Decision Records

## 01-why-tier-sum

### Context
We needed a system that preserves...

### Decision
Use chapter-first splitting...

### Consequences
- Structure preserved
- Queries return whole sections</code></pre>
                            </div>
                        </div>
                    </section>

                    <hr class="border-slate-800" />

                    <!-- Feature 2 -->
                    <section class="grid grid-cols-1 lg:grid-cols-2 gap-8 items-center">
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <div class="space-y-3">
                                    <div class="flex items-center justify-between p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
                                        <div class="flex items-center gap-3">
                                            <div class="w-3 h-3 rounded-full bg-amber-400"></div>
                                            <span class="text-amber-200 font-medium">Hot Document</span>
                                        </div>
                                        <span class="text-xs text-amber-400/80">LLM analyzed</span>
                                    </div>
                                    <div class="flex items-center justify-between p-3 rounded-lg bg-sky-500/10 border border-sky-500/20">
                                        <div class="flex items-center gap-3">
                                            <div class="w-3 h-3 rounded-full bg-sky-400"></div>
                                            <span class="text-sky-200 font-medium">Cold Document</span>
                                        </div>
                                        <span class="text-xs text-sky-400/80">Indexed only</span>
                                    </div>
                                    <div class="flex items-center justify-between p-3 rounded-lg bg-slate-700/30 border border-slate-600/30">
                                        <div class="flex items-center gap-3">
                                            <div class="w-3 h-3 rounded-full bg-slate-400"></div>
                                            <span class="text-slate-200 font-medium">Warming Document</span>
                                        </div>
                                        <span class="text-xs text-slate-400/80">Promoting...</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div>
                            <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs font-medium mb-4">
                                Cost Optimization
                            </div>
                            <h2 class="text-2xl font-bold text-slate-100 mb-4">Hot / Cold Tiering</h2>
                            <p class="text-slate-400 leading-relaxed mb-4">
                                Not all documents need full LLM analysis. TierSum lets you choose the right ingest path
                                for each document, balancing query quality against cost.
                            </p>
                            <ul class="space-y-2 text-slate-400">
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Hot: Full LLM summaries + tags on ingest
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Cold: BM25 + vector hybrid search
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Auto mode picks based on content + quota
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-emerald-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Auto-promotion from cold to hot on frequent queries
                                </li>
                            </ul>
                        </div>
                    </section>

                    <hr class="border-slate-800" />

                    <!-- Feature 3 -->
                    <section class="grid grid-cols-1 lg:grid-cols-2 gap-8 items-center">
                        <div class="order-2 lg:order-1">
                            <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-violet-500/10 border border-violet-500/20 text-violet-300 text-xs font-medium mb-4">
                                Search
                            </div>
                            <h2 class="text-2xl font-bold text-slate-100 mb-4">Progressive Query</h2>
                            <p class="text-slate-400 leading-relaxed mb-4">
                                Instead of a single vector similarity search, TierSum uses a multi-stage pipeline
                                that mimics how humans search: narrow by topic, then document, then section.
                            </p>
                            <ul class="space-y-2 text-slate-400">
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-violet-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Stage 1: Filter by catalog tags / topics
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-violet-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Stage 2: Rank documents by LLM relevance
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-violet-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Stage 3: Score chapters, return top matches
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-violet-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Optional: Synthesize answer with citations
                                </li>
                            </ul>
                        </div>
                        <div class="order-1 lg:order-2 card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <pre class="text-sm text-slate-300 font-mono overflow-x-auto"><code>POST /api/v1/query/progressive
{
  "question": "How does hot/cold tiering work?",
  "max_results": 10
}

// Response
{
  "answer": "Hot/cold tiering balances...",
  "steps": [
    { "stage": "tags", "matches": 3 },
    { "stage": "documents", "matches": 12 },
    { "stage": "chapters", "matches": 8 }
  ],
  "results": [...]
}</code></pre>
                            </div>
                        </div>
                    </section>

                    <hr class="border-slate-800" />

                    <!-- Feature 4 -->
                    <section class="grid grid-cols-1 lg:grid-cols-2 gap-8 items-center">
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <div class="space-y-4">
                                    <div class="p-4 rounded-lg bg-slate-800/50">
                                        <div class="flex items-center gap-2 mb-2">
                                            <div class="w-2 h-2 rounded-full bg-blue-400"></div>
                                            <span class="text-sm font-medium text-slate-200">API Keys</span>
                                        </div>
                                        <p class="text-xs text-slate-500">Programmatic access with scoped tokens (read / write / admin)</p>
                                    </div>
                                    <div class="p-4 rounded-lg bg-slate-800/50">
                                        <div class="flex items-center gap-2 mb-2">
                                            <div class="w-2 h-2 rounded-full bg-emerald-400"></div>
                                            <span class="text-sm font-medium text-slate-200">Browser Sessions</span>
                                        </div>
                                        <p class="text-xs text-slate-500">HttpOnly cookies for web UI with role-based access</p>
                                    </div>
                                    <div class="p-4 rounded-lg bg-slate-800/50">
                                        <div class="flex items-center gap-2 mb-2">
                                            <div class="w-2 h-2 rounded-full bg-amber-400"></div>
                                            <span class="text-sm font-medium text-slate-200">Passkeys</span>
                                        </div>
                                        <p class="text-xs text-slate-500">WebAuthn support for passwordless authentication</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div>
                            <div class="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs font-medium mb-4">
                                Security
                            </div>
                            <h2 class="text-2xl font-bold text-slate-100 mb-4">Dual-Track Authentication</h2>
                            <p class="text-slate-400 leading-relaxed mb-4">
                                Separate authentication paths for humans and programs. Humans use browser sessions
                                with passkey support. Programs use scoped API keys.
                            </p>
                            <ul class="space-y-2 text-slate-400">
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-rose-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Role-based access: viewer, user, admin
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-rose-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    API key scopes: read, write, admin
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-rose-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Device tokens for persistent sessions
                                </li>
                                <li class="flex items-start gap-2">
                                    <svg class="w-5 h-5 text-rose-400 shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>
                                    </svg>
                                    Rate limiting and brute-force protection
                                </li>
                            </ul>
                        </div>
                    </section>
                </div>
            </main>
        </div>
    `
};