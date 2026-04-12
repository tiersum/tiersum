import { apiClient } from '../api_client.js';

export const MonitoringPage = {
    data() {
        return {
            loading: true,
            error: null,
            health: null,
            monitoring: null,
            metricsText: '',
            metricsError: null,
            metricsLoading: false,
            lastRefresh: null
        };
    },
    mounted() {
        this.refreshAll();
    },
    methods: {
        async refreshAll() {
            this.loading = true;
            this.error = null;
            try {
                const [health, mon] = await Promise.all([
                    fetch(`${apiClient.baseURL}/health`).then((r) => {
                        if (!r.ok) throw new Error(`health: HTTP ${r.status}`);
                        return r.json();
                    }),
                    apiClient.getMonitoring()
                ]);
                this.health = health;
                this.monitoring = mon;
                this.lastRefresh = new Date();
            } catch (e) {
                this.error = e.message || String(e);
            } finally {
                this.loading = false;
            }
        },
        async loadMetricsPreview() {
            this.metricsLoading = true;
            this.metricsError = null;
            this.metricsText = '';
            try {
                this.metricsText = await apiClient.getMetricsText();
            } catch (e) {
                this.metricsError = e.message || String(e);
            } finally {
                this.metricsLoading = false;
            }
        },
        formatTime(iso) {
            if (!iso) return '—';
            try {
                return new Date(iso).toLocaleString();
            } catch {
                return String(iso);
            }
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-8">
                    <div>
                        <h1 class="text-2xl sm:text-3xl font-bold text-slate-100">Monitoring</h1>
                        <p class="text-slate-500 text-sm mt-1">Service health, document counts, quota, and Prometheus metrics.</p>
                    </div>
                    <button type="button" class="btn btn-outline border-slate-600 btn-sm" :disabled="loading" @click="refreshAll">
                        <span v-if="loading" class="loading loading-spinner loading-sm"></span>
                        Refresh
                    </button>
                </div>

                <div v-if="loading && !monitoring" class="space-y-4">
                    <div class="h-24 bg-slate-800/80 rounded-xl animate-pulse"></div>
                    <div class="h-40 bg-slate-800/80 rounded-xl animate-pulse"></div>
                </div>

                <div v-else-if="error" class="alert alert-error bg-red-950/40 border-red-900 text-red-200">
                    {{ error }}
                </div>

                <div v-else class="space-y-6">
                    <p v-if="lastRefresh" class="text-xs text-slate-600">Last updated: {{ lastRefresh.toLocaleString() }}</p>

                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <h2 class="card-title text-slate-200 text-base">Health</h2>
                                <dl class="text-sm space-y-2 mt-2">
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Status</dt><dd class="text-slate-200 font-medium">{{ health?.status || '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Version</dt><dd class="text-slate-200 font-mono text-xs">{{ health?.version || '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Cold index (rows)</dt><dd class="text-slate-200">{{ health?.cold_doc_count ?? '—' }}</dd></div>
                                </dl>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <h2 class="card-title text-slate-200 text-base">Build / API</h2>
                                <dl class="text-sm space-y-2 mt-2">
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Module version</dt><dd class="text-slate-200 font-mono text-xs">{{ monitoring?.server?.version || '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Cold chapters (approx)</dt><dd class="text-slate-200">{{ monitoring?.cold_index?.approx_chapters ?? '—' }}</dd></div>
                                </dl>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800 lg:col-span-2">
                            <div class="card-body">
                                <h2 class="card-title text-slate-200 text-base">Go runtime</h2>
                                <p class="text-xs text-slate-500 mt-1">Process Go toolchain and OS/arch (from <code class="text-slate-600">runtime</code>).</p>
                                <dl class="text-sm space-y-2 mt-2 grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-2">
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Version</dt><dd class="text-slate-200 font-mono text-xs">{{ monitoring?.go?.version || '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">GOOS / GOARCH</dt><dd class="text-slate-200 font-mono text-xs">{{ monitoring?.go?.goos || '—' }} / {{ monitoring?.go?.goarch || '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Compiler</dt><dd class="text-slate-200 font-mono text-xs">{{ monitoring?.go?.compiler || '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">NumCPU / GOMAXPROCS</dt><dd class="text-slate-200 font-mono text-xs">{{ monitoring?.go?.num_cpu ?? '—' }} / {{ monitoring?.go?.gomaxprocs ?? '—' }}</dd></div>
                                </dl>
                            </div>
                        </div>
                    </div>

                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <h2 class="card-title text-slate-200 text-base">Vector index (HNSW)</h2>
                                <p class="text-xs text-slate-500 mt-1">Dense embeddings per cold chapter for hybrid retrieval.</p>
                                <dl class="text-sm space-y-2 mt-2">
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">HNSW nodes</dt><dd class="text-slate-200 font-mono">{{ monitoring?.cold_index?.vector?.hnsw_nodes ?? '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Vector dimension</dt><dd class="text-slate-200 font-mono">{{ monitoring?.cold_index?.vector?.vector_dim ?? '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">M / ef_search</dt><dd class="text-slate-200 font-mono">{{ monitoring?.cold_index?.vector?.hnsw_m ?? '—' }} / {{ monitoring?.cold_index?.vector?.hnsw_ef_search ?? '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Text embedder configured</dt><dd class="text-slate-200">{{ monitoring?.cold_index?.vector?.text_embedder_configured === true ? 'Yes' : monitoring?.cold_index?.vector?.text_embedder_configured === false ? 'No' : '—' }}</dd></div>
                                </dl>
                            </div>
                        </div>
                        <div class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <h2 class="card-title text-slate-200 text-base">Inverted index (Bleve)</h2>
                                <p class="text-xs text-slate-500 mt-1">Full-text BM25 search over cold chapters (in-memory inverted index).</p>
                                <dl class="text-sm space-y-2 mt-2">
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Bleve documents</dt><dd class="text-slate-200 font-mono">{{ monitoring?.cold_index?.inverted?.bleve_doc_count ?? '—' }}</dd></div>
                                    <div class="flex justify-between gap-4"><dt class="text-slate-500">Storage backend</dt><dd class="text-slate-200 font-mono text-xs break-all">{{ monitoring?.cold_index?.inverted?.storage_backend || '—' }}</dd></div>
                                    <div class="flex flex-col gap-1 sm:flex-row sm:justify-between sm:gap-4"><dt class="text-slate-500 shrink-0">Text analyzer</dt><dd class="text-slate-200 text-xs break-words sm:text-right">{{ monitoring?.cold_index?.inverted?.text_analyzer || '—' }}</dd></div>
                                </dl>
                            </div>
                        </div>
                    </div>

                    <div class="card bg-slate-900/50 border border-slate-800">
                        <div class="card-body">
                            <h2 class="card-title text-slate-200 text-base">Documents</h2>
                            <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mt-4">
                                <div class="text-center p-3 rounded-lg bg-slate-800/40">
                                    <div class="text-2xl font-bold text-slate-100">{{ monitoring?.documents?.total ?? 0 }}</div>
                                    <div class="text-xs text-slate-500 uppercase tracking-wide">Total</div>
                                </div>
                                <div class="text-center p-3 rounded-lg bg-amber-500/10 border border-amber-500/20">
                                    <div class="text-2xl font-bold text-amber-200">{{ monitoring?.documents?.hot ?? 0 }}</div>
                                    <div class="text-xs text-amber-500/80 uppercase tracking-wide">Hot</div>
                                </div>
                                <div class="text-center p-3 rounded-lg bg-sky-500/10 border border-sky-500/20">
                                    <div class="text-2xl font-bold text-sky-200">{{ monitoring?.documents?.cold ?? 0 }}</div>
                                    <div class="text-xs text-sky-500/80 uppercase tracking-wide">Cold</div>
                                </div>
                                <div class="text-center p-3 rounded-lg bg-slate-700/50">
                                    <div class="text-2xl font-bold text-slate-200">{{ monitoring?.documents?.warming ?? 0 }}</div>
                                    <div class="text-xs text-slate-500 uppercase tracking-wide">Warming</div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="card bg-slate-900/50 border border-slate-800">
                        <div class="card-body">
                            <h2 class="card-title text-slate-200 text-base">Hot ingest quota</h2>
                            <p class="text-sm text-slate-500 mt-1">Resets hourly (see server config quota.per_hour).</p>
                            <dl class="text-sm space-y-2 mt-4">
                                <div class="flex justify-between gap-4"><dt class="text-slate-500">Used / total</dt><dd class="text-slate-200">{{ monitoring?.quota?.used ?? 0 }} / {{ monitoring?.quota?.total ?? '—' }}</dd></div>
                                <div class="flex justify-between gap-4"><dt class="text-slate-500">Reset at</dt><dd class="text-slate-200">{{ formatTime(monitoring?.quota?.reset_at) }}</dd></div>
                            </dl>
                        </div>
                    </div>

                    <div class="card bg-slate-900/50 border border-slate-800">
                        <div class="card-body">
                            <div class="flex flex-wrap items-center justify-between gap-2">
                                <h2 class="card-title text-slate-200 text-base">Prometheus metrics</h2>
                                <div class="flex gap-2">
                                    <button type="button" class="btn btn-sm btn-outline border-slate-600" :disabled="metricsLoading" @click="loadMetricsPreview">
                                        Load preview
                                    </button>
                                    <a :href="(monitoring?.prometheus_metrics_path || '/metrics')" target="_blank" rel="noopener noreferrer"
                                        class="btn btn-sm btn-primary">Open raw</a>
                                </div>
                            </div>
                            <p v-if="metricsError" class="text-sm text-red-400 mt-2">{{ metricsError }}</p>
                            <pre v-else-if="metricsText" class="mt-4 p-4 rounded-lg bg-slate-950 border border-slate-800 text-xs text-emerald-300/90 overflow-x-auto max-h-80 overflow-y-auto whitespace-pre-wrap font-mono">{{ metricsText }}</pre>
                            <p v-else class="text-sm text-slate-600 mt-4">Load preview or open raw metrics in a new tab (<code>/metrics</code>, Prometheus default path; no API key).</p>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    `
};
