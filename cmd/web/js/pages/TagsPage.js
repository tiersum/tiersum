import { apiClient, isBrowserViewerRole } from '../api_client.js';

export const TagsPage = {
    data() {
        return {
            topics: [],
            tags: [],
            loading: true,
            tagsPanelLoading: false,
            selectedTopic: null,
            profile: null
        };
    },
    computed: {
        isViewer() {
            return isBrowserViewerRole(this.profile?.role);
        }
    },
    async mounted() {
        await this.loadData();
    },
    methods: {
        /** Pick first topic that has an id (required for GET /tags?topic_ids=). */
        pickDefaultTopic() {
            const list = Array.isArray(this.topics) ? this.topics : [];
            const first = list.find((g) => g && (g.id != null && String(g.id).trim() !== ''));
            return first || null;
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
        selectTopic(topic) {
            this.selectedTopic = topic;
            return this.loadTagsForSelectedTopic();
        },
        async loadData() {
            try {
                try {
                    this.profile = await apiClient.getProfile();
                } catch {
                    this.profile = null;
                }
                this.loading = true;
                const prevId = this.selectedTopic && this.selectedTopic.id != null ? String(this.selectedTopic.id) : '';
                const raw = await apiClient.getTopics();
                this.topics = Array.isArray(raw) ? raw : [];
                const still = prevId && this.topics.find((g) => g && String(g.id) === prevId);
                this.selectedTopic = still || this.pickDefaultTopic();
                await this.loadTagsForSelectedTopic();
            } catch (error) {
                console.error('Failed to load tags:', error);
                this.topics = [];
                this.tags = [];
            } finally {
                this.loading = false;
            }
        },
        async triggerRegroup() {
            try {
                await apiClient.triggerTopicRegroup();
                alert('Topic regrouping completed');
                await this.loadData();
            } catch (error) {
                console.error('Failed to regroup:', error);
                alert('Failed to regroup: ' + error.message);
            }
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div class="mb-8">
                    <div class="flex justify-between items-start">
                        <div>
                            <h1 class="text-3xl font-bold text-slate-100 mb-2">Topics &amp; tags</h1>
                            <p class="text-slate-400">Topics (themes) organize catalog tags. Select a topic to list its tags.</p>
                        </div>
                        <button v-if="!isViewer" @click="triggerRegroup" class="btn btn-outline btn-sm">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                            </svg>
                            Regroup into topics
                        </button>
                    </div>
                </div>

                <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
                    <div class="lg:col-span-4">
                        <div class="card bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
                            <div class="card-body p-0">
                                <div class="p-4 border-b border-slate-800">
                                    <h2 class="text-lg font-semibold text-slate-100 flex items-center gap-2">
                                        <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        Topics
                                    </h2>
                                </div>
                                <div class="p-4 overflow-y-auto h-[calc(100%-80px)] space-y-2">
                                    <div v-if="loading" class="space-y-2">
                                        <div v-for="i in 5" :key="i" class="h-16 bg-slate-800 rounded animate-pulse"></div>
                                    </div>
                                    <div v-else-if="topics.length === 0" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        <p>No topics found</p>
                                    </div>
                                    <div v-else>
                                        <button
                                            v-for="topic in topics"
                                            :key="topic.id"
                                            type="button"
                                            @click="selectTopic(topic)"
                                            :class="['w-full text-left p-4 rounded-lg transition-all border',
                                                     selectedTopic?.id === topic.id
                                                        ? 'bg-blue-500/10 border-blue-500/50'
                                                        : 'bg-slate-800/30 border-transparent hover:bg-slate-800/60 hover:border-slate-700']"
                                        >
                                            <div class="flex items-center justify-between">
                                                <div>
                                                    <h3 :class="['font-medium', selectedTopic?.id === topic.id ? 'text-blue-400' : 'text-slate-200']">
                                                        {{ topic.name }}
                                                    </h3>
                                                    <p v-if="topic.description" class="text-sm text-slate-500 mt-1 line-clamp-1">
                                                        {{ topic.description }}
                                                    </p>
                                                </div>
                                                <span class="badge badge-ghost">{{ topic.tag_names?.length || 0 }}</span>
                                            </div>
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <div class="lg:col-span-8">
                        <div class="card bg-slate-900/50 border-slate-800 h-[calc(100vh-280px)]">
                            <div class="card-body p-0">
                                <div class="p-4 border-b border-slate-800 flex items-center justify-between">
                                    <div class="flex items-center gap-2">
                                        <svg class="w-5 h-5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                                        </svg>
                                        <h2 class="text-lg font-semibold text-slate-100">Tags</h2>
                                    </div>
                                    <span v-if="selectedTopic" class="badge badge-outline badge-primary">{{ selectedTopic.name }}</span>
                                </div>
                                <div class="p-4 overflow-y-auto h-[calc(100%-80px)]">
                                    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                        <div v-for="i in 6" :key="i" class="h-20 bg-slate-800 rounded animate-pulse"></div>
                                    </div>
                                    <div v-else-if="topics.length === 0" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        <p>No topics yet. Ingest tagged documents or use <strong>Regroup into topics</strong>.</p>
                                    </div>
                                    <div v-else-if="!selectedTopic" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        <p>Select a topic on the left to load tags from the server.</p>
                                    </div>
                                    <div v-else-if="tagsPanelLoading" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                        <div v-for="i in 6" :key="'t'+i" class="h-20 bg-slate-800 rounded animate-pulse"></div>
                                    </div>
                                    <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                        <div
                                            v-for="tag in tags"
                                            :key="tag.id"
                                            class="group p-4 rounded-lg bg-slate-800/30 border border-transparent hover:bg-slate-800/60 hover:border-slate-700 transition-all cursor-pointer"
                                        >
                                            <div class="flex items-start justify-between">
                                                <div class="flex items-center gap-3">
                                                    <div class="w-10 h-10 rounded-lg bg-slate-800 flex items-center justify-center group-hover:bg-slate-700 transition-colors">
                                                        <svg class="w-5 h-5 text-slate-500 group-hover:text-emerald-500 transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 20l4-16m2 16l4-16M6 9h14M4 15h14"/>
                                                        </svg>
                                                    </div>
                                                    <div>
                                                        <h3 class="font-medium text-slate-200 group-hover:text-slate-100 transition-colors">{{ tag.name }}</h3>
                                                        <p class="text-sm text-slate-500">{{ tag.document_count }} documents</p>
                                                    </div>
                                                </div>
                                                <svg class="w-5 h-5 text-slate-600 group-hover:text-slate-400 transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
                                                </svg>
                                            </div>
                                        </div>
                                        <div v-if="tags.length === 0" class="col-span-2 text-center py-12 text-slate-500">
                                            <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                                            </svg>
                                            <p>No tags in this topic</p>
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
