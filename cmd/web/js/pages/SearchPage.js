import { apiClient } from '../api_client.js';
import { parseMarkdown } from '../markdown.js';

export const SearchPage = {
    data() {
        return {
            query: '',
            loading: false,
            results: [],
            hasSearched: false,
            aiAnswer: '',
            aiLoading: false,
            highlightedRef: null,
            traceDebug: false,
            lastTraceID: null,
            /** null until GET /bff/v1/monitoring — whether OpenTelemetry HTTP tracing + DB export is active */
            httpTracingActive: null
        };
    },
    mounted() {
        this.refreshTelemetryHint();
    },
    computed: {
        hasResults() {
            return this.results && this.results.length > 0;
        },
        traceSampleBlocked() {
            return this.traceDebug && this.httpTracingActive === false;
        }
    },
    methods: {
        async refreshTelemetryHint() {
            try {
                const m = await apiClient.getMonitoring();
                this.httpTracingActive = m?.telemetry?.http_tracing_active === true;
            } catch {
                this.httpTracingActive = null;
            }
        },
        async handleSearch() {
            if (!this.query.trim()) return;

            this.loading = true;
            this.aiLoading = true;
            this.hasSearched = true;
            this.aiAnswer = '';
            this.results = [];
            this.lastTraceID = null;

            try {
                const response = await apiClient.progressiveQuery(this.query, { trace: this.traceDebug });
                this.results = (response.results || []).map(r => ({
                    ...r,
                    docStatus: (r.status && String(r.status).trim()) || 'hot'
                }));
                const serverAnswer = (response.answer || '').trim();
                if (serverAnswer) {
                    this.aiAnswer = serverAnswer;
                } else {
                    await this.generateAiAnswerFallback();
                }
                if (response.trace_id) {
                    this.lastTraceID = response.trace_id;
                }
            } catch (error) {
                console.error('Search failed:', error);
            } finally {
                this.loading = false;
                this.aiLoading = false;
            }
        },

        async generateAiAnswerFallback() {
            await new Promise(resolve => setTimeout(resolve, 300));
            if (this.results.length === 0) {
                this.aiAnswer = 'No reference excerpts were found. Try different keywords or ingest more documents.';
                return;
            }
            const topResults = this.results.slice(0, 3);
            this.aiAnswer = `No server-generated answer was returned (LLM may be unavailable). Showing a quick preview from the top references:

${topResults.map((r) => `- **${r.title}** (relevance ${(r.relevance * 100).toFixed(0)}%)`).join('\n')}

${topResults[0]?.content?.substring(0, 280) || ''}${topResults[0]?.content?.length > 280 ? '…' : ''}`;
        },

        handleKeyDown(e) {
            if (e.key === 'Enter') {
                this.handleSearch();
            }
        },

        handleCitationClick(refNum) {
            this.highlightedRef = refNum;
            const element = document.getElementById(`ref-${refNum}`);
            if (element) {
                element.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }
        },

        renderMarkdown(content) {
            return parseMarkdown(content);
        },

        extractChapterPath(path) {
            const parts = path?.split('/') || [];
            return parts.length > 1 ? parts.slice(1).join('/') : '';
        },

        /** Short plain-text snippet for reference card body (scrolls inside fixed-height card). */
        refSnippet(result) {
            const raw = (result?.content || '').replace(/\s+/g, ' ').trim();
            if (!raw) return '—';
            const max = 400;
            return raw.length > max ? `${raw.slice(0, max)}…` : raw;
        },

        refTierLabel(docStatus) {
            const s = (docStatus || '').toLowerCase();
            if (s === 'hot') return 'Hot';
            if (s === 'cold') return 'Cold';
            if (s === 'warming') return 'Warming';
            return s ? s : 'Unknown';
        },

        refTierBadgeClass(docStatus) {
            const s = (docStatus || '').toLowerCase();
            if (s === 'hot') return 'badge-warning';
            if (s === 'cold') return 'badge-info';
            if (s === 'warming') return 'badge-secondary';
            return 'badge-ghost';
        },

        /** Progressive query: where `content` text came from (API `content_source`). */
        contentSourceLabel(src) {
            const s = (src || '').toLowerCase();
            if (s === 'chapter_summary') return 'Summary';
            if (s === 'bm25') return 'BM25';
            if (s === 'vector') return 'Vector';
            if (s === 'hybrid') return 'Hybrid (BM25+vector)';
            return src || '—';
        },

        /** Open document detail with optional chapter path (matches API chapter `path`). */
        goToDocumentFromSearch(result) {
            const docId = result?.id;
            if (!docId) return;
            const path = (result.path && String(result.path).trim()) || '';
            const query = path ? { path } : {};
            this.$router.push({ path: `/docs/${docId}`, query });
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div :class="['transition-all duration-500', hasSearched ? 'mb-6' : 'mb-0 mt-32']">
                    <div v-if="!hasSearched" class="text-center mb-8">
                        <h1 class="text-4xl font-bold text-slate-100 mb-4">
                            Search Your Knowledge Base
                        </h1>
                        <p class="text-slate-400 text-lg max-w-2xl mx-auto">
                            AI-powered search with hierarchical summarization.
                            Find exactly what you need across all your documents.
                        </p>
                    </div>

                    <div class="max-w-3xl mx-auto relative">
                        <div class="relative group">
                            <svg class="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-500 group-focus-within:text-blue-500 transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                            </svg>
                            <input
                                v-model="query"
                                @keydown="handleKeyDown"
                                placeholder="Ask anything about your documents..."
                                class="w-full h-14 pl-12 pr-32 text-lg bg-slate-900/50 border border-slate-800 text-slate-100 placeholder:text-slate-500 focus:border-blue-500/50 focus:ring-2 focus:ring-blue-500/20 rounded-xl outline-none transition-all"
                            />
                            <button
                                @click="handleSearch"
                                :disabled="loading || !query.trim()"
                                class="absolute right-2 top-1/2 -translate-y-1/2 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-700 disabled:cursor-not-allowed text-white px-6 py-2 rounded-lg font-medium transition-colors"
                            >
                                <span v-if="loading" class="flex items-center">
                                    <svg class="animate-spin -ml-1 mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24">
                                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                    </svg>
                                    Searching...
                                </span>
                                <span v-else>Search</span>
                            </button>
                        </div>
                        <div class="mt-3 flex flex-col items-center gap-2 text-sm text-slate-400">
                            <p v-if="traceSampleBlocked" class="text-amber-400/90 text-xs max-w-xl text-center">
                                Trace sample is on, but the server is not exporting HTTP spans to the database.
                                Set <code class="text-amber-200/90">telemetry.enabled: true</code> and
                                <code class="text-amber-200/90">telemetry.persist_to_db: true</code> in config, then restart.
                                <button type="button" class="link link-primary text-xs ml-1" @click="refreshTelemetryHint">Recheck</button>
                            </p>
                            <div class="flex flex-wrap items-center justify-center gap-3">
                            <label class="flex items-center gap-2 cursor-pointer select-none">
                                <input type="checkbox" v-model="traceDebug" class="checkbox checkbox-sm checkbox-primary" />
                                <span
                                    class="text-sm"
                                    title="Appends ?debug_trace=1 to force-record this request. Requires telemetry.enabled and telemetry.persist_to_db (see GET /bff/v1/monitoring telemetry.http_tracing_active). Also requires query.allow_progressive_debug for detailed progressive spans."
                                >Trace sample (OpenTelemetry)</span>
                            </label>
                            <router-link
                                v-if="lastTraceID && hasSearched"
                                :to="{ path: '/observability', query: { tab: 'traces', trace: lastTraceID } }"
                                class="btn btn-ghost btn-xs text-cyan-400"
                            >View trace in Observability</router-link>
                            </div>
                        </div>
                    </div>
                </div>

                <div v-if="hasSearched" class="grid grid-cols-1 lg:grid-cols-12 gap-6 mt-8">
                    <div class="lg:col-span-8">
                        <div class="card bg-slate-900/50 border border-slate-800 h-[calc(100vh-280px)]">
                            <div class="card-body p-0">
                                <div class="p-4 border-b border-slate-800 flex items-center justify-between">
                                    <div class="flex items-center gap-2">
                                        <svg class="w-5 h-5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"/>
                                        </svg>
                                        <h2 class="text-lg font-semibold text-slate-100">AI Answer</h2>
                                    </div>
                                    <span class="badge badge-outline badge-success">
                                        Based on {{ results.length }} references
                                    </span>
                                </div>
                                <div class="p-6 overflow-y-auto h-[calc(100%-80px)]">
                                    <div v-if="aiLoading" class="space-y-4">
                                        <div class="h-4 bg-slate-800 rounded animate-pulse w-full"></div>
                                        <div class="h-4 bg-slate-800 rounded animate-pulse w-5/6"></div>
                                        <div class="h-4 bg-slate-800 rounded animate-pulse w-4/6"></div>
                                        <div class="h-20 bg-slate-800 rounded animate-pulse w-full mt-4"></div>
                                    </div>
                                    <div v-else-if="aiAnswer" class="markdown-body max-w-none px-0 py-0 text-[15px] leading-relaxed" v-html="renderMarkdown(aiAnswer)"></div>
                                    <div v-else class="text-center py-12 text-slate-500">
                                        <p>Generating AI answer...</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="lg:col-span-4">
                        <div class="card bg-slate-900/50 border border-slate-800 h-[calc(100vh-280px)]">
                            <div class="card-body p-0">
                                <div class="p-4 border-b border-slate-800 flex items-center justify-between">
                                    <div class="flex items-center gap-2">
                                        <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                                        </svg>
                                        <h2 class="text-lg font-semibold text-slate-100">References</h2>
                                    </div>
                                    <span class="badge bg-slate-800 text-slate-300">{{ results.length }} items</span>
                                </div>
                                <div class="p-3 overflow-y-auto h-[calc(100%-72px)] flex flex-col gap-3">
                                    <div v-if="loading" class="space-y-4">
                                        <div v-for="i in 3" :key="i" class="p-4 rounded-lg bg-slate-800/50 space-y-3">
                                            <div class="h-5 bg-slate-700 rounded animate-pulse w-3/4"></div>
                                            <div class="h-4 bg-slate-700 rounded animate-pulse w-full"></div>
                                            <div class="h-4 bg-slate-700 rounded animate-pulse w-2/3"></div>
                                        </div>
                                    </div>
                                    <div v-else-if="results.length === 0" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                                        </svg>
                                        <p>No references found</p>
                                    </div>
                                    <div v-else>
                                        <div v-for="(result, index) in results" :key="(result.path || result.id || '') + '-' + index"
                                             :id="'ref-' + index"
                                             :class="['rounded-xl border bg-slate-800/30 transition-all cursor-pointer flex flex-col max-h-[248px] overflow-hidden shrink-0',
                                                      highlightedRef === index ? 'border-blue-500 ring-2 ring-blue-500/50' : 'border-slate-700 hover:border-slate-600']"
                                             @click="highlightedRef = index">
                                            <div class="p-3 flex flex-col min-h-0 flex-1 gap-2">
                                                <div class="flex justify-between items-start gap-2 shrink-0">
                                                    <div class="flex items-center gap-2 min-w-0">
                                                        <span :class="['badge badge-sm shrink-0', refTierBadgeClass(result.docStatus)]">
                                                            {{ refTierLabel(result.docStatus) }}
                                                        </span>
                                                        <span class="text-xs text-slate-500 shrink-0">#{{ index + 1 }}</span>
                                                    </div>
                                                    <span class="badge badge-outline badge-sm shrink-0">{{ (result.relevance * 100).toFixed(0) }}%</span>
                                                </div>
                                                <h3 class="font-semibold text-slate-200 line-clamp-2 text-sm leading-snug shrink-0">{{ result.title }}</h3>
                                                <div class="text-[11px] text-slate-500 space-y-0.5 shrink-0 leading-tight">
                                                    <p class="font-mono truncate" :title="result.id"><span class="text-slate-600">ID</span> {{ result.id }}</p>
                                                    <p class="truncate"><span class="text-slate-600">Source</span> <span class="text-slate-400">{{ contentSourceLabel(result.content_source) }}</span></p>
                                                    <p class="truncate" :title="extractChapterPath(result.path) || ''"><span class="text-slate-600">Path</span> {{ extractChapterPath(result.path) || '(whole doc)' }}</p>
                                                </div>
                                                <div class="flex-1 min-h-0 overflow-y-auto overscroll-contain text-xs text-slate-400 leading-relaxed border border-slate-700/40 rounded-lg px-2 py-1.5 bg-slate-900/40">
                                                    {{ refSnippet(result) }}
                                                </div>
                                                <div class="flex justify-end items-center shrink-0 pt-1 border-t border-slate-700/50">
                                                    <button type="button" @click.stop="goToDocumentFromSearch(result)" class="text-xs font-medium text-blue-400 hover:text-blue-300 flex items-center gap-1">
                                                        Open <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
                                                    </button>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    `
};
