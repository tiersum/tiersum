import { apiClient } from '../api_client.js';
import { MonitoringPage } from './MonitoringPage.js';

export const ObservabilityPage = {
    components: { MonitoringPage },
    data() {
        return {
            tab: 'monitoring',
            traces: [],
            tracesError: null,
            tracesLoading: false,
            selectedTraceId: null,
            spans: [],
            spansError: null,
            spansLoading: false,
            attrsModalOpen: false,
            attrsModalSpanName: '',
            attrsModalRows: [],
            coldQ: '',
            coldMaxResults: 20,
            coldLoading: false,
            coldError: '',
            coldItems: [],
            coldRan: false,
            coldExpanded: {}
        };
    },
    mounted() {
        const q = new URLSearchParams(window.location.search || '');
        const t = (q.get('tab') || '').toLowerCase();
        const tid = (q.get('trace') || '').trim();
        if (tid) {
            this.tab = 'traces';
            this.loadTraces();
            this.openTrace(tid);
        } else if (t === 'traces') {
            this.tab = 'traces';
            this.loadTraces();
        } else if (t === 'cold') {
            this.tab = 'cold';
        } else {
            this.tab = 'monitoring';
        }
    },
    watch: {
        $route() {
            this.syncFromRoute();
        }
    },
    methods: {
        syncFromRoute() {
            const q = new URLSearchParams(this.$route.query || {});
            const t = (q.get('tab') || '').toLowerCase();
            const tid = (q.get('trace') || '').trim();
            if (tid && tid !== this.selectedTraceId) {
                this.openTrace(tid);
                return;
            }
            if (t === 'traces') this.tab = 'traces';
            else if (t === 'cold') this.tab = 'cold';
            else this.tab = 'monitoring';
        },
        tabQueryFor(name) {
            if (name === 'traces') return 'traces';
            if (name === 'cold') return 'cold';
            return 'monitoring';
        },
        setTab(name) {
            this.tab = name;
            const next = { ...this.$route.query, tab: this.tabQueryFor(name) };
            if (name !== 'traces') {
                delete next.trace;
            }
            this.$router.replace({ query: next });
            if (name === 'traces' && !this.traces.length && !this.tracesLoading) {
                this.loadTraces();
            }
        },
        async loadTraces() {
            this.tracesLoading = true;
            this.tracesError = null;
            try {
                const data = await apiClient.listTraces({ limit: 50, offset: 0 });
                this.traces = data.traces || [];
            } catch (e) {
                this.tracesError = e.message || String(e);
                this.traces = [];
            } finally {
                this.tracesLoading = false;
            }
        },
        async openTrace(traceId) {
            this.selectedTraceId = traceId;
            this.spans = [];
            this.spansError = null;
            this.spansLoading = true;
            this.$router.replace({ query: { ...this.$route.query, tab: 'traces', trace: traceId } });
            try {
                const data = await apiClient.getTrace(traceId);
                this.spans = data.spans || [];
            } catch (e) {
                this.spansError = e.message || String(e);
            } finally {
                this.spansLoading = false;
            }
        },
        formatNanoRange(startNs, endNs) {
            const d = (Number(endNs) - Number(startNs)) / 1e6;
            if (!Number.isFinite(d) || d < 0) return '—';
            return `${d.toFixed(1)} ms`;
        },
        spanAttrsPreview(jsonStr) {
            if (!jsonStr) return '';
            try {
                const o = JSON.parse(jsonStr);
                const keys = Object.keys(o);
                return keys.slice(0, 6).map((k) => `${k}=${String(o[k]).slice(0, 80)}`).join('; ');
            } catch {
                return jsonStr.slice(0, 200);
            }
        },
        spanAttrsKeySort(a, b) {
            const rank = (k) => {
                if (k.startsWith('tier.request.')) return 0;
                if (k.startsWith('tier.llm.request.')) return 1;
                if (k.startsWith('tier.response.')) return 2;
                if (k.startsWith('tier.llm.response.')) return 3;
                return 4;
            };
            const ra = rank(a);
            const rb = rank(b);
            if (ra !== rb) return ra - rb;
            return a.localeCompare(b);
        },
        openAttrsModal(sp) {
            this.attrsModalSpanName = (sp && sp.name) || '';
            this.attrsModalRows = [];
            const raw = sp && sp.attributes_json;
            if (!raw || !String(raw).trim()) {
                this.attrsModalOpen = true;
                return;
            }
            try {
                const o = JSON.parse(raw);
                const keys = Object.keys(o).sort((a, b) => this.spanAttrsKeySort(a, b));
                for (const k of keys) {
                    let v = o[k];
                    if (v !== null && typeof v === 'object') {
                        v = JSON.stringify(v, null, 2);
                    } else if (v === undefined) {
                        v = '';
                    } else {
                        v = String(v);
                    }
                    this.attrsModalRows.push({ key: k, value: v });
                }
            } catch {
                this.attrsModalRows = [{ key: '(unparsed JSON)', value: String(raw) }];
            }
            this.attrsModalOpen = true;
        },
        closeAttrsModal() {
            this.attrsModalOpen = false;
            this.attrsModalSpanName = '';
            this.attrsModalRows = [];
        },
        normalizeColdKeywords(raw) {
            return String(raw || '')
                .split(/[\s,]+/)
                .map((s) => s.trim())
                .filter(Boolean)
                .join(',');
        },
        async runColdProbe() {
            const q = this.normalizeColdKeywords(this.coldQ);
            if (!q) {
                this.coldError = 'Enter at least one keyword (spaces or commas).';
                this.coldItems = [];
                this.coldRan = true;
                return;
            }
            this.coldLoading = true;
            this.coldError = '';
            this.coldItems = [];
            this.coldExpanded = {};
            try {
                const data = await apiClient.getColdDocSource(q, this.coldMaxResults);
                this.coldItems = data.items || [];
            } catch (e) {
                this.coldError = e.message || String(e);
                this.coldItems = [];
            } finally {
                this.coldLoading = false;
                this.coldRan = true;
            }
        },
        toggleColdExpand(i) {
            this.coldExpanded = { ...this.coldExpanded, [i]: !this.coldExpanded[i] };
        },
        coldContextPreview(text) {
            const t = String(text || '');
            if (t.length <= 320) return t;
            return `${t.slice(0, 320)}…`;
        },
        formatColdScore(s) {
            const n = Number(s);
            if (!Number.isFinite(n)) return '—';
            return n.toFixed(4);
        },
        traceWaterfallRows() {
            if (!this.spans.length) return [];
            let t0 = Infinity;
            for (const s of this.spans) {
                const v = Number(s.start_time_unix_nano);
                if (v < t0) t0 = v;
            }
            if (!Number.isFinite(t0)) t0 = 0;
            const total = Math.max(
                1,
                ...this.spans.map((s) => Number(s.end_time_unix_nano) - t0)
            );
            const depth = (id) => {
                let d = 0;
                let cur = this.spans.find((x) => x.span_id === id);
                while (cur && cur.parent_span_id) {
                    d++;
                    cur = this.spans.find((x) => x.span_id === cur.parent_span_id);
                    if (d > 32) break;
                }
                return d;
            };
            return this.spans.map((s) => {
                const st = Number(s.start_time_unix_nano);
                const en = Number(s.end_time_unix_nano);
                const rel = (st - t0) / total;
                const w = Math.max(0.002, (en - st) / total);
                return { s, depth: depth(s.span_id), leftPct: rel * 100, widthPct: w * 100, total };
            });
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <div
                v-if="attrsModalOpen"
                class="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm"
                role="dialog"
                aria-modal="true"
                aria-labelledby="attrs-modal-title"
                @click.self="closeAttrsModal"
            >
                <div class="bg-slate-900 border border-slate-700 rounded-xl shadow-2xl w-full max-w-4xl max-h-[88vh] flex flex-col" @click.stop>
                    <div class="flex items-start justify-between gap-3 px-4 py-3 border-b border-slate-800 shrink-0">
                        <div class="min-w-0">
                            <h3 id="attrs-modal-title" class="text-sm font-semibold text-slate-200">Span attributes</h3>
                            <p v-if="attrsModalSpanName" class="text-xs font-mono text-cyan-300/90 truncate mt-0.5" :title="attrsModalSpanName">{{ attrsModalSpanName }}</p>
                        </div>
                        <button type="button" class="btn btn-sm btn-ghost text-slate-400 shrink-0" @click="closeAttrsModal">Close</button>
                    </div>
                    <div class="overflow-auto flex-1 min-h-0 p-3">
                        <p v-if="!attrsModalRows.length" class="text-sm text-slate-500 px-1">No attributes on this span.</p>
                        <table v-else class="table table-sm w-full border border-slate-800 rounded-lg overflow-hidden">
                            <thead class="sticky top-0 z-10 bg-slate-800/95 text-slate-300 text-xs uppercase tracking-wide">
                                <tr>
                                    <th class="w-[28%] min-w-[8rem] align-top">Key</th>
                                    <th class="align-top">Value</th>
                                </tr>
                            </thead>
                            <tbody class="text-xs">
                                <tr v-for="(row, i) in attrsModalRows" :key="i" class="hover:bg-slate-800/50 border-b border-slate-800/80 last:border-0">
                                    <td class="font-mono text-cyan-200/90 align-top whitespace-nowrap">{{ row.key }}</td>
                                    <td class="text-slate-300 align-top">
                                        <pre class="whitespace-pre-wrap break-words font-sans text-[11px] leading-relaxed m-0 max-h-64 overflow-y-auto">{{ row.value }}</pre>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
            <main class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div class="mb-6">
                    <h1 class="text-2xl sm:text-3xl font-bold text-slate-100">Observability</h1>
                    <p class="text-slate-500 text-sm mt-1">Monitoring, cold-index debugging, and persisted progressive-query traces (OpenTelemetry).</p>
                </div>
                <div role="tablist" class="tabs tabs-boxed bg-slate-900/80 border border-slate-800 mb-6 flex flex-wrap gap-y-1 w-full max-w-3xl">
                    <a role="tab" :class="['tab', tab === 'monitoring' ? 'tab-active' : '']" @click.prevent="setTab('monitoring')">Monitoring</a>
                    <a role="tab" :class="['tab', tab === 'cold' ? 'tab-active' : '']" @click.prevent="setTab('cold')">Cold probe</a>
                    <a role="tab" :class="['tab', tab === 'traces' ? 'tab-active' : '']" @click.prevent="setTab('traces')">Traces</a>
                </div>

                <div v-if="tab === 'monitoring'">
                    <monitoring-page />
                </div>

                <div v-if="tab === 'cold'" class="space-y-4">
                    <div>
                        <h2 class="text-lg font-semibold text-slate-200">Cold chapter probe</h2>
                        <p class="text-[10px] font-mono text-slate-500 mt-0.5">GET /bff/v1/cold/doc_source</p>
                    </div>
                    <p class="text-sm text-slate-500 leading-relaxed max-w-3xl">
                        Direct hybrid search (Bleve BM25 + HNSW vector) over <strong class="text-slate-400">cold</strong> document chapters.
                        Same endpoint as <code class="text-cyan-600/90">/api/v1/cold/doc_source</code>. Use this to verify hits, scores, and
                        <code class="text-cyan-600/90">source</code> (e.g. bm25 / vector / hybrid) without running progressive query.
                    </p>
                    <div class="flex flex-wrap items-end gap-2">
                        <label class="form-control min-w-[12rem] flex-1 max-w-xl">
                            <span class="label py-0"><span class="label-text text-xs text-slate-400">Keywords</span></span>
                            <input
                                v-model="coldQ"
                                type="text"
                                class="input input-bordered input-sm bg-slate-950 border-slate-700 w-full"
                                placeholder="scheduler, pods"
                                autocomplete="off"
                                @keydown.enter.prevent="runColdProbe"
                            />
                        </label>
                        <label class="form-control w-28">
                            <span class="label py-0"><span class="label-text text-xs text-slate-400">max_results</span></span>
                            <input
                                v-model.number="coldMaxResults"
                                type="number"
                                min="1"
                                max="500"
                                class="input input-bordered input-sm bg-slate-950 border-slate-700 w-full"
                            />
                        </label>
                        <button
                            type="button"
                            class="btn btn-primary btn-sm"
                            :disabled="coldLoading"
                            @click="runColdProbe"
                        >
                            {{ coldLoading ? 'Running…' : 'Run probe' }}
                        </button>
                    </div>
                    <p v-if="coldError" class="text-sm text-red-400 whitespace-pre-wrap">{{ coldError }}</p>
                    <p v-else-if="coldRan && !coldLoading && !coldItems.length" class="text-sm text-slate-500">No hits for these terms.</p>
                    <div v-if="coldItems.length" class="overflow-x-auto rounded-lg border border-slate-800">
                        <table class="table table-sm">
                            <thead>
                                <tr class="bg-slate-900/90 text-slate-400 text-xs">
                                    <th>Title</th>
                                    <th>Path</th>
                                    <th>Source</th>
                                    <th>Score</th>
                                    <th>Document</th>
                                    <th>Context</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr v-for="(row, i) in coldItems" :key="i" class="align-top hover:bg-slate-800/30">
                                    <td class="text-sm text-slate-200 max-w-[10rem]">{{ row.title || '—' }}</td>
                                    <td class="font-mono text-[11px] text-cyan-200/80 max-w-[12rem] break-all">{{ row.path || '—' }}</td>
                                    <td class="text-xs text-amber-200/90">{{ row.source || '—' }}</td>
                                    <td class="text-xs text-slate-400 whitespace-nowrap">{{ formatColdScore(row.score) }}</td>
                                    <td class="text-xs">
                                        <router-link
                                            v-if="row.document_id"
                                            :to="'/docs/' + row.document_id"
                                            class="link link-primary"
                                        >Open</router-link>
                                        <span v-else class="text-slate-600">—</span>
                                    </td>
                                    <td class="max-w-md min-w-[8rem]">
                                        <p class="text-[11px] text-slate-400 m-0 whitespace-pre-wrap break-words">
                                            {{ coldExpanded[i] ? row.context : coldContextPreview(row.context) }}
                                        </p>
                                        <button
                                            v-if="row.context && String(row.context).length > 320"
                                            type="button"
                                            class="btn btn-link btn-xs text-violet-300 px-0 min-h-0 h-auto"
                                            @click="toggleColdExpand(i)"
                                        >
                                            {{ coldExpanded[i] ? 'Show less' : 'Show full chapter' }}
                                        </button>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>

                <div v-if="tab === 'traces'" class="space-y-6">
                    <div class="flex items-center justify-between gap-4">
                        <h2 class="text-lg font-semibold text-slate-200">Stored traces</h2>
                        <button type="button" class="btn btn-sm btn-outline border-slate-600" :disabled="tracesLoading" @click="loadTraces">Refresh list</button>
                    </div>
                    <p v-if="tracesError" class="text-sm text-red-400">{{ tracesError }}</p>
                    <div v-else-if="tracesLoading" class="text-slate-500 text-sm">Loading…</div>
                    <div v-else class="overflow-x-auto rounded-lg border border-slate-800">
                        <table class="table table-sm">
                            <thead>
                                <tr class="bg-slate-900/80 text-slate-400">
                                    <th>Trace ID</th>
                                    <th>Root span</th>
                                    <th>Spans</th>
                                    <th>Duration</th>
                                    <th></th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr v-for="t in traces" :key="t.trace_id" class="hover:bg-slate-800/40">
                                    <td class="font-mono text-xs text-cyan-300/90">{{ t.trace_id }}</td>
                                    <td class="text-sm text-slate-300">{{ t.root_span_name || '—' }}</td>
                                    <td class="text-slate-400">{{ t.span_count }}</td>
                                    <td class="text-slate-400 text-xs">{{ formatNanoRange(t.started_at_unix_nano, t.ended_at_unix_nano) }}</td>
                                    <td>
                                        <button type="button" class="btn btn-ghost btn-xs" @click="openTrace(t.trace_id)">Waterfall</button>
                                    </td>
                                </tr>
                                <tr v-if="!traces.length">
                                    <td colspan="5" class="text-slate-500 text-sm">No traces yet. Run a progressive search with “Trace sample” enabled (or rely on sampling).</td>
                                </tr>
                            </tbody>
                        </table>
                    </div>

                    <div v-if="selectedTraceId" class="card bg-slate-900/50 border border-slate-800">
                        <div class="card-body">
                            <h3 class="card-title text-slate-200 text-base">Trace <span class="font-mono text-cyan-300/90 text-sm">{{ selectedTraceId }}</span></h3>
                            <p v-if="spansError" class="text-sm text-red-400">{{ spansError }}</p>
                            <div v-else-if="spansLoading" class="text-slate-500 text-sm py-4">Loading spans…</div>
                            <div v-else class="space-y-4 mt-2">
                                <div class="space-y-1">
                                    <div v-for="(row, idx) in traceWaterfallRows()" :key="idx" class="flex items-center gap-2 min-h-[26px]">
                                        <div class="w-8 text-[10px] text-slate-500 text-right shrink-0">{{ row.depth }}</div>
                                        <div class="flex-1 relative h-6 bg-slate-800/60 rounded overflow-hidden min-w-0">
                                            <div
                                                class="absolute top-1 bottom-1 rounded bg-violet-600/85 border border-violet-400/30 min-w-[2px]"
                                                :title="row.s.name + ' — ' + formatNanoRange(row.s.start_time_unix_nano, row.s.end_time_unix_nano)"
                                                :style="{ left: row.leftPct + '%', width: row.widthPct + '%' }"
                                            ></div>
                                        </div>
                                        <div class="w-40 lg:w-52 truncate text-[11px] text-slate-400 shrink-0" :title="row.s.name">{{ row.s.name }}</div>
                                    </div>
                                </div>
                                <div class="overflow-x-auto max-h-96 overflow-y-auto rounded border border-slate-800">
                                    <table class="table table-xs">
                                        <thead><tr class="text-slate-400"><th>Name</th><th>Kind</th><th>Status</th><th>Duration</th><th>Attributes</th></tr></thead>
                                        <tbody>
                                            <tr v-for="sp in spans" :key="sp.span_id">
                                                <td class="font-mono text-xs text-slate-200">{{ sp.name }}</td>
                                                <td class="text-xs text-slate-500">{{ sp.kind }}</td>
                                                <td class="text-xs">{{ sp.status_code }}</td>
                                                <td class="text-xs text-slate-400">{{ formatNanoRange(sp.start_time_unix_nano, sp.end_time_unix_nano) }}</td>
                                                <td class="max-w-xs lg:max-w-md">
                                                    <div class="flex items-start gap-2 min-w-0">
                                                        <p class="text-[10px] text-slate-500 truncate flex-1 min-w-0 m-0" :title="spanAttrsPreview(sp.attributes_json)">{{ spanAttrsPreview(sp.attributes_json) || '—' }}</p>
                                                        <button type="button" class="btn btn-ghost btn-xs shrink-0 text-violet-300/90 hover:text-violet-200" @click="openAttrsModal(sp)">Table</button>
                                                    </div>
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    `
};
