import { apiClient } from '../api_client.js';

export const TagsPage = {
    data() {
        return {
            tagGroups: [],
            tags: [],
            loading: true,
            tagsPanelLoading: false,
            selectedGroup: null
        };
    },
    async mounted() {
        await this.loadData();
    },
    methods: {
        /** Pick first L1 group that has an id (required for GET /tags?group_ids=). */
        pickDefaultGroup() {
            const list = Array.isArray(this.tagGroups) ? this.tagGroups : [];
            const first = list.find((g) => g && (g.id != null && String(g.id).trim() !== ''));
            return first || null;
        },
        async loadTagsForSelectedGroup() {
            if (!this.selectedGroup || this.selectedGroup.id == null || String(this.selectedGroup.id).trim() === '') {
                this.tags = [];
                return;
            }
            this.tagsPanelLoading = true;
            try {
                this.tags = await apiClient.getTags({
                    group_ids: [String(this.selectedGroup.id)],
                    max_results: 5000
                });
            } catch (e) {
                console.error('Failed to load tags for group:', e);
                this.tags = [];
            } finally {
                this.tagsPanelLoading = false;
            }
        },
        selectGroup(group) {
            this.selectedGroup = group;
            return this.loadTagsForSelectedGroup();
        },
        async loadData() {
            try {
                this.loading = true;
                const prevId = this.selectedGroup && this.selectedGroup.id != null ? String(this.selectedGroup.id) : '';
                const raw = await apiClient.getTagGroups();
                this.tagGroups = Array.isArray(raw) ? raw : [];
                const still = prevId && this.tagGroups.find((g) => g && String(g.id) === prevId);
                this.selectedGroup = still || this.pickDefaultGroup();
                await this.loadTagsForSelectedGroup();
            } catch (error) {
                console.error('Failed to load tags:', error);
                this.tagGroups = [];
                this.tags = [];
            } finally {
                this.loading = false;
            }
        },
        async triggerGrouping() {
            try {
                await apiClient.triggerTagGrouping();
                alert('Tag grouping triggered successfully');
                await this.loadData();
            } catch (error) {
                console.error('Failed to trigger grouping:', error);
                alert('Failed to trigger grouping: ' + error.message);
            }
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div class="mb-8">
                    <div class="flex justify-between items-start">
                        <div>
                            <h1 class="text-3xl font-bold text-slate-100 mb-2">Tag Browser</h1>
                            <p class="text-slate-400">Browse documents by hierarchical tags. L1 groups organize L2 tags into categories.</p>
                        </div>
                        <button @click="triggerGrouping" class="btn btn-outline btn-sm">
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                            </svg>
                            Regroup Tags
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
                                        L1 Groups
                                    </h2>
                                </div>
                                <div class="p-4 overflow-y-auto h-[calc(100%-80px)] space-y-2">
                                    <div v-if="loading" class="space-y-2">
                                        <div v-for="i in 5" :key="i" class="h-16 bg-slate-800 rounded animate-pulse"></div>
                                    </div>
                                    <div v-else-if="tagGroups.length === 0" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        <p>No tag groups found</p>
                                    </div>
                                    <div v-else>
                                        <button
                                            v-for="group in tagGroups"
                                            :key="group.id"
                                            type="button"
                                            @click="selectGroup(group)"
                                            :class="['w-full text-left p-4 rounded-lg transition-all border',
                                                     selectedGroup?.id === group.id
                                                        ? 'bg-blue-500/10 border-blue-500/50'
                                                        : 'bg-slate-800/30 border-transparent hover:bg-slate-800/60 hover:border-slate-700']"
                                        >
                                            <div class="flex items-center justify-between">
                                                <div>
                                                    <h3 :class="['font-medium', selectedGroup?.id === group.id ? 'text-blue-400' : 'text-slate-200']">
                                                        {{ group.name }}
                                                    </h3>
                                                    <p v-if="group.description" class="text-sm text-slate-500 mt-1 line-clamp-1">
                                                        {{ group.description }}
                                                    </p>
                                                </div>
                                                <span class="badge badge-ghost">{{ group.tags?.length || 0 }}</span>
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
                                        <h2 class="text-lg font-semibold text-slate-100">L2 Tags</h2>
                                    </div>
                                    <span v-if="selectedGroup" class="badge badge-outline badge-primary">{{ selectedGroup.name }}</span>
                                </div>
                                <div class="p-4 overflow-y-auto h-[calc(100%-80px)]">
                                    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                        <div v-for="i in 6" :key="i" class="h-20 bg-slate-800 rounded animate-pulse"></div>
                                    </div>
                                    <div v-else-if="tagGroups.length === 0" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        <p>No L1 groups yet. Ingest tagged documents or use <strong>Regroup Tags</strong>.</p>
                                    </div>
                                    <div v-else-if="!selectedGroup" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        <p>Select an L1 group on the left to load L2 tags from the server.</p>
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
                                            <p>No tags in this group</p>
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
