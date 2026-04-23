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
            spanSortBy: 'start',
            spanView: 'tree',
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
                this.coldError = this.$t('observabilityKeywords') + ' (spaces or commas).';
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
        traceTimeBounds() {
            const list = this.spans || [];
            if (!list.length) return { t0: 0, total: 1 };
            let t0 = Infinity;
            let t1 = 0;
            for (const s of list) {
                const st = Number(s.start_time_unix_nano);
                const en = Number(s.end_time_unix_nano);
                if (Number.isFinite(st) && st < t0) t0 = st;
                if (Number.isFinite(en) && en > t1) t1 = en;
            }
            if (!Number.isFinite(t0)) t0 = 0;
            const total = Math.max(1, t1 - t0);
            return { t0, total };
        },
        spanBarMetric(sp) {
            const { t0, total } = this.traceTimeBounds();
            const st = Number(sp.start_time_unix_nano);
            const en = Number(sp.end_time_unix_nano);
            if (!Number.isFinite(st) || !Number.isFinite(en)) {
                return { leftPct: 0, widthPct: 0 };
            }
            const leftPct = Math.max(0, Math.min(100, ((st - t0) / total) * 100));
            const rawW = ((en - st) / total) * 100;
            const widthPct = Math.max(0.12, Math.min(100 - leftPct, Number.isFinite(rawW) ? rawW : 0));
            return { leftPct, widthPct };
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
                            <h3 id="attrs-modal-title" class="text-sm font-semibold text-slate-200">{{ $t('observabilitySpanAttributes') }}</h3>
                            <p v-if="attrsModalSpanName" class="text-xs font-mono text-cyan-300/90 truncate mt-0.5" :title="attrsModalSpanName">{{ attrsModalSpanName }}</p>
                        </div>
                        <button type="button" class="btn btn-sm btn-ghost text-slate-400 shrink-0" @click="closeAttrsModal">{{ $t('close') }}</button>
                    </div>
                    <div class="overflow-auto flex-1 min-h-0 p-3">
                        <p v-if="!attrsModalRows.length" class="text-sm text-slate-500 px-1">{{ $t('observabilityNoAttributes') }}</p>
                        <table v-else class="table table-sm w-full border border-slate-800 rounded-lg overflow-hidden">
                            <thead class="sticky top-0 z-10 bg-slate-800/95 text-slate-300 text-xs uppercase tracking-wide">
                                <tr>
                                    <th class="w-[28%] min-w-[8rem] align-top">{{ $t('observabilityKey') }}</th>
                                    <th class="align-top">{{ $t('observabilityValue') }}</th>
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
            <main class="max-w-[1800px] mx-auto px-4 sm:px-6 lg:px-8 py-6 pb-16">
                <div class="mb-6">
                    <h1 class="text-2xl sm:text-3xl font-bold text-slate-100">{{ $t('observabilityTitle') }}</h1>
                    <p class="text-slate-500 text-sm mt-1">{{ $t('observabilityDescFull') }}</p>
                </div>
                <div role="tablist" class="tabs tabs-boxed bg-slate-900/80 border border-slate-800 mb-6 flex flex-wrap gap-y-1 w-full max-w-full sm:max-w-xl">
                    <a role="tab" :class="['tab', tab === 'monitoring' ? 'tab-active' : '']" @click.prevent="setTab('monitoring')">{{ $t('observabilityTabMonitoring') }}</a>
                    <a role="tab" :class="['tab', tab === 'cold' ? 'tab-active' : '']" @click.prevent="setTab('cold')">{{ $t('observabilityTabCold') }}</a>
                    <a role="tab" :class="['tab', tab === 'traces' ? 'tab-active' : '']" @click.prevent="setTab('traces')">{{ $t('observabilityTabTraces') }}</a>
                </div>

                <div v-if="tab === 'monitoring'">
                    <monitoring-page />
                </div>

                <div v-if="tab === 'cold'" class="space-y-4">
                    <div>
                        <h2 class="text-lg font-semibold text-slate-200">{{ $t('observabilityColdTitle') }}</h2>
                        <p class="text-[10px] font-mono text-slate-500 mt-0.5">GET /bff/v1/cold/chapter_hits</p>
                    </div>
                    <p class="text-sm text-slate-500 leading-relaxed max-w-3xl">
                        {{ $t('observabilityColdDesc') }}
                    </p>
                    <div class="flex flex-wrap items-end gap-2">
                        <label class="form-control min-w-[12rem] flex-1 max-w-xl">
                            <span class="label py-0"><span class="label-text text-xs text-slate-400">{{ $t('observabilityKeywords') }}</span></span>
                            <input
                                v-model="coldQ"
                                type="text"
                                class="input input-bordered input-sm bg-slate-950 border-slate-700 w-full"
                                :placeholder="$t('observabilityKeywords')"
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
                            {{ coldLoading ? $t('observabilityRunning') : $t('observabilityRunProbe') }}
                        </button>
                    </div>
                    <p v-if="coldError" class="text-sm text-red-400 whitespace-pre-wrap">{{ coldError }}</p>
                    <p v-else-if="coldRan && !coldLoading && !coldItems.length" class="text-sm text-slate-500">{{ $t('observabilityNoHits') }}</p>
                    <div v-if="coldItems.length" class="overflow-x-auto rounded-lg border border-slate-800">
                        <table class="table table-sm">
                            <thead>
                                <tr class="bg-slate-900/90 text-slate-400 text-xs">
                                    <th>{{ $t('observabilityTitleCol') }}</th>
                                    <th>{{ $t('observabilityPath') }}</th>
                                    <th>{{ $t('observabilitySource') }}</th>
                                    <th>{{ $t('observabilityScore') }}</th>
                                    <th>{{ $t('observabilityDocument') }}</th>
                                    <th>{{ $t('observabilityContext') }}</th>
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
                                        >{{ $t('open') }}</router-link>
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
                                            {{ coldExpanded[i] ? $t('observabilityShowLess') : $t('observabilityShowFull') }}
                                        </button>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>

                <div v-if="tab === 'traces'" class="rounded-xl border border-slate-800 bg-slate-900/25 overflow-hidden min-h-[calc(100vh-11rem)] flex flex-col lg:flex-row">
                    <!-- Left: trace search -->
                    <aside class="w-full lg:w-[22rem] xl:w-[26rem] shrink-0 border-b lg:border-b-0 lg:border-r border-slate-800 flex flex-col min-h-0 max-h-[42vh] lg:max-h-none lg:min-h-[calc(100vh-12rem)]">
                        <div class="px-3 py-3 border-b border-slate-800 bg-slate-950/40 shrink-0">
                            <h2 class="text-sm font-semibold text-slate-200 tracking-wide uppercase">{{ $t('observabilityTracesTitle') }}</h2>
                            <p class="text-[11px] text-slate-500 mt-0.5">{{ $t('observabilityTracesDesc') }}</p>
                            <div class="flex gap-2 mt-2">
                                <input
                                    v-model="traceListQuery"
                                    type="text"
                                    class="input input-bordered input-sm bg-slate-950 border-slate-700 flex-1 min-w-0"
                                    :placeholder="$t('observabilityTracePlaceholder')"
                                    autocomplete="off"
                                />
                                <button type="button" class="btn btn-sm btn-outline border-slate-600 shrink-0" :disabled="tracesLoading" @click="loadTraces">{{ $t('refresh') }}</button>
                            </div>
                        </div>
                        <p v-if="tracesError" class="text-xs text-red-400 px-3 py-2 shrink-0">{{ tracesError }}</p>
                        <div v-else-if="tracesLoading" class="text-slate-500 text-sm px-3 py-4 shrink-0">{{ $t('loading') }}</div>
                        <div v-else class="flex-1 min-h-0 overflow-y-auto overflow-x-hidden">
                            <table class="table table-sm w-full">
                                <thead class="sticky top-0 z-[1] bg-slate-900/95 shadow-sm">
                                    <tr class="text-slate-400 text-[10px] uppercase tracking-wide">
                                        <th class="py-2">{{ $t('observabilityService') }} / op</th>
                                        <th class="py-2 w-14 text-right">{{ $t('observabilitySpans') }}</th>
                                        <th class="py-2 w-24 text-right">{{ $t('observabilityDuration') }}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr
                                        v-for="t in filteredTraces()"
                                        :key="t.trace_id"
                                        class="cursor-pointer border-b border-slate-800/80 hover:bg-slate-800/35"
                                        :class="selectedTraceId === t.trace_id ? 'bg-slate-800/55 border-l-2 border-l-cyan-400/90' : ''"
                                        @click="openTrace(t.trace_id)"
                                    >
                                        <td class="align-top py-2 min-w-0">
                                            <div class="font-mono text-[11px] text-cyan-300/90 truncate" :title="t.trace_id">{{ String(t.trace_id || '').slice(0, 10) }}…</div>
                                            <div class="text-xs text-slate-300 line-clamp-2 mt-0.5" :title="t.root_span_name || ''">{{ t.root_span_name || '—' }}</div>
                                            <div class="text-[10px] text-slate-500 mt-0.5 whitespace-nowrap">{{ formatUnixNanoAsLocal(t.started_at_unix_nano) }}</div>
                                        </td>
                                        <td class="align-top py-2 text-right text-slate-400 text-xs whitespace-nowrap">{{ t.span_count }}</td>
                                        <td class="align-top py-2 text-right text-slate-400 text-[11px] font-mono whitespace-nowrap">{{ formatNanoRange(t.started_at_unix_nano, t.ended_at_unix_nano) }}</td>
                                    </tr>
                                    <tr v-if="!traces.length">
                                        <td colspan="3" class="text-slate-500 text-sm py-6 px-3">{{ $t('observabilityNoSpans') }}</td>
                                    </tr>
                                </tbody>
                            </table>
                        </div>
                    </aside>

                    <!-- Right: trace timeline + span detail -->
                    <section class="flex-1 min-w-0 min-h-0 flex flex-col bg-slate-950/20">
                        <div v-if="!selectedTraceId" class="flex-1 flex flex-col items-center justify-center text-center px-6 py-16 text-slate-500">
                            <p class="text-sm font-medium text-slate-400">{{ $t('observabilityNoTraceSelected') }}</p>
                            <p class="text-xs mt-2 max-w-sm leading-relaxed">{{ $t('observabilityNoTraceHint') }}</p>
                        </div>
                        <div v-else class="flex flex-col flex-1 min-h-0">
                            <header class="shrink-0 border-b border-slate-800 px-4 py-3 bg-slate-900/40">
                                <div class="flex flex-wrap items-start justify-between gap-3">
                                    <div class="min-w-0 flex-1">
                                        <div class="text-[10px] uppercase tracking-wide text-slate-500 font-semibold">{{ $t('observabilityTraceID') }}</div>
                                        <h3 class="text-sm font-mono text-cyan-300/90 break-all leading-snug mt-0.5">{{ selectedTraceId }}</h3>
                                        <p class="text-xs text-slate-500 mt-1.5">
                                            <span class="text-slate-400">{{ $t('observabilityRoot') }}</span>
                                            <span class="text-slate-200">{{ traceRootSpan()?.name || '—' }}</span>
                                            <span v-if="spanServiceName(traceRootSpan())" class="ml-2 text-slate-600 font-mono text-[11px]">{{ spanServiceName(traceRootSpan()) }}</span>
                                        </p>
                                    </div>
                                    <div class="flex flex-wrap items-center gap-2 shrink-0">
                                        <span class="badge badge-outline badge-sm text-slate-300">{{ spans.length }} {{ $t('observabilitySpans') }}</span>
                                        <span v-if="traceErrorCount() > 0" class="badge badge-error badge-sm">{{ traceErrorCount() }} {{ $t('observabilityErrors') }}</span>
                                        <span class="badge badge-ghost badge-sm text-slate-400 font-mono">{{ formatNanoRange(traceRootSpan()?.start_time_unix_nano || 0, traceRootSpan()?.end_time_unix_nano || 0) }}</span>
                                    </div>
                                </div>
                                <div class="mt-3 flex flex-wrap items-center gap-2">
                                    <input v-model="spanQuery" type="text" class="input input-bordered input-xs bg-slate-950 border-slate-700 w-48 sm:w-56" :placeholder="$t('observabilityFilterSpans')" />
                                    <label class="flex items-center gap-1.5 text-[11px] text-slate-400 select-none">
                                        <input type="checkbox" class="checkbox checkbox-xs checkbox-primary" v-model="spanShowErrorsOnly" />
                                        {{ $t('observabilityErrorsOnly') }}
                                    </label>
                                    <label class="flex items-center gap-1.5 text-[11px] text-slate-400 select-none">
                                        <input type="checkbox" class="checkbox checkbox-xs checkbox-primary" v-model="spanShowSlowOnly" />
                                        {{ $t('observabilitySlowOnly') }}
                                    </label>
                                    <input v-if="spanShowSlowOnly" v-model.number="spanSlowThresholdMs" type="number" min="0" class="input input-bordered input-xs bg-slate-950 border-slate-700 w-16" />
                                    <select v-model="spanStatusFilter" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                        <option value="all">{{ $t('observabilityStatus') }}</option>
                                        <option value="ok">ok</option>
                                        <option value="error">error</option>
                                        <option value="unset">unset</option>
                                    </select>
                                    <select v-model="spanKindFilter" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                        <option value="all">{{ $t('observabilityKind') }}</option>
                                        <option value="server">server</option>
                                        <option value="client">client</option>
                                        <option value="internal">internal</option>
                                    </select>
                                    <select v-model="spanSortBy" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                        <option value="start">{{ $t('observabilitySortStart') }}</option>
                                        <option value="duration">{{ $t('observabilitySortDuration') }}</option>
                                    </select>
                                    <select v-model="spanView" class="select select-bordered select-xs bg-slate-950 border-slate-700">
                                        <option value="tree">{{ $t('observabilityViewTree') }}</option>
                                        <option value="table">{{ $t('observabilityViewTable') }}</option>
                                    </select>
                                </div>
                            </header>

                            <p v-if="spansError" class="text-sm text-red-400 px-4 py-2 shrink-0">{{ spansError }}</p>
                            <div v-else-if="spansLoading" class="text-slate-500 text-sm px-4 py-6 shrink-0">{{ $t('observabilityLoadingSpans') }}</div>
                            <div v-else class="flex-1 min-h-0 flex flex-col gap-0 overflow-hidden">
                                <div v-if="spanView === 'tree'" class="flex-1 min-h-0 flex flex-col mx-3 my-2 mb-3 rounded-lg border border-slate-800 bg-slate-950/30 overflow-hidden">
                                    <div
                                        class="sticky top-0 z-[2] shrink-0 grid gap-x-2 items-center border-b border-slate-800 bg-slate-900/98 px-2 py-2 text-[10px] uppercase tracking-wide text-slate-500 font-semibold min-w-[720px]"
                                        style="grid-template-columns: minmax(11rem, 30%) 3.25rem 4.75rem 4.25rem minmax(12rem, 1fr) 6.5rem;"
                                    >
                                        <div>{{ $t('observabilitySpan') }}</div>
                                        <div>{{ $t('observabilitySpanKind') }}</div>
                                        <div>{{ $t('observabilitySpanStatus') }}</div>
                                        <div class="whitespace-nowrap">{{ $t('observabilityDuration') }}</div>
                                        <div>{{ $t('observabilityWaterfall') }}</div>
                                        <div class="text-right pr-1">{{ $t('observabilityServiceAttrs') }}</div>
                                    </div>
                                    <div class="flex-1 min-h-0 overflow-auto">
                                        <div
                                            v-for="row in buildSpanTreeRows()"
                                            :key="row.s.span_id"
                                            class="grid gap-x-2 items-center border-b border-slate-800/70 px-2 py-1.5 min-h-[32px] min-w-[720px] hover:bg-slate-800/25 cursor-pointer text-xs"
                                            style="grid-template-columns: minmax(11rem, 30%) 3.25rem 4.75rem 4.25rem minmax(12rem, 1fr) 6.5rem;"
                                            :class="focusedSpanId === row.s.span_id ? 'bg-slate-800/45' : ''"
                                            @click="focusSpan(row.s.span_id)"
                                        >
                                            <div class="min-w-0 flex items-center">
                                                <button
                                                    type="button"
                                                    class="link link-hover text-left font-mono text-slate-200 truncate min-w-0"
                                                    :style="{ paddingLeft: (6 + row.depth * 14) + 'px' }"
                                                    :title="row.s.name"
                                                    @click.stop="focusSpan(row.s.span_id)"
                                                >{{ row.s.name }}</button>
                                            </div>
                                            <div class="text-slate-500 truncate text-[11px]" :title="row.s.kind">{{ row.s.kind }}</div>
                                            <div>
                                                <span class="badge badge-xs" :class="spanStatusBadgeClass(row.s.status_code)">{{ row.s.status_code || '—' }}</span>
                                            </div>
                                            <div class="text-slate-400 font-mono text-[11px] whitespace-nowrap">{{ formatNanoRange(row.s.start_time_unix_nano, row.s.end_time_unix_nano) }}</div>
                                            <div class="relative h-5 bg-slate-800/55 rounded overflow-hidden min-w-[6rem]">
                                                <div
                                                    class="absolute top-0.5 bottom-0.5 rounded border min-w-[2px] pointer-events-none"
                                                    :class="[spanBarClass(row.s.status_code), focusedSpanId === row.s.span_id ? 'ring-2 ring-cyan-300/35' : '']"
                                                    :title="row.s.name + ' — ' + formatNanoRange(row.s.start_time_unix_nano, row.s.end_time_unix_nano)"
                                                    :style="{ left: spanBarMetric(row.s).leftPct + '%', width: spanBarMetric(row.s).widthPct + '%' }"
                                                ></div>
                                            </div>
                                            <div class="min-w-0 flex flex-col items-end gap-0.5 text-[10px] text-slate-500">
                                                <span class="font-mono truncate max-w-full text-right" :title="spanServiceName(row.s)">{{ spanServiceName(row.s) || '—' }}</span>
                                                <div class="flex items-center gap-1 shrink-0">
                                                    <span v-if="spanHTTPMethod(row.s) || spanHTTPRoute(row.s)" class="truncate max-w-[5.5rem] text-slate-600 text-right hidden sm:inline" :title="spanHTTPMethod(row.s) + ' ' + spanHTTPRoute(row.s)">{{ spanHTTPMethod(row.s) || '' }} {{ spanHTTPRoute(row.s) || '' }}</span>
                                                    <button type="button" class="btn btn-ghost btn-xs min-h-0 h-6 px-1.5 text-violet-300/90 hover:text-violet-200" @click.stop="openAttrsModal(row.s)">{{ $t('observabilityAttributes') }}</button>
                                                </div>
                                            </div>
                                        </div>
                                        <p v-if="!buildSpanTreeRows().length" class="text-slate-500 text-sm px-3 py-6">{{ $t('observabilityNoSpans') }}</p>
                                    </div>
                                </div>
                                <div v-else class="flex-1 min-h-0 overflow-hidden rounded-lg border border-slate-800 m-3 my-2 mb-3 bg-slate-950/30">
                                    <div class="h-full overflow-auto">
                                        <table class="table table-xs w-full min-w-[900px]">
                                            <thead class="sticky top-0 z-[1] bg-slate-900/95 text-slate-400 text-[10px] uppercase tracking-wide">
                                                <tr>
                                                    <th>{{ $t('observabilitySpan') }}</th>
                                                    <th>{{ $t('observabilitySpanKind') }}</th>
                                                    <th>{{ $t('observabilitySpanStatus') }}</th>
                                                    <th>{{ $t('observabilityDuration') }}</th>
                                                    <th class="min-w-[10rem]">{{ $t('observabilityWaterfall') }}</th>
                                                    <th>{{ $t('observabilityService') }}</th>
                                                    <th>HTTP</th>
                                                    <th>{{ $t('observabilitySpanCol') }}</th>
                                                    <th>{{ $t('observabilityAttributes') }}</th>
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
                                                        <button type="button" class="link link-hover text-slate-200 text-left" @click="focusSpan(sp.span_id)" :title="sp.name">{{ sp.name }}</button>
                                                    </td>
                                                    <td class="text-xs text-slate-500">{{ sp.kind }}</td>
                                                    <td class="text-xs">
                                                        <span class="badge badge-xs" :class="spanStatusBadgeClass(sp.status_code)">{{ sp.status_code || '—' }}</span>
                                                    </td>
                                                    <td class="text-xs text-slate-400 font-mono whitespace-nowrap">{{ formatNanoRange(sp.start_time_unix_nano, sp.end_time_unix_nano) }}</td>
                                                    <td class="align-middle py-2">
                                                        <div class="relative h-5 w-full min-w-[8rem] max-w-[18rem] bg-slate-800/55 rounded overflow-hidden">
                                                            <div
                                                                class="absolute top-0.5 bottom-0.5 rounded border min-w-[2px]"
                                                                :class="[spanBarClass(sp.status_code), focusedSpanId === sp.span_id ? 'ring-2 ring-cyan-300/35' : '']"
                                                                :title="sp.name + ' — ' + formatNanoRange(sp.start_time_unix_nano, sp.end_time_unix_nano)"
                                                                :style="{ left: spanBarMetric(sp).leftPct + '%', width: spanBarMetric(sp).widthPct + '%' }"
                                                            ></div>
                                                        </div>
                                                    </td>
                                                    <td class="text-[10px] text-slate-500 font-mono max-w-[8rem] truncate" :title="spanServiceName(sp)">{{ spanServiceName(sp) || '—' }}</td>
                                                    <td class="text-[10px] text-slate-500 font-mono max-w-[12rem] truncate" :title="spanHTTPMethod(sp) + ' ' + spanHTTPRoute(sp)">
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
                                                    <td class="max-w-[14rem] xl:max-w-md">
                                                        <div class="flex items-start gap-2 min-w-0">
                                                            <p class="text-[10px] text-slate-500 truncate flex-1 min-w-0 m-0" :title="spanAttrsPreview(sp.attributes_json)">{{ spanAttrsPreview(sp.attributes_json) || '—' }}</p>
                                                            <button type="button" class="btn btn-ghost btn-xs shrink-0 text-violet-300/90 hover:text-violet-200" @click.stop="openAttrsModal(sp)">{{ $t('observabilityTable') }}</button>
                                                        </div>
                                                    </td>
                                                </tr>
                                                <tr v-if="!filteredSpans().length">
                                                    <td colspan="9" class="text-slate-500 text-sm">{{ $t('observabilityNoSpans') }}</td>
                                                </tr>
                                            </tbody>
                                        </table>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </section>
                </div>
            </main>
        </div>
    `
};
