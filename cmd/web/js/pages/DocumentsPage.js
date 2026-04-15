import { apiClient, isBrowserViewerRole } from '../api_client.js';

export const DocumentsPage = {
    data() {
        return {
            documents: [],
            loading: true,
            searchQuery: '',
            profile: null
        };
    },
    computed: {
        isViewer() {
            return isBrowserViewerRole(this.profile?.role);
        },
        filteredDocs() {
            if (!this.searchQuery) return this.documents;
            const query = this.searchQuery.toLowerCase();
            return this.documents.filter(doc =>
                doc.title?.toLowerCase().includes(query) ||
                doc.tags?.some(tag => tag.toLowerCase().includes(query))
            );
        }
    },
    async mounted() {
        try {
            this.profile = await apiClient.getProfile();
        } catch {
            this.profile = null;
        }
        await this.loadDocuments();
    },
    methods: {
        async loadDocuments() {
            try {
                this.loading = true;
                this.documents = await apiClient.getDocuments();
            } catch (error) {
                console.error('Failed to load documents:', error);
            } finally {
                this.loading = false;
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
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div class="flex items-center justify-between mb-8">
                    <div>
                        <h1 class="text-3xl font-bold text-slate-100 mb-2">Documents</h1>
                        <p class="text-slate-400">Browse and manage your knowledge base documents. Click a row to open details.</p>
                    </div>
                    <router-link v-if="!isViewer" to="/docs/new" class="btn btn-primary">
                        <svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
                        </svg>
                        Add Document
                    </router-link>
                </div>

                <div class="mb-6">
                    <div class="relative max-w-md">
                        <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                        </svg>
                        <input
                            v-model="searchQuery"
                            placeholder="Search documents..."
                            class="w-full pl-10 pr-4 py-2 bg-slate-900/50 border border-slate-800 text-slate-100 placeholder:text-slate-500 focus:border-blue-500/50 focus:ring-2 focus:ring-blue-500/20 rounded-lg outline-none"
                        />
                    </div>
                </div>

                <div class="grid gap-4">
                    <div v-if="loading" class="space-y-4">
                        <div v-for="i in 3" :key="i" class="card bg-slate-900/50 border-slate-800">
                            <div class="card-body">
                                <div class="h-6 bg-slate-800 rounded animate-pulse w-1/3 mb-2"></div>
                                <div class="h-4 bg-slate-800 rounded animate-pulse w-2/3"></div>
                            </div>
                        </div>
                    </div>

                    <div v-else-if="filteredDocs.length === 0" class="text-center py-12">
                        <svg class="w-16 h-16 mx-auto mb-4 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                        </svg>
                        <h3 class="text-xl font-medium text-slate-300 mb-2">No documents found</h3>
                        <p class="text-slate-500 mb-6">Get started by adding your first document</p>
                        <router-link v-if="!isViewer" to="/docs/new" class="btn btn-primary">Add Document</router-link>
                    </div>

                    <div v-else v-for="doc in filteredDocs" :key="doc.id"
                         class="card bg-slate-900/50 border-slate-800 hover:border-slate-700 transition-colors cursor-pointer"
                         @click="goToDoc(doc.id)">
                        <div class="card-body p-6">
                            <div class="flex items-start justify-between">
                                <div class="flex-1">
                                    <div class="flex items-center gap-3 mb-2">
                                        <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                                        </svg>
                                        <h3 class="text-lg font-semibold text-slate-200">{{ doc.title }}</h3>
                                        <span class="badge badge-outline badge-sm">{{ doc.format }}</span>
                                        <span :class="['badge badge-sm', doc.status === 'hot' ? 'badge-warning' : doc.status === 'cold' ? 'badge-info' : 'badge-ghost']">
                                            {{ doc.status }}
                                        </span>
                                    </div>
                                    <p class="text-slate-500 text-sm mb-3 line-clamp-1">{{ doc.content?.substring(0, 100) }}...</p>
                                    <div class="flex items-center gap-4 text-sm">
                                        <div class="flex items-center gap-1 text-slate-500">
                                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
                                            </svg>
                                            {{ formatDate(doc.created_at) }}
                                        </div>
                                        <div v-if="doc.tags?.length" class="flex items-center gap-1 text-slate-500">
                                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                                            </svg>
                                            {{ doc.tags.join(', ') }}
                                        </div>
                                    </div>
                                </div>
                                <div class="text-right ml-4 flex flex-col items-end gap-2" @click.stop>
                                    <div class="text-2xl font-bold text-slate-200">{{ (doc.hot_score || 0).toFixed(2) }}</div>
                                    <div class="text-xs text-slate-500">hot score</div>
                                    <button type="button" class="btn btn-sm btn-outline btn-primary" @click="goToDoc(doc.id)">
                                        View details
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    `
};
