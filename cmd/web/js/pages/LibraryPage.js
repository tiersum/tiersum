import { apiClient, isBrowserViewerRole } from '../api_client.js';

/** Browse mode: fixed buckets or a catalog topic (themes from regroup). */
const BROWSE_ALL = 'all';
const BROWSE_COLD = 'cold';
const BROWSE_UNTAGGED = 'untagged';
const BROWSE_TOPIC = 'topic';

export const LibraryPage = {
    data() {
        return {
            profile: null,
            documents: [],
            topics: [],
            tags: [],
            browseMode: BROWSE_ALL,
            selectedTopic: null,
            /** When `browseMode === topic`, filter to this catalog tag name, or null for any tag in the topic. */
            selectedCatalogTagName: null,
            searchQuery: '',
            loading: true,
            tagsPanelLoading: false
        };
    },
    computed: {
        isViewer() {
            return isBrowserViewerRole(this.profile?.role);
        },
        /** Catalog tag names for the selected topic (from `GET /tags?topic_ids=`). */
        catalogTagNameSet() {
            const set = new Set();
            for (const t of this.tags || []) {
                const n = t && t.name != null ? String(t.name) : '';
                if (n.trim() !== '') set.add(n);
            }
            return set;
        },
        topicColumnVisible() {
            return this.browseMode === BROWSE_TOPIC && this.selectedTopic;
        },
        filteredDocs() {
            let list = Array.isArray(this.documents) ? [...this.documents] : [];
            const q = (this.searchQuery || '').trim().toLowerCase();
            if (q) {
                list = list.filter(
                    (doc) =>
                        doc.title?.toLowerCase().includes(q) ||
                        doc.tags?.some((tag) => String(tag).toLowerCase().includes(q))
                );
            }
            if (this.browseMode === BROWSE_COLD) {
                list = list.filter((d) => d.status === 'cold');
            } else if (this.browseMode === BROWSE_UNTAGGED) {
                list = list.filter((d) => !d.tags || d.tags.length === 0);
            } else if (this.browseMode === BROWSE_TOPIC && this.selectedTopic) {
                const set = this.catalogTagNameSet;
                if (this.selectedCatalogTagName) {
                    list = list.filter((d) => d.tags?.includes(this.selectedCatalogTagName));
                } else if (set.size === 0) {
                    list = [];
                } else {
                    list = list.filter((d) => d.tags?.some((dt) => set.has(dt)));
                }
            }
            return list;
        },
        /** Full width on small screens; shares row with nav (+ optional tags) on lg. */
        docColumnClass() {
            return this.topicColumnVisible ? 'col-span-12 lg:col-span-6' : 'col-span-12 lg:col-span-9';
        }
    },
    async mounted() {
        try {
            this.profile = await apiClient.getProfile();
        } catch {
            this.profile = null;
        }
        await this.loadInitial();
    },
    methods: {
        pickDefaultTopic() {
            const list = Array.isArray(this.topics) ? this.topics : [];
            return list.find((g) => g && g.id != null && String(g.id).trim() !== '') || null;
        },
        async loadTagsForSelectedTopic() {
            if (!this.selectedTopic || this.selectedTopic.id == null || String(this.selectedTopic.id).trim() === '') {
                this.tags = [];
                return;
            }
            this.tagsPanelLoading = true;
            try {
                this.tags = await apiClient.getTags({
                    topic_ids: [String(this.selectedTopic.id)],
                    max_results: 5000
                });
            } catch (e) {
                console.error('Failed to load tags for topic:', e);
                this.tags = [];
            } finally {
                this.tagsPanelLoading = false;
            }
        },
        async loadInitial() {
            this.loading = true;
            try {
                const [docs, rawTopics] = await Promise.all([apiClient.getDocuments(), apiClient.getTopics()]);
                this.documents = Array.isArray(docs) ? docs : [];
                this.topics = Array.isArray(rawTopics) ? rawTopics : [];
            } catch (e) {
                console.error('Failed to load library:', e);
                this.documents = [];
                this.topics = [];
            } finally {
                this.loading = false;
            }
        },
        async loadData() {
            try {
                try {
                    this.profile = await apiClient.getProfile();
                } catch {
                    this.profile = null;
                }
                this.loading = true;
                const prevTopicId =
                    this.browseMode === BROWSE_TOPIC && this.selectedTopic && this.selectedTopic.id != null
                        ? String(this.selectedTopic.id)
                        : '';
                const [docs, rawTopics] = await Promise.all([apiClient.getDocuments(), apiClient.getTopics()]);
                this.documents = Array.isArray(docs) ? docs : [];
                this.topics = Array.isArray(rawTopics) ? rawTopics : [];
                if (this.browseMode === BROWSE_TOPIC && prevTopicId) {
                    const still = this.topics.find((g) => g && String(g.id) === prevTopicId);
                    this.selectedCatalogTagName = null;
                    if (still) {
                        this.selectedTopic = still;
                        await this.loadTagsForSelectedTopic();
                    } else {
                        this.selectedTopic = this.pickDefaultTopic();
                        if (this.selectedTopic) await this.loadTagsForSelectedTopic();
                        else {
                            this.tags = [];
                            this.browseMode = BROWSE_ALL;
                        }
                    }
                }
            } catch (e) {
                console.error('Failed to reload library:', e);
            } finally {
                this.loading = false;
            }
        },
        selectBrowseAll() {
            this.browseMode = BROWSE_ALL;
            this.selectedTopic = null;
            this.selectedCatalogTagName = null;
            this.tags = [];
        },
        selectBrowseCold() {
            this.browseMode = BROWSE_COLD;
            this.selectedTopic = null;
            this.selectedCatalogTagName = null;
            this.tags = [];
        },
        selectBrowseUntagged() {
            this.browseMode = BROWSE_UNTAGGED;
            this.selectedTopic = null;
            this.selectedCatalogTagName = null;
            this.tags = [];
        },
        selectTopic(topic) {
            this.browseMode = BROWSE_TOPIC;
            this.selectedTopic = topic;
            this.selectedCatalogTagName = null;
            return this.loadTagsForSelectedTopic();
        },
        selectCatalogTag(tag) {
            const name = tag && tag.name != null ? String(tag.name) : '';
            if (!name) return;
            if (this.selectedCatalogTagName === name) {
                this.selectedCatalogTagName = null;
            } else {
                this.selectedCatalogTagName = name;
            }
        },
        clearCatalogTagFilter() {
            this.selectedCatalogTagName = null;
        },
        async triggerRegroup() {
            try {
                await apiClient.triggerTopicRegroup();
                alert('Topic regrouping completed');
                await this.loadData();
            } catch (error) {
                console.error('Failed to regroup:', error);
                alert('Failed to regroup: ' + (error && error.message ? error.message : String(error)));
            }
        },
        formatDate(dateStr) {
            return new Date(dateStr).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric'
            });
        },
        goToDoc(id) {
            this.$router.push(`/docs/${id}`);
        },
        leftNavBtnClass(active) {
            return [
                'w-full text-left px-4 py-3 rounded-lg transition-all border text-sm',
                active
                    ? 'bg-blue-500/10 border-blue-500/50 text-blue-300'
                    : 'bg-slate-800/30 border-transparent text-slate-200 hover:bg-slate-800/60 hover:border-slate-700'
            ].join(' ');
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4 mb-8">
                    <div>
                        <h1 class="text-3xl font-bold text-slate-100 mb-2">Library</h1>
                        <p class="text-slate-400">
                            Browse by bucket or topic, pick a catalog tag to filter documents, or search by title and tags.
                        </p>
                    </div>
                    <div class="flex flex-wrap items-center gap-2 shrink-0">
                        <button v-if="!isViewer" type="button" @click="triggerRegroup" class="btn btn-outline btn-sm">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                            </svg>
                            Regroup into topics
                        </button>
                        <router-link v-if="!isViewer" to="/docs/new" class="btn btn-primary btn-sm">
                            <svg class="w-5 h-5 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
                            </svg>
                            Add document
                        </router-link>
                    </div>
                </div>

                <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
                    <!-- Left: buckets + topics -->
                    <div class="col-span-12 lg:col-span-3">
                        <div class="card bg-slate-900/50 border-slate-800 min-h-[280px] lg:h-[calc(100vh-280px)]">
                            <div class="card-body p-0 flex flex-col h-full">
                                <div class="p-4 border-b border-slate-800 shrink-0">
                                    <h2 class="text-lg font-semibold text-slate-100 flex items-center gap-2">
                                        <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        Browse
                                    </h2>
                                </div>
                                <div class="p-3 overflow-y-auto flex-1 space-y-1">
                                    <div v-if="loading" class="space-y-2">
                                        <div v-for="i in 6" :key="'sk'+i" class="h-12 bg-slate-800 rounded animate-pulse"></div>
                                    </div>
                                    <template v-else>
                                        <button type="button" :class="leftNavBtnClass(browseMode === 'all')" @click="selectBrowseAll">
                                            All documents
                                        </button>
                                        <button type="button" :class="leftNavBtnClass(browseMode === 'cold')" @click="selectBrowseCold">
                                            Cold
                                        </button>
                                        <button type="button" :class="leftNavBtnClass(browseMode === 'untagged')" @click="selectBrowseUntagged">
                                            Untagged
                                        </button>
                                        <div class="border-t border-slate-800 my-2 pt-2">
                                            <p class="text-[10px] uppercase tracking-wider text-slate-500 px-2 mb-1">Topics</p>
                                            <template v-if="topics.length === 0">
                                                <div class="text-center py-6 text-slate-500 text-sm px-2">
                                                    No topics yet. Ingest tagged documents or use Regroup.
                                                </div>
                                            </template>
                                            <template v-else>
                                                <button
                                                    v-for="topic in topics"
                                                    :key="topic.id"
                                                    type="button"
                                                    :class="leftNavBtnClass(browseMode === 'topic' && selectedTopic?.id === topic.id)"
                                                    @click="selectTopic(topic)"
                                                >
                                                    <div class="flex items-center justify-between gap-2">
                                                        <span class="font-medium line-clamp-2">{{ topic.name }}</span>
                                                        <span class="badge badge-ghost badge-sm shrink-0">{{ topic.tag_names?.length || 0 }}</span>
                                                    </div>
                                                </button>
                                            </template>
                                        </div>
                                    </template>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Middle: catalog tags (topic mode only) -->
                    <div v-if="topicColumnVisible" class="col-span-12 lg:col-span-3">
                        <div class="card bg-slate-900/50 border-slate-800 min-h-[200px] lg:h-[calc(100vh-280px)]">
                            <div class="card-body p-0 flex flex-col h-full">
                                <div class="p-4 border-b border-slate-800 flex items-center justify-between gap-2 shrink-0">
                                    <div class="flex items-center gap-2 min-w-0">
                                        <svg class="w-5 h-5 text-emerald-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                                        </svg>
                                        <h2 class="text-lg font-semibold text-slate-100 truncate">Tags</h2>
                                    </div>
                                    <span class="badge badge-outline badge-primary truncate max-w-[8rem]">{{ selectedTopic.name }}</span>
                                </div>
                                <div class="p-3 overflow-y-auto flex-1 space-y-2">
                                    <button
                                        type="button"
                                        :class="leftNavBtnClass(!selectedCatalogTagName)"
                                        @click="clearCatalogTagFilter"
                                    >
                                        All tags in topic
                                    </button>
                                    <div v-if="tagsPanelLoading" class="space-y-2">
                                        <div v-for="i in 5" :key="'tg'+i" class="h-14 bg-slate-800 rounded animate-pulse"></div>
                                    </div>
                                    <template v-else>
                                        <button
                                            v-for="tag in tags"
                                            :key="tag.id"
                                            type="button"
                                            :class="leftNavBtnClass(selectedCatalogTagName === tag.name)"
                                            @click="selectCatalogTag(tag)"
                                        >
                                            <div class="flex items-center justify-between gap-2">
                                                <span class="font-medium line-clamp-2">{{ tag.name }}</span>
                                                <span class="text-xs text-slate-500 shrink-0">{{ tag.document_count }}</span>
                                            </div>
                                        </button>
                                        <p v-if="tags.length === 0" class="text-center text-slate-500 text-sm py-6 px-2">
                                            No catalog tags in this topic.
                                        </p>
                                    </template>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Right: documents -->
                    <div :class="docColumnClass">
                        <div class="card bg-slate-900/50 border-slate-800 min-h-[320px] lg:h-[calc(100vh-280px)]">
                            <div class="card-body p-0 flex flex-col h-full">
                                <div class="p-4 border-b border-slate-800 shrink-0">
                                    <div class="relative max-w-md">
                                        <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                                        </svg>
                                        <input
                                            v-model="searchQuery"
                                            placeholder="Search by title or tag text..."
                                            class="w-full pl-10 pr-4 py-2 bg-slate-900/50 border border-slate-800 text-slate-100 placeholder:text-slate-500 focus:border-blue-500/50 focus:ring-2 focus:ring-blue-500/20 rounded-lg outline-none text-sm"
                                        />
                                    </div>
                                    <p class="text-xs text-slate-500 mt-2">
                                        <span v-if="browseMode === 'all'">Showing all documents</span>
                                        <span v-else-if="browseMode === 'cold'">Cold tier only</span>
                                        <span v-else-if="browseMode === 'untagged'">Documents with no tags</span>
                                        <span v-else-if="browseMode === 'topic' && selectedTopic">
                                            Topic “{{ selectedTopic.name }}”
                                            <template v-if="selectedCatalogTagName"> — tag “{{ selectedCatalogTagName }}”</template>
                                        </span>
                                        · {{ filteredDocs.length }} shown
                                    </p>
                                </div>
                                <div class="p-4 overflow-y-auto flex-1">
                                    <div v-if="loading" class="space-y-4">
                                        <div v-for="i in 3" :key="'d'+i" class="rounded-lg border border-slate-800 bg-slate-900/40 p-4 animate-pulse">
                                            <div class="h-5 bg-slate-800 rounded w-1/3 mb-2"></div>
                                            <div class="h-4 bg-slate-800 rounded w-2/3"></div>
                                        </div>
                                    </div>
                                    <div v-else-if="filteredDocs.length === 0" class="text-center py-12">
                                        <svg class="w-16 h-16 mx-auto mb-4 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                                        </svg>
                                        <h3 class="text-xl font-medium text-slate-300 mb-2">No documents match</h3>
                                        <p class="text-slate-500 mb-4 text-sm">Try another bucket, tag, or search.</p>
                                        <router-link v-if="!isViewer" to="/docs/new" class="btn btn-primary btn-sm">Add document</router-link>
                                    </div>
                                    <div v-else class="grid gap-3">
                                        <div
                                            v-for="doc in filteredDocs"
                                            :key="doc.id"
                                            class="rounded-xl border border-slate-800 bg-slate-900/40 hover:border-slate-700 transition-colors cursor-pointer p-4 sm:p-5"
                                            @click="goToDoc(doc.id)"
                                        >
                                            <div class="flex items-start justify-between gap-3">
                                                <div class="flex-1 min-w-0">
                                                    <div class="flex flex-wrap items-center gap-2 mb-2">
                                                        <svg class="w-5 h-5 text-blue-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                                                        </svg>
                                                        <h3 class="text-base font-semibold text-slate-200 truncate">{{ doc.title }}</h3>
                                                        <span class="badge badge-outline badge-sm">{{ doc.format }}</span>
                                                        <span :class="['badge badge-sm', doc.status === 'hot' ? 'badge-warning' : doc.status === 'cold' ? 'badge-info' : 'badge-ghost']">
                                                            {{ doc.status }}
                                                        </span>
                                                    </div>
                                                    <p class="text-slate-500 text-sm mb-2 line-clamp-2">{{ doc.content?.substring(0, 120) }}...</p>
                                                    <div class="flex flex-wrap items-center gap-3 text-xs text-slate-500">
                                                        <span class="inline-flex items-center gap-1">
                                                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
                                                            </svg>
                                                            {{ formatDate(doc.created_at) }}
                                                        </span>
                                                        <span v-if="doc.tags?.length" class="inline-flex items-center gap-1 min-w-0">
                                                            <svg class="w-3.5 h-3.5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                                                            </svg>
                                                            <span class="truncate">{{ doc.tags.join(', ') }}</span>
                                                        </span>
                                                        <span v-else class="text-slate-600">No tags</span>
                                                    </div>
                                                </div>
                                                <div class="text-right shrink-0 flex flex-col items-end gap-2" @click.stop>
                                                    <div class="text-xl font-bold text-slate-200">{{ (doc.hot_score || 0).toFixed(2) }}</div>
                                                    <div class="text-[10px] text-slate-500">hot score</div>
                                                    <button type="button" class="btn btn-xs btn-outline btn-primary" @click="goToDoc(doc.id)">Open</button>
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
