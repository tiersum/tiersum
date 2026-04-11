import { apiClient } from '../api_client.js';
import { parseMarkdownOrError } from '../markdown.js';

export const DocumentDetailPage = {
    props: {
        id: { type: String, required: true }
    },
    data() {
        return {
            doc: null,
            summaries: [],
            chapters: [],
            loading: true,
            loadError: null,
            viewMode: 'summary',
            selectedNav: 'overview'
        };
    },
    computed: {
        docSummaryRecord() {
            return this.summaries.find(s => s.tier === 'document');
        },
        docSummaryText() {
            return (this.docSummaryRecord?.content || '').trim();
        },
        /** True when chapter nav has more than a single implicit placeholder (or has overview). */
        hasChapterSidebar() {
            return this.docSummaryText.length > 0 || (this.chapters && this.chapters.length > 0);
        },
        activeChapter() {
            if (this.selectedNav === 'overview') return null;
            return this.chapters.find(c => c.path === this.selectedNav) || null;
        },
        summaryBodyMarkdown() {
            if (this.selectedNav === 'overview') {
                return this.docSummaryText || '_No document-level summary._';
            }
            const ch = this.activeChapter;
            return (ch?.summary || '').trim() || '_No content for this section._';
        }
    },
    watch: {
        id: {
            immediate: true,
            handler() {
                this.load();
            }
        },
        '$route.query.path'() {
            if (this.doc && !this.loading) {
                this.applyRouteChapterSelection();
            }
        }
    },
    methods: {
        async load() {
            this.loading = true;
            this.loadError = null;
            this.doc = null;
            this.summaries = [];
            this.chapters = [];
            try {
                const docId = this.id;
                const [doc, summaries, chapters] = await Promise.all([
                    apiClient.getDocument(docId),
                    apiClient.getDocumentSummaries(docId).catch(() => []),
                    apiClient.getDocumentChapters(docId).catch(() => [])
                ]);
                this.doc = doc;
                this.summaries = summaries;
                this.chapters = chapters;
                this.applyDefaultView();
                this.applyRouteChapterSelection();
            } catch (e) {
                this.loadError = e.message || String(e);
            } finally {
                this.loading = false;
            }
        },
        applyDefaultView() {
            this.viewMode = 'summary';
        },

        /** Pick section from `?path=` (full chapter path as in search / API) or sensible default. */
        applyRouteChapterSelection() {
            const raw = this.$route.query.path;
            if (raw === undefined || raw === null || raw === '') {
                this.setDefaultChapterNav();
                return;
            }
            const want = (Array.isArray(raw) ? raw[0] : String(raw)).trim();
            if (!want) {
                this.setDefaultChapterNav();
                return;
            }
            let decoded = want;
            try {
                decoded = decodeURIComponent(want);
            } catch {
                /* keep want */
            }
            decoded = decoded.trim();
            this.viewMode = 'summary';

            if (decoded === 'overview') {
                if (this.docSummaryText) {
                    this.selectedNav = 'overview';
                } else {
                    this.setDefaultChapterNav();
                }
                return;
            }

            const norm = (p) => String(p || '').replace(/\\/g, '/').trim();
            const dWant = norm(decoded);

            const exact = this.chapters.find((c) => norm(c.path) === dWant);
            if (exact) {
                this.selectedNav = exact.path;
                return;
            }

            const docId = this.id || '';
            if (docId && dWant.startsWith(`${docId}/`)) {
                const rel = dWant.slice(docId.length + 1);
                const byRel = this.chapters.find((c) => {
                    const p = norm(c.path);
                    const suffix = p.startsWith(`${docId}/`) ? p.slice(docId.length + 1) : p;
                    return suffix === rel;
                });
                if (byRel) {
                    this.selectedNav = byRel.path;
                    return;
                }
            }

            const wantLast = dWant.split('/').filter(Boolean).pop();
            if (wantLast) {
                const byLast = this.chapters.find((c) => {
                    const p = norm(c.path);
                    const last = p.split('/').filter(Boolean).pop();
                    return last === wantLast || norm(c.title) === wantLast;
                });
                if (byLast) {
                    this.selectedNav = byLast.path;
                    return;
                }
            }

            this.setDefaultChapterNav();
        },

        setDefaultChapterNav() {
            if (this.docSummaryText) {
                this.selectedNav = 'overview';
            } else if (this.chapters.length) {
                this.selectedNav = this.chapters[0].path;
            } else {
                this.selectedNav = 'overview';
            }
        },
        renderMd(text) {
            return parseMarkdownOrError(text, '<p class="text-red-400">Failed to render markdown.</p>');
        },
        selectNav(key) {
            this.selectedNav = key;
        },
        setViewMode(mode) {
            this.viewMode = mode;
        },
        goBack() {
            this.$router.push('/docs');
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <button type="button" @click="goBack" class="btn btn-ghost btn-sm text-slate-400 mb-6 gap-2">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
                    </svg>
                    Back to documents
                </button>

                <div v-if="loading" class="space-y-4">
                    <div class="h-10 bg-slate-800 rounded animate-pulse w-1/2"></div>
                    <div class="h-96 bg-slate-900/80 border border-slate-800 rounded-xl animate-pulse"></div>
                </div>

                <div v-else-if="loadError" class="alert alert-error bg-red-950/50 border-red-900 text-red-200">
                    <span>Failed to load document: {{ loadError }}</span>
                </div>

                <div v-else-if="doc">
                    <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4 mb-6">
                        <div class="min-w-0">
                            <h1 class="text-2xl sm:text-3xl font-bold text-slate-100 mb-2 break-words">{{ doc.title }}</h1>
                            <div class="flex flex-wrap items-center gap-2 text-sm">
                                <span class="badge badge-outline badge-sm">{{ doc.format }}</span>
                                <span :class="['badge badge-sm', doc.status === 'hot' ? 'badge-warning' : doc.status === 'cold' ? 'badge-info' : 'badge-ghost']">
                                    {{ doc.status }}
                                </span>
                                <span v-if="doc.tags?.length" class="text-slate-500">{{ doc.tags.join(', ') }}</span>
                            </div>
                        </div>
                        <div class="shrink-0 join">
                            <button type="button"
                                class="btn btn-sm join-item"
                                :class="viewMode === 'summary' ? 'btn-primary' : 'btn-ghost border border-slate-700'"
                                @click="setViewMode('summary')">
                                Chapters
                            </button>
                            <button type="button"
                                class="btn btn-sm join-item"
                                :class="viewMode === 'source' ? 'btn-primary' : 'btn-ghost border border-slate-700'"
                                @click="setViewMode('source')">
                                Original
                            </button>
                        </div>
                    </div>

                    <div v-if="viewMode === 'summary'" class="grid grid-cols-1 lg:grid-cols-12 gap-6">
                        <aside v-if="hasChapterSidebar" class="lg:col-span-3">
                            <div class="card bg-slate-900/50 border border-slate-800 lg:sticky lg:top-24">
                                <div class="card-body p-4">
                                    <h2 class="text-sm font-semibold text-slate-400 uppercase tracking-wide mb-3">Sections</h2>
                                    <nav class="flex flex-col gap-1">
                                        <button
                                            v-if="docSummaryText"
                                            type="button"
                                            @click="selectNav('overview')"
                                            :class="['text-left px-3 py-2 rounded-lg text-sm transition-colors',
                                                selectedNav === 'overview' ? 'bg-blue-500/20 text-blue-300 border border-blue-500/40' : 'text-slate-300 hover:bg-slate-800 border border-transparent']">
                                            Overview
                                        </button>
                                        <button
                                            v-for="ch in chapters"
                                            :key="ch.path"
                                            type="button"
                                            @click="selectNav(ch.path)"
                                            :class="['text-left px-3 py-2 rounded-lg text-sm transition-colors break-words',
                                                selectedNav === ch.path ? 'bg-blue-500/20 text-blue-300 border border-blue-500/40' : 'text-slate-300 hover:bg-slate-800 border border-transparent']">
                                            {{ ch.title || ch.path }}
                                        </button>
                                    </nav>
                                </div>
                            </div>
                        </aside>
                        <div :class="hasChapterSidebar ? 'lg:col-span-9' : 'lg:col-span-12'">
                            <div class="card bg-slate-900/50 border border-slate-800 min-h-[320px]">
                                <div class="card-body">
                                    <h2 class="text-lg font-semibold text-slate-200 mb-2">
                                        {{ selectedNav === 'overview' ? 'Document summary' : (activeChapter?.title || 'Section') }}
                                    </h2>
                                    <div class="prose prose-invert max-w-none prose-headings:text-slate-100 prose-p:text-slate-300 border-t border-slate-800 pt-4">
                                        <div v-html="renderMd(summaryBodyMarkdown)"></div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div v-else class="card bg-slate-900/50 border border-slate-800">
                        <div class="card-body">
                            <h2 class="text-lg font-semibold text-slate-200 mb-4">Original</h2>
                            <div class="prose prose-invert max-w-none prose-headings:text-slate-100 prose-p:text-slate-300 border-t border-slate-800 pt-4">
                                <div v-html="renderMd(doc.content || '')"></div>
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    `
};
