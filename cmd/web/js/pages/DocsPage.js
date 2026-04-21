/** Documentation page: user guide and API reference. Static, no backend calls. */

export const DocsPage = {
    data() {
        return {
            activeSection: 'guide'
        };
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 pb-16">
                <div class="flex flex-col lg:flex-row gap-8">
                    <!-- Sidebar -->
                    <aside class="lg:w-64 shrink-0">
                        <nav class="sticky top-24 space-y-6">
                            <div>
                                <h3 class="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-3">User Guide</h3>
                                <ul class="space-y-1">
                                    <li>
                                        <button
                                            @click="activeSection = 'guide'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'guide' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            Quick Start
                                        </button>
                                    </li>
                                    <li>
                                        <button
                                            @click="activeSection = 'ingest'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'ingest' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            Ingesting Documents
                                        </button>
                                    </li>
                                    <li>
                                        <button
                                            @click="activeSection = 'query'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'query' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            Querying
                                        </button>
                                    </li>
                                    <li>
                                        <button
                                            @click="activeSection = 'tiering'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'tiering' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            Hot / Cold Tiering
                                        </button>
                                    </li>
                                </ul>
                            </div>
                            <div>
                                <h3 class="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-3">API Reference</h3>
                                <ul class="space-y-1">
                                    <li>
                                        <button
                                            @click="activeSection = 'api-auth'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'api-auth' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            Authentication
                                        </button>
                                    </li>
                                    <li>
                                        <button
                                            @click="activeSection = 'api-docs'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'api-docs' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            Documents
                                        </button>
                                    </li>
                                    <li>
                                        <button
                                            @click="activeSection = 'api-query'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'api-query' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            Query
                                        </button>
                                    </li>
                                    <li>
                                        <button
                                            @click="activeSection = 'api-mcp'"
                                            :class="['w-full text-left px-3 py-2 rounded-lg text-sm transition-colors', activeSection === 'api-mcp' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            MCP Protocol
                                        </button>
                                    </li>
                                </ul>
                            </div>
                        </nav>
                    </aside>

                    <!-- Content -->
                    <main class="flex-1 min-w-0">
                        <!-- Quick Start -->
                        <div v-if="activeSection === 'guide'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">Quick Start</h1>
                                <p class="text-slate-400 mb-6">
                                    Get TierSum running locally in a few minutes.
                                </p>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">1. Prerequisites</h3>
                                    <ul class="list-disc pl-5 space-y-1 text-slate-400">
                                        <li>Go 1.23 or later</li>
                                        <li>Make</li>
                                        <li>OpenAI API key (or Anthropic, or local Ollama)</li>
                                    </ul>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">2. Installation</h3>
                                    <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono"><code># Clone the repository
git clone https://github.com/tiersum/tiersum.git
cd tiersum

# Copy and edit configuration
cp configs/config.example.yaml configs/config.yaml
# Edit configs/config.yaml and set your LLM API key

# Build
make build

# Run
make run</code></pre>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">3. Bootstrap</h3>
                                    <p class="text-slate-400 mb-4">
                                        Open <code class="text-slate-300">http://localhost:8080</code> in your browser.
                                        Complete the bootstrap wizard to create the first admin user.
                                    </p>
                                    <div class="alert alert-info bg-blue-950/30 border-blue-900/50 text-blue-200">
                                        <svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                        </svg>
                                        <span>The bootstrap endpoint is only accessible from localhost by default for security.</span>
                                    </div>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">4. First Document</h3>
                                    <p class="text-slate-400 mb-4">Navigate to the Library page and click "Add Document". Paste Markdown content and choose an ingest mode:</p>
                                    <ul class="list-disc pl-5 space-y-1 text-slate-400">
                                        <li><strong class="text-slate-200">Auto</strong> — Let TierSum decide based on content length and quota</li>
                                        <li><strong class="text-slate-200">Hot</strong> — Force full LLM analysis (better queries, uses quota)</li>
                                        <li><strong class="text-slate-200">Cold</strong> — Index only (faster ingest, BM25 + vector search)</li>
                                    </ul>
                                </div>
                            </div>
                        </div>

                        <!-- Ingesting Documents -->
                        <div v-if="activeSection === 'ingest'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">Ingesting Documents</h1>
                                <p class="text-slate-400 mb-6">
                                    TierSum ingests Markdown documents and processes them according to the chosen mode.
                                </p>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Supported Formats</h3>
                                    <p class="text-slate-400">Currently, TierSum supports Markdown (<code class="text-slate-300">.md</code>, <code class="text-slate-300">.markdown</code>) documents. The parser recognizes ATX headings (<code class="text-slate-300">#</code>, <code class="text-slate-300">##</code>, etc.) to split chapters.</p>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Ingest Modes</h3>
                                    <div class="space-y-4">
                                        <div class="p-4 rounded-lg bg-amber-500/10 border border-amber-500/20">
                                            <div class="flex items-center gap-2 mb-2">
                                                <span class="badge badge-warning badge-sm">Hot</span>
                                                <span class="text-amber-200 font-semibold">Full LLM Analysis</span>
                                            </div>
                                            <p class="text-slate-400 text-sm">Generates document summary, chapter summaries, and catalog tags. Best for frequently queried documents. Counts against hourly quota.</p>
                                        </div>
                                        <div class="p-4 rounded-lg bg-sky-500/10 border border-sky-500/20">
                                            <div class="flex items-center gap-2 mb-2">
                                                <span class="badge badge-info badge-sm">Cold</span>
                                                <span class="text-sky-200 font-semibold">Index Only</span>
                                            </div>
                                            <p class="text-slate-400 text-sm">Splits into chapters and indexes with BM25 + vector search. No LLM calls on ingest. Best for large archives and cost-sensitive deployments.</p>
                                        </div>
                                        <div class="p-4 rounded-lg bg-slate-700/30 border border-slate-600/30">
                                            <div class="flex items-center gap-2 mb-2">
                                                <span class="badge badge-ghost badge-sm">Auto</span>
                                                <span class="text-slate-200 font-semibold">Smart Selection</span>
                                            </div>
                                            <p class="text-slate-400 text-sm">Chooses hot if content length > 5000 chars and quota allows; otherwise cold. Recommended for most use cases.</p>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">API Example</h3>
                                    <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono"><code>POST /api/v1/documents
Content-Type: application/json
X-API-Key: tsk_live_xxx

{
  "title": "Architecture Decision Records",
  "content": "# Why TierSum...",
  "format": "markdown",
  "tags": ["architecture", "adr"],
  "ingest_mode": "auto"
}</code></pre>
                                </div>
                            </div>
                        </div>

                        <!-- Querying -->
                        <div v-if="activeSection === 'query'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">Querying</h1>
                                <p class="text-slate-400 mb-6">
                                    TierSum offers progressive query for intelligent retrieval and direct cold search for raw chapter hits.
                                </p>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Progressive Query</h3>
                                    <p class="text-slate-400 mb-4">The recommended query method. Walks through three stages:</p>
                                    <ol class="list-decimal pl-5 space-y-2 text-slate-400">
                                        <li><strong class="text-slate-200">Tag Filter</strong> — Find relevant catalog tags from the query</li>
                                        <li><strong class="text-slate-200">Document Rank</strong> — Score matching documents with LLM relevance</li>
                                        <li><strong class="text-slate-200">Chapter Select</strong> — Pick top chapters from ranked documents</li>
                                    </ol>
                                    <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono mt-4"><code>POST /api/v1/query/progressive
{
  "question": "How does authentication work?",
  "max_results": 10
}

// Returns: answer, steps, references</code></pre>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Cold Search</h3>
                                    <p class="text-slate-400 mb-4">Direct BM25 + vector hybrid search over cold chapter index. Returns raw chapter text without LLM synthesis.</p>
                                    <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono"><code>GET /api/v1/cold/chapter_hits?q=auth,login&max_results=20</code></pre>
                                </div>
                            </div>
                        </div>

                        <!-- Hot / Cold Tiering -->
                        <div v-if="activeSection === 'tiering'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">Hot / Cold Tiering</h1>
                                <p class="text-slate-400 mb-6">
                                    TierSum's core cost optimization mechanism. Documents can be hot (fully analyzed) or cold (indexed only).
                                </p>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Promotion</h3>
                                    <p class="text-slate-400">
                                        Cold documents with <code class="text-slate-300">query_count >= 3</code> are automatically queued for promotion.
                                        A background job runs every 5 minutes to promote queued documents to hot,
                                        running full LLM analysis.
                                    </p>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Quota Management</h3>
                                    <p class="text-slate-400">
                                        Hot ingest is rate-limited to control LLM costs. Default: 100 documents per hour.
                                        Check current quota at <code class="text-slate-300">GET /api/v1/quota</code>.
                                    </p>
                                </div>
                            </div>
                        </div>

                        <!-- API Auth -->
                        <div v-if="activeSection === 'api-auth'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">Authentication</h1>
                                <p class="text-slate-400 mb-6">
                                    TierSum uses dual-track authentication: API keys for programs, browser sessions for humans.
                                </p>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">API Keys</h3>
                                    <p class="text-slate-400 mb-4">Include in every request via header:</p>
                                    <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono"><code>X-API-Key: tsk_live_xxx
# or
Authorization: Bearer tsk_live_xxx</code></pre>
                                    <p class="text-slate-400 mt-4">Scopes: <code class="text-slate-300">read</code> (GET + query), <code class="text-slate-300">write</code> (+ ingest), <code class="text-slate-300">admin</code> (full access).</p>
                                </div>
                            </div>
                        </div>

                        <!-- API Docs -->
                        <div v-if="activeSection === 'api-docs'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">Documents API</h1>
                            </div>

                            <div class="space-y-4">
                                <div class="card bg-slate-900/50 border border-slate-800">
                                    <div class="card-body">
                                        <div class="flex items-center gap-3 mb-2">
                                            <span class="badge badge-success">POST</span>
                                            <span class="font-mono text-slate-200">/api/v1/documents</span>
                                        </div>
                                        <p class="text-slate-400 text-sm">Create a new document. Requires <code class="text-slate-300">write</code> scope.</p>
                                    </div>
                                </div>

                                <div class="card bg-slate-900/50 border border-slate-800">
                                    <div class="card-body">
                                        <div class="flex items-center gap-3 mb-2">
                                            <span class="badge badge-primary">GET</span>
                                            <span class="font-mono text-slate-200">/api/v1/documents</span>
                                        </div>
                                        <p class="text-slate-400 text-sm">List all documents. Supports <code class="text-slate-300">max_results</code> query param.</p>
                                    </div>
                                </div>

                                <div class="card bg-slate-900/50 border border-slate-800">
                                    <div class="card-body">
                                        <div class="flex items-center gap-3 mb-2">
                                            <span class="badge badge-primary">GET</span>
                                            <span class="font-mono text-slate-200">/api/v1/documents/:id</span>
                                        </div>
                                        <p class="text-slate-400 text-sm">Get a single document by ID.</p>
                                    </div>
                                </div>

                                <div class="card bg-slate-900/50 border border-slate-800">
                                    <div class="card-body">
                                        <div class="flex items-center gap-3 mb-2">
                                            <span class="badge badge-primary">GET</span>
                                            <span class="font-mono text-slate-200">/api/v1/documents/:id/chapters</span>
                                        </div>
                                        <p class="text-slate-400 text-sm">List chapter summaries for a document.</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- API Query -->
                        <div v-if="activeSection === 'api-query'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">Query API</h1>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <div class="flex items-center gap-3 mb-2">
                                        <span class="badge badge-success">POST</span>
                                        <span class="font-mono text-slate-200">/api/v1/query/progressive</span>
                                    </div>
                                    <p class="text-slate-400 text-sm mb-4">Run a progressive query. Requires <code class="text-slate-300">read</code> scope.</p>
                                    <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono"><code>{
  "question": "string",
  "max_results": 100
}</code></pre>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <div class="flex items-center gap-3 mb-2">
                                        <span class="badge badge-primary">GET</span>
                                        <span class="font-mono text-slate-200">/api/v1/cold/chapter_hits</span>
                                    </div>
                                    <p class="text-slate-400 text-sm">Cold hybrid search. Query params: <code class="text-slate-300">q</code> (keywords), <code class="text-slate-300">max_results</code>.</p>
                                </div>
                            </div>
                        </div>

                        <!-- API MCP -->
                        <div v-if="activeSection === 'api-mcp'" class="space-y-8">
                            <div>
                                <h1 class="text-3xl font-bold text-slate-100 mb-4">MCP Protocol</h1>
                                <p class="text-slate-400 mb-6">
                                    TierSum implements the Model Context Protocol for AI agent integration.
                                </p>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Endpoints</h3>
                                    <div class="space-y-3">
                                        <div class="flex items-center gap-3">
                                            <span class="badge badge-primary">GET</span>
                                            <span class="font-mono text-slate-200 text-sm">/mcp/sse</span>
                                            <span class="text-slate-500 text-sm">— SSE stream</span>
                                        </div>
                                        <div class="flex items-center gap-3">
                                            <span class="badge badge-success">POST</span>
                                            <span class="font-mono text-slate-200 text-sm">/mcp/message</span>
                                            <span class="text-slate-500 text-sm">— JSON-RPC messages</span>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div class="card bg-slate-900/50 border border-slate-800">
                                <div class="card-body">
                                    <h3 class="text-lg font-semibold text-slate-100 mb-4">Configuration</h3>
                                    <pre class="bg-slate-950 rounded-lg p-4 text-sm text-slate-300 overflow-x-auto font-mono"><code>mcp:
  enabled: true
  api_key: ${TIERSUM_API_KEY}  # Optional override</code></pre>
                                </div>
                            </div>
                        </div>
                    </main>
                </div>
            </div>
        </div>
    `
};