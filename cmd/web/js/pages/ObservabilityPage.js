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
            traceListQuery: '',
            selectedTraceId: null,
            spans: [],
            spansError: null,
            spansLoading: false,
            spanQuery: '',
            spanStatusFilter: 'all',
            spanKindFilter: 'all',
            spanShowErrorsOnly: false,
            spanShowSlowOnly: false,
            spanSlowThresholdMs: 50,
            spanSortBy: 'start', // start | duration
            spanView: 'tree', // tree | table
            focusedSpanId: null,
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
            this.focusedSpanId = null;
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
        traceRowMatches(t) {
            const q = String(this.traceListQuery || '').trim().toLowerCase();
            if (!q) return true;
            return (
                String(t.trace_id || '').toLowerCase().includes(q) ||
                String(t.root_span_name || '').toLowerCase().includes(q)
            );
        },
        filteredTraces() {
            return (this.traces || []).filter((t) => this.traceRowMatches(t));
        },
        formatUnixNanoAsLocal(ns) {
            const n = Number(ns);
            if (!Number.isFinite(n) || n <= 0) return '—';
            try {
                return new Date(n / 1e6).toLocaleString();
            } catch {
                return '—';
            }
        },
        spanDurationMs(sp) {
            const d = (Number(sp.end_time_unix_nano) - Number(sp.start_time_unix_nano)) / 1e6;
            return Number.isFinite(d) && d >= 0 ? d : NaN;
        },
        traceRootSpan() {
            const roots = (this.spans || []).filter((s) => !s.parent_span_id);
            if (!roots.length) return null;
            roots.sort((a, b) => Number(a.start_time_unix_nano) - Number(b.start_time_unix_nano));
            return roots[0] || null;
        },
        traceErrorCount() {
            return (this.spans || []).filter((s) => String(s.status_code || '').toUpperCase() === 'ERROR').length;
        },
        parseAttrs(jsonStr) {
            if (!jsonStr) return null;
            try {
                return JSON.parse(jsonStr);
            } catch {
                return null;
            }
        },
        attrValue(sp, key) {
            const o = this.parseAttrs(sp?.attributes_json);
            if (!o) return '';
            const v = o[key];
            if (v === undefined || v === null) return '';
            return String(v);
        },
        spanServiceName(sp) {
            return this.attrValue(sp, 'service.name');
        },
        spanHTTPMethod(sp) {
            return this.attrValue(sp, 'http.method') || this.attrValue(sp, 'http.request.method');
        },
        spanHTTPRoute(sp) {
            return (
                this.attrValue(sp, 'http.route') ||
                this.attrValue(sp, 'http.target') ||
                this.attrValue(sp, 'url.path')
            );
        },
        spanHTTPStatus(sp) {
            return this.attrValue(sp, 'http.status_code') || this.attrValue(sp, 'http.response.status_code');
        },
        spanMatchesFilters(sp) {
            const q = String(this.spanQuery || '').trim().toLowerCase();
            const st = String(this.spanStatusFilter || 'all').toLowerCase();
            const kd = String(this.spanKindFilter || 'all').toLowerCase();

            if (st !== 'all') {
                const code = String(sp.status_code || '').toLowerCase();
                if (code !== st) return false;
            }
            if (kd !== 'all') {
                const kind = String(sp.kind || '').toLowerCase();
                if (kind !== kd) return false;
            }
            if (this.spanShowErrorsOnly) {
                if (String(sp.status_code || '').toUpperCase() !== 'ERROR') return false;
            }
            if (this.spanShowSlowOnly) {
                const ms = this.spanDurationMs(sp);
                const thr = Number(this.spanSlowThresholdMs);
                if (!Number.isFinite(ms) || !Number.isFinite(thr) || ms < thr) return false;
            }
            if (!q) return true;
            const attrs = String(sp.attributes_json || '').toLowerCase();
            return (
                String(sp.name || '').toLowerCase().includes(q) ||
                String(sp.span_id || '').toLowerCase().includes(q) ||
                String(sp.parent_span_id || '').toLowerCase().includes(q) ||
                attrs.includes(q)
            );
        },
        filteredSpans() {
            const list = (this.spans || []).filter((s) => this.spanMatchesFilters(s));
            if (this.spanSortBy === 'duration') {
                return list.slice().sort((a, b) => {
                    const da = this.spanDurationMs(a);
                    const db = this.spanDurationMs(b);
                    if (!Number.isFinite(da) && !Number.isFinite(db)) return 0;
                    if (!Number.isFinite(da)) return 1;
                    if (!Number.isFinite(db)) return -1;
                    return db - da;
                });
            }
            return list.slice().sort((a, b) => Number(a.start_time_unix_nano) - Number(b.start_time_unix_nano));
        },
        buildSpanTreeRows() {
            const spans = this.filteredSpans();
            if (!spans.length) return [];
            const byID = new Map();
            for (const s of spans) byID.set(s.span_id, s);
            const children = new Map();
            for (const s of spans) {
                const pid = s.parent_span_id || '';
                if (!children.has(pid)) children.set(pid, []);
                children.get(pid).push(s);
            }
            for (const [k, arr] of children.entries()) {
                arr.sort((a, b) => Number(a.start_time_unix_nano) - Number(b.start_time_unix_nano));
                children.set(k, arr);
            }
            const roots = [];
            for (const s of spans) {
                const pid = s.parent_span_id || '';
                if (!pid || !byID.has(pid)) roots.push(s);
            }
            roots.sort((a, b) => Number(a.start_time_unix_nano) - Number(b.start_time_unix_nano));
            const out = [];
            const dfs = (s, depth) => {
                out.push({ s, depth });
                const kids = children.get(s.span_id) || [];
                for (const c of kids) dfs(c, depth + 1);
            };
            for (const r of roots) dfs(r, 0);
            return out;
        },
        spanStatusBadgeClass(code) {
            const c = String(code || '').toUpperCase();
            if (c === 'ERROR') return 'badge-error';
            if (c === 'OK') return 'badge-success';
            if (c === 'UNSET') return 'badge-ghost';
            return 'badge-ghost';
        },
        spanBarClass(code) {
            const c = String(code || '').toUpperCase();
            if (c === 'ERROR') return 'bg-rose-500/85 border-rose-300/30';
            return 'bg-violet-600/85 border-violet-400/30';
        },
        focusSpan(spanId) {
            this.focusedSpanId = spanId;
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
                const data = await apiClient.getColdChapterHits(q, this.coldMaxResults);
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
                        <p class="text-[10px] font-mono text-slate-500 mt-0.5">GET /bff/v1/cold/chapter_hits</p>
                    </div>
                    <p class="text-sm text-slate-500 leading-relaxed max-w-3xl">
                        Direct hybrid search (Bleve BM25 + HNSW vector) over <strong class="text-slate-400">cold</strong> document chapters.
                        Same endpoint as <code class="text-cyan-600/90">/api/v1/cold/chapter_hits</code>. Use this to verify hits, scores, and
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
                        <div class="flex items-center gap-2">
                            <input
                                v-model="traceListQuery"
                                type="text"
                                class="input input-bordered input-sm bg-slate-950 border-slate-700 w-56"
                                placeholder="Search trace id / root span…"
                                autocomplete="off"
                            />
                            <button type="button" class="btn btn-sm btn-outline border-slate-600" :disabled="tracesLoading" @click="loadTraces">Refresh</button>
                        </div>
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
                                    <th>Started</th>
                                    <th>Duration</th>
                                    <th></th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr v-for="t in filteredTraces()" :key="t.trace_id" class="hover:bg-slate-800/40">
                                    <td class="font-mono text-xs text-cyan-300/90">
                                        <span :title="t.trace_id">{{ String(t.trace_id || '').slice(0, 16) }}…</span>
                                    </td>
                                    <td class="text-sm text-slate-300">{{ t.root_span_name || '—' }}</td>
                                    <td class="text-slate-400">{{ t.span_count }}</td>
                                    <td class="text-slate-400 text-xs whitespace-nowrap">{{ formatUnixNanoAsLocal(t.started_at_unix_nano) }}</td>
                                    <td class="text-slate-400 text-xs">{{ formatNanoRange(t.started_at_unix_nano, t.ended_at_unix_nano) }}</td>
                                    <td>
                                        <button type="button" class="btn btn-ghost btn-xs" @click="openTrace(t.trace_id)">Open</button>
                                    </td>
                                </tr>
                                <tr v-if="!traces.length">
                                    <td colspan="6" class="text-slate-500 text-sm">No traces yet. Run a progressive search with “Trace sample” enabled (or rely on sampling).</td>
                                </tr>
                            </tbody>
                        </table>
                    </div>

                    <div v-if="selectedTraceId" class="card bg-slate-900/50 border border-slate-800">
                        <div class="card-body">
                            <div class="flex flex-col gap-3">
                                <div class="flex flex-wrap items-start justify-between gap-3">
                                    <div class="min-w-0">
                                        <h3 class="text-base font-semibold text-slate-200">
                                            Trace
                                            <span class="font-mono text-cyan-300/90 text-sm break-all">{{ selectedTraceId }}</span>
                                        </h3>
                                        <p class="text-xs text-slate-500 mt-1">
                                            Root span:
                                            <span class="text-slate-300">{{ traceRootSpan()?.name || '—' }}</span>
                                            <span v-if="spanServiceName(traceRootSpan())" class="ml-2 text-slate-600">service.name={{ spanServiceName(traceRootSpan()) }}</span>
                                        </p>
                                    </div>
                                    <div class="flex items-center gap-2">
                                        <span class="badge badge-outline text-slate-300">{{ spans.length }} spans</span>
                                        <span v-if="traceErrorCount() > 0" class="badge badge-error">{{ traceErrorCount() }} errors</span>
                                    </div>
                                </div>
                                <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                                    <div class="rounded-lg border border-slate-800 bg-slate-950/40 p-3">
                                        <div class="text-[11px] text-slate-500">Started</div>
                                        <div class="text-xs text-slate-200 mt-1 font-mono">{{ formatUnixNanoAsLocal(traceRootSpan()?.start_time_unix_nano || 0) }}</div>
                                    </div>
                                    <div class="rounded-lg border border-slate-800 bg-slate-950/40 p-3">
                                        <div class="text-[11px] text-slate-500">Total duration</div>
                                        <div class="text-xs text-slate-200 mt-1 font-mono">{{ formatNanoRange(traceRootSpan()?.start_time_unix_nano || 0, traceRootSpan()?.end_time_unix_nano || 0) }}</div>
                                    </div>
                                    <div class="rounded-lg border border-slate-800 bg-slate-950/40 p-3">
                                        <div class="text-[11px] text-slate-500">Span filters</div>
                                        <div class="mt-1 flex flex-wrap gap-2">
                                            <input v-model="spanQuery" type="text" class="input input-bordered input-xs bg-slate-950 border-slate-700 w-44" placeholder="Search spans…" />
                                            <label class="flex items-center gap-1.5 text-[11px] text-slate-400 select-none">
                                                <input type="checkbox" class="checkbox checkbox-xs checkbox-primary" v-model="spanShowErrorsOnly" />
                                                Errors only
                                            </label>
                                            <label class="flex items-center gap-1.5 text-[11px] text-slate-400 select-none">
                                                <input type="checkbox" class="checkbox checkbox-xs checkbox-primary" v-model="spanShowSlowOnly" />
                                                Slow ≥
                                            </label>
                                            <input v-if="spanShowSlowOnly" v-model.number="spanSlowThresholdMs" type="number" min="0" class="input input-bordered input-xs bg-slate-950 border-slate-700 w-20" />
                                            <select v-model="spanStatusFilter" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                                <option value="all">status: all</option>
                                                <option value="ok">ok</option>
                                                <option value="error">error</option>
                                                <option value="unset">unset</option>
                                            </select>
                                            <select v-model="spanKindFilter" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                                <option value="all">kind: all</option>
                                                <option value="server">server</option>
                                                <option value="client">client</option>
                                                <option value="internal">internal</option>
                                            </select>
                                            <select v-model="spanSortBy" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                                <option value="start">sort: start</option>
                                                <option value="duration">sort: duration</option>
                                            </select>
                                            <select v-model="spanView" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                                <option value="tree">view: tree</option>
                                                <option value="table">view: table</option>
                                            </select>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <p v-if="spansError" class="text-sm text-red-400">{{ spansError }}</p>
                            <div v-else-if="spansLoading" class="text-slate-500 text-sm py-4">Loading spans…</div>
                            <div v-else class="space-y-4 mt-2">
                                <div class="space-y-1">
                                    <div v-for="(row, idx) in traceWaterfallRows()" :key="idx" class="flex items-center gap-2 min-h-[26px]">
                                        <div class="w-8 text-[10px] text-slate-500 text-right shrink-0">{{ row.depth }}</div>
                                        <div class="flex-1 relative h-6 bg-slate-800/60 rounded overflow-hidden min-w-0">
                                            <div
                                                class="absolute top-1 bottom-1 rounded border min-w-[2px] cursor-pointer"
                                                :class="[spanBarClass(row.s.status_code), focusedSpanId === row.s.span_id ? 'ring-2 ring-cyan-300/40' : '']"
                                                :title="row.s.name + ' — ' + formatNanoRange(row.s.start_time_unix_nano, row.s.end_time_unix_nano)"
                                                :style="{ left: row.leftPct + '%', width: row.widthPct + '%' }"
                                                @click="focusSpan(row.s.span_id)"
                                            ></div>
                                        </div>
                                        <div class="w-40 lg:w-52 truncate text-[11px] text-slate-400 shrink-0" :title="row.s.name">{{ row.s.name }}</div>
                                    </div>
                                </div>
                                <div v-if="spanView === 'tree'" class="rounded border border-slate-800 overflow-hidden">
                                    <div class="max-h-96 overflow-y-auto">
                                        <table class="table table-xs">
                                            <thead>
                                                <tr class="text-slate-400 bg-slate-900/60">
                                                    <th>Name</th>
                                                    <th>Kind</th>
                                                    <th>Status</th>
                                                    <th>Duration</th>
                                                    <th>Service</th>
                                                    <th>HTTP</th>
                                                    <th>Attributes</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                <tr
                                                    v-for="row in buildSpanTreeRows()"
                                                    :key="row.s.span_id"
                                                    :class="focusedSpanId === row.s.span_id ? 'bg-slate-800/40' : ''"
                                                    class="hover:bg-slate-800/30"
                                                >
                                                    <td class="text-xs text-slate-200 min-w-[18rem]">
                                                        <div class="flex items-center gap-2 min-w-0">
                                                            <span class="text-slate-600 shrink-0" :style="{ width: (row.depth * 12) + 'px' }"></span>
                                                            <span v-if="row.depth" class="text-slate-700 shrink-0">↳</span>
                                                            <button type="button" class="link link-hover text-slate-200 font-mono truncate" @click="focusSpan(row.s.span_id)" :title="row.s.name">{{ row.s.name }}</button>
                                                        </div>
                                                    </td>
                                                    <td class="text-xs text-slate-500">{{ row.s.kind }}</td>
                                                    <td class="text-xs">
                                                        <span class="badge badge-xs" :class="spanStatusBadgeClass(row.s.status_code)">{{ row.s.status_code || '—' }}</span>
                                                    </td>
                                                    <td class="text-xs text-slate-400 font-mono whitespace-nowrap">{{ formatNanoRange(row.s.start_time_unix_nano, row.s.end_time_unix_nano) }}</td>
                                                    <td class="text-[10px] text-slate-500 font-mono max-w-[10rem] truncate" :title="spanServiceName(row.s)">{{ spanServiceName(row.s) || '—' }}</td>
                                                    <td class="text-[10px] text-slate-500 font-mono max-w-[14rem] truncate" :title="spanHTTPMethod(row.s) + ' ' + spanHTTPRoute(row.s)">
                                                        <span v-if="spanHTTPMethod(row.s) || spanHTTPRoute(row.s)">
                                                            {{ spanHTTPMethod(row.s) || '—' }} {{ spanHTTPRoute(row.s) || '' }}
                                                            <span v-if="spanHTTPStatus(row.s)" class="text-slate-600">({{ spanHTTPStatus(row.s) }})</span>
                                                        </span>
                                                        <span v-else>—</span>
                                                    </td>
                                                    <td class="max-w-xs lg:max-w-md">
                                                        <div class="flex items-start gap-2 min-w-0">
                                                            <p class="text-[10px] text-slate-500 truncate flex-1 min-w-0 m-0" :title="spanAttrsPreview(row.s.attributes_json)">{{ spanAttrsPreview(row.s.attributes_json) || '—' }}</p>
                                                            <button type="button" class="btn btn-ghost btn-xs shrink-0 text-violet-300/90 hover:text-violet-200" @click="openAttrsModal(row.s)">Table</button>
                                                        </div>
                                                    </td>
                                                </tr>
                                                <tr v-if="!buildSpanTreeRows().length">
                                                    <td colspan="7" class="text-slate-500 text-sm">No spans match the filters.</td>
                                                </tr>
                                            </tbody>
                                        </table>
                                    </div>
                                </div>

                                <div v-else class="overflow-x-auto max-h-96 overflow-y-auto rounded border border-slate-800">
                                    <table class="table table-xs">
                                        <thead>
                                            <tr class="text-slate-400">
                                                <th>Name</th>
                                                <th>Kind</th>
                                                <th>Status</th>
                                                <th>Duration</th>
                                                <th>Service</th>
                                                <th>HTTP</th>
                                                <th>Span</th>
                                                <th>Attributes</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <tr
                                                v-for="sp in filteredSpans()"
                                                :key="sp.span_id"
                                                :class="focusedSpanId === sp.span_id ? 'bg-slate-800/40' : ''"
                                                class="hover:bg-slate-800/30"
                                            >
                                                <td class="font-mono text-xs text-slate-200">
                                                    <button type="button" class="link link-hover text-slate-200" @click="focusSpan(sp.span_id)" :title="sp.name">{{ sp.name }}</button>
                                                </td>
                                                <td class="text-xs text-slate-500">{{ sp.kind }}</td>
                                                <td class="text-xs">
                                                    <span class="badge badge-xs" :class="spanStatusBadgeClass(sp.status_code)">{{ sp.status_code || '—' }}</span>
                                                </td>
                                                <td class="text-xs text-slate-400 font-mono">{{ formatNanoRange(sp.start_time_unix_nano, sp.end_time_unix_nano) }}</td>
                                                <td class="text-[10px] text-slate-500 font-mono max-w-[10rem] truncate" :title="spanServiceName(sp)">{{ spanServiceName(sp) || '—' }}</td>
                                                <td class="text-[10px] text-slate-500 font-mono max-w-[14rem] truncate" :title="spanHTTPMethod(sp) + ' ' + spanHTTPRoute(sp)">
                                                    <span v-if="spanHTTPMethod(sp) || spanHTTPRoute(sp)">
                                                        {{ spanHTTPMethod(sp) || '—' }} {{ spanHTTPRoute(sp) || '' }}
                                                        <span v-if="spanHTTPStatus(sp)" class="text-slate-600">({{ spanHTTPStatus(sp) }})</span>
                                                    </span>
                                                    <span v-else>—</span>
                                                </td>
                                                <td class="text-[10px] font-mono text-slate-500">
                                                    <div :title="sp.span_id">id {{ String(sp.span_id || '').slice(0, 8) }}…</div>
                                                    <div v-if="sp.parent_span_id" :title="sp.parent_span_id">parent {{ String(sp.parent_span_id).slice(0, 8) }}…</div>
                                                </td>
                                                <td class="max-w-xs lg:max-w-md">
                                                    <div class="flex items-start gap-2 min-w-0">
                                                        <p class="text-[10px] text-slate-500 truncate flex-1 min-w-0 m-0" :title="spanAttrsPreview(sp.attributes_json)">{{ spanAttrsPreview(sp.attributes_json) || '—' }}</p>
                                                        <button type="button" class="btn btn-ghost btn-xs shrink-0 text-violet-300/90 hover:text-violet-200" @click="openAttrsModal(sp)">Table</button>
                                                    </div>
                                                </td>
                                            </tr>
                                            <tr v-if="!filteredSpans().length">
                                                <td colspan="8" class="text-slate-500 text-sm">No spans match the filters.</td>
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
