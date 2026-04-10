// ==================== LLM Client ====================
class LLMClient {
    constructor(config = {}) {
        this.provider = config.provider || 'openai';
        this.apiKey = config.apiKey;
        this.baseUrl = config.baseUrl || this.getDefaultBaseUrl();
        this.model = config.model || this.getDefaultModel();
        this.maxTokens = config.maxTokens || 2000;
        this.temperature = config.temperature || 0.3;
    }

    getDefaultBaseUrl() {
        switch (this.provider) {
            case 'openai':
                return 'https://api.openai.com/v1';
            case 'anthropic':
                return 'https://api.anthropic.com/v1';
            case 'ollama':
                return 'http://localhost:11434';
            default:
                return 'https://api.openai.com/v1';
        }
    }

    getDefaultModel() {
        switch (this.provider) {
            case 'openai':
                return 'gpt-3.5-turbo';
            case 'anthropic':
                return 'claude-3-haiku-20240307';
            case 'ollama':
                return 'llama2';
            default:
                return 'gpt-3.5-turbo';
        }
    }

    async complete(prompt, { systemPrompt = '', description, max_tokens, temperature }) {
        const response = await fetch(`${this.baseUrl}/chat/completions`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${this.apiKey}`
            },
            body: JSON.stringify({
                model: this.model,
                messages: [
                    { role: 'system', content: systemPrompt },
                    { role: 'user', content: prompt }
                ],
                max_tokens: max_tokens || this.maxTokens,
                temperature: temperature || this.temperature
            })
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(`OpenAI API error: ${errorData.error?.message || response.statusText}`);
        }

        const data = await response.json();
        return data.choices[0].message.content;
    }

    async generateSummary(text, options = {}) {
        const prompt = `Please provide a concise summary of the following text:\n\n${text}`;
        return this.complete(prompt, {
            systemPrompt: 'You are a helpful assistant that summarizes text accurately and concisely.',
            max_tokens: 500,
            ...options
        });
    }

    async generateEmbedding(text) {
        const response = await fetch(`${this.baseUrl}/embeddings`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${this.apiKey}`
            },
            body: JSON.stringify({
                model: 'text-embedding-3-small',
                input: text
            })
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(`OpenAI API error: ${errorData.error?.message || response.statusText}`);
        }

        const data = await response.json();
        return data.data[0].embedding;
    }
}

// ==================== API Client ====================
const apiClient = {
    baseURL: '',

    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${endpoint}`;
        const config = {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        };

        try {
            const response = await fetch(url, config);
            if (!response.ok) {
                const error = await response.json().catch(() => ({ message: response.statusText }));
                throw new Error(error.message || `HTTP ${response.status}`);
            }
            return await response.json();
        } catch (error) {
            console.error('API request failed:', error);
            throw error;
        }
    },

    // Documents
    getDocuments: () => apiClient.request('/api/v1/documents').then(r => r.documents || []),
    getDocument: (id) => apiClient.request(`/api/v1/documents/${id}`),
    getDocumentSummaries: (id) => apiClient.request(`/api/v1/documents/${id}/summaries`).then(r => r.summaries || []),
    getDocumentChapters: (id) => apiClient.request(`/api/v1/documents/${id}/chapters`).then(r => r.chapters || []),
    createDocument: (data) => apiClient.request('/api/v1/documents', { method: 'POST', body: JSON.stringify(data) }),
    deleteDocument: (id) => apiClient.request(`/api/v1/documents/${id}`, { method: 'DELETE' }),
    
    // Query
    progressiveQuery: (question) => apiClient.request('/api/v1/query/progressive', { 
        method: 'POST', 
        body: JSON.stringify({ question, max_results: 100 }) 
    }),
    
    // Tags
    getTags: () => apiClient.request('/api/v1/tags').then(r => r.tags || []),
    getTagGroups: () => apiClient.request('/api/v1/tags/groups').then(r => r.groups || []),
    triggerTagGrouping: () => apiClient.request('/api/v1/tags/group', { method: 'POST' }),
};

// ==================== Components ====================

// Navigation Header
const AppHeader = {
    template: `
        <header class="border-b border-slate-800 bg-slate-950/80 backdrop-blur-md sticky top-0 z-50">
            <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
                <router-link to="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity">
                    <svg class="w-7 h-7 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                    </svg>
                    <span class="text-xl font-bold bg-gradient-to-r from-blue-400 to-emerald-400 bg-clip-text text-transparent">
                        TierSum
                    </span>
                </router-link>
                <nav class="flex items-center gap-1">
                    <router-link to="/" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                            </svg>
                            Search
                        </button>
                    </router-link>
                    <router-link to="/docs" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                            </svg>
                            Documents
                        </button>
                    </router-link>
                    <router-link to="/tags" custom v-slot="{ navigate, isActive }">
                        <button @click="navigate" :class="['btn btn-ghost btn-sm', isActive ? 'text-blue-400 bg-blue-500/10' : 'text-slate-400']">
                            <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
                            </svg>
                            Tags
                        </button>
                    </router-link>
                </nav>
            </div>
        </header>
    `
};

// Search Page
const SearchPage = {
    data() {
        return {
            query: '',
            loading: false,
            results: [],
            hasSearched: false,
            aiAnswer: '',
            aiLoading: false,
            highlightedRef: null
        }
    },
    computed: {
        hasResults() {
            return this.results && this.results.length > 0;
        }
    },
    methods: {
        async handleSearch() {
            if (!this.query.trim()) return;
            
            this.loading = true;
            this.aiLoading = true;
            this.hasSearched = true;
            this.aiAnswer = '';
            this.results = [];
            
            try {
                const response = await apiClient.progressiveQuery(this.query);
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

${topResults.map((r, i) => `- **${r.title}** (relevance ${(r.relevance * 100).toFixed(0)}%)`).join('\n')}

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
            if (!content) return '';
            return marked.parse(content);
        },
        
        extractDocName(path) {
            return path?.split('/')[0] || path || 'Unknown';
        },
        
        extractChapterPath(path) {
            const parts = path?.split('/') || [];
            return parts.length > 1 ? parts.slice(1).join('/') : '';
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
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-[1600px] mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <!-- Search Section -->
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
                    </div>
                </div>

                <!-- Results Section -->
                <div v-if="hasSearched" class="grid grid-cols-1 lg:grid-cols-12 gap-6 mt-8">
                    <!-- Left Panel - AI Answer -->
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
                                    <div v-else-if="aiAnswer" class="prose prose-invert max-w-none">
                                        <div v-html="renderMarkdown(aiAnswer)" class="text-slate-300 leading-relaxed"></div>
                                    </div>
                                    <div v-else class="text-center py-12 text-slate-500">
                                        <p>Generating AI answer...</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Right Panel - References -->
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
                                <div class="p-4 overflow-y-auto h-[calc(100%-80px)] space-y-4">
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
                                        <div v-for="(result, index) in results" :key="result.id" 
                                             :id="'ref-' + index"
                                             :class="['card bg-slate-800/30 border transition-all cursor-pointer', 
                                                      highlightedRef === index ? 'border-blue-500 ring-2 ring-blue-500/50' : 'border-slate-700 hover:border-slate-600']"
                                             @click="highlightedRef = index">
                                            <div class="card-body p-4">
                                                <div class="flex justify-between items-start mb-2">
                                                    <div class="flex items-center gap-2">
                                                        <span :class="['badge badge-sm', refTierBadgeClass(result.docStatus)]">
                                                            {{ refTierLabel(result.docStatus) }}
                                                        </span>
                                                        <span class="text-xs text-slate-500">#{{ index + 1 }}</span>
                                                    </div>
                                                    <span class="badge badge-outline badge-sm">{{ (result.relevance * 100).toFixed(0) }}%</span>
                                                </div>
                                                <h3 class="font-semibold text-slate-200 line-clamp-2 mb-1">{{ result.title }}</h3>
                                                <p class="text-xs text-slate-500 mb-2">
                                                    From: {{ extractDocName(result.path) }} {{ extractChapterPath(result.path) ? '· ' + extractChapterPath(result.path) : '' }}
                                                </p>
                                                <p class="text-sm text-slate-400 line-clamp-4">{{ result.content?.substring(0, 300) }}{{ result.content?.length > 300 ? '...' : '' }}</p>
                                                <div class="flex justify-between items-center mt-3 pt-2 border-t border-slate-700/50">
                                                    <span class="text-xs text-slate-600 truncate max-w-[150px]">{{ result.path }}</span>
                                                    <button type="button" @click.stop="$router.push('/docs/' + result.id)" class="text-sm text-blue-400 hover:text-blue-300 flex items-center gap-1">
                                                        View <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
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

// New document: full-page editor + live Markdown preview (no modal)
const DocumentCreatePage = {
    data() {
        return {
            newDoc: {
                title: '',
                content: '',
                format: 'markdown',
                tags: [],
                force_hot: false
            },
            tagInput: '',
            submitting: false
        }
    },
    methods: {
        addTag() {
            if (this.tagInput.trim() && !this.newDoc.tags.includes(this.tagInput.trim())) {
                this.newDoc.tags.push(this.tagInput.trim());
                this.tagInput = '';
            }
        },
        removeTag(index) {
            this.newDoc.tags.splice(index, 1);
        },
        renderPreview(text) {
            const t = (text || '').trim();
            if (!t) {
                return '<p class="text-slate-500 italic">Preview appears here as you type Markdown.</p>';
            }
            try {
                return marked.parse(t);
            } catch {
                return '<p class="text-red-400">Invalid Markdown.</p>';
            }
        },
        async submitDocument() {
            if (!this.newDoc.title.trim() || !this.newDoc.content.trim()) return;
            this.submitting = true;
            try {
                const payload = {
                    title: this.newDoc.title.trim(),
                    content: this.newDoc.content,
                    format: this.newDoc.format || 'markdown',
                    tags: this.newDoc.tags
                };
                if (this.newDoc.force_hot) {
                    payload.force_hot = true;
                }
                const created = await apiClient.createDocument(payload);
                const id = created?.id || created?.ID;
                if (id) {
                    this.$router.push(`/docs/${id}`);
                } else {
                    this.$router.push('/docs');
                }
            } catch (error) {
                console.error('Failed to create document:', error);
                alert('Failed to create document: ' + error.message);
            } finally {
                this.submitting = false;
            }
        },
        goBack() {
            this.$router.push('/docs');
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <main class="max-w-[1800px] mx-auto px-4 sm:px-6 lg:px-8 py-6 pb-16">
                <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
                    <div class="flex items-center gap-3">
                        <button type="button" @click="goBack" class="btn btn-ghost btn-sm gap-2 text-slate-400">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
                            </svg>
                            Back to list
                        </button>
                        <h1 class="text-2xl sm:text-3xl font-bold text-slate-100">New document</h1>
                    </div>
                    <div class="flex flex-wrap items-center gap-2">
                        <label class="label cursor-pointer gap-2 py-0">
                            <input type="checkbox" v-model="newDoc.force_hot" class="checkbox checkbox-sm checkbox-primary" />
                            <span class="label-text text-slate-400 text-sm">Force hot (use quota, full LLM)</span>
                        </label>
                        <button type="button"
                            @click="submitDocument"
                            :disabled="submitting || !newDoc.title.trim() || !newDoc.content.trim()"
                            class="btn btn-primary">
                            <span v-if="submitting" class="loading loading-spinner loading-sm mr-2"></span>
                            Create &amp; open
                        </button>
                    </div>
                </div>

                <div class="grid grid-cols-1 xl:grid-cols-2 gap-6 min-h-[calc(100vh-12rem)]">
                    <!-- Editor column -->
                    <div class="flex flex-col gap-4 min-h-0">
                        <div class="card bg-slate-900/50 border border-slate-800 flex-1 flex flex-col min-h-[520px] xl:min-h-[calc(100vh-14rem)]">
                            <div class="card-body flex flex-col flex-1 min-h-0 gap-4">
                                <div>
                                    <label class="label"><span class="label-text text-slate-300">Title</span></label>
                                    <input v-model="newDoc.title" type="text" placeholder="Document title"
                                        class="input input-bordered w-full bg-slate-800/80 border-slate-700 text-slate-100" />
                                </div>
                                <div>
                                    <label class="label"><span class="label-text text-slate-300">Tags</span></label>
                                    <div class="flex gap-2 mb-2">
                                        <input v-model="tagInput" @keydown.enter.prevent="addTag" type="text"
                                            placeholder="Tag name, Enter to add"
                                            class="input input-bordered flex-1 bg-slate-800/80 border-slate-700 text-slate-100" />
                                        <button type="button" @click="addTag" class="btn btn-outline border-slate-600">Add</button>
                                    </div>
                                    <div class="flex flex-wrap gap-2">
                                        <span v-for="(tag, index) in newDoc.tags" :key="index" class="badge badge-primary gap-1">
                                            {{ tag }}
                                            <button type="button" @click="removeTag(index)" class="hover:text-white" aria-label="Remove tag">×</button>
                                        </span>
                                    </div>
                                </div>
                                <div class="flex flex-col flex-1 min-h-0">
                                    <label class="label py-0"><span class="label-text text-slate-300">Content (Markdown)</span></label>
                                    <textarea v-model="newDoc.content"
                                        placeholder="# Heading — write Markdown here…"
                                        class="textarea textarea-bordered flex-1 min-h-[320px] w-full bg-slate-800/80 border-slate-700 text-slate-100 font-mono text-sm leading-relaxed resize-y"></textarea>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Preview column -->
                    <div class="flex flex-col min-h-0 xl:sticky xl:top-20 xl:self-start xl:max-h-[calc(100vh-6rem)]">
                        <div class="card bg-slate-900/50 border border-slate-800 h-full min-h-[320px] xl:max-h-[calc(100vh-14rem)] flex flex-col">
                            <div class="px-4 py-3 border-b border-slate-800 flex items-center justify-between shrink-0">
                                <h2 class="text-sm font-semibold text-slate-400 uppercase tracking-wide">Preview</h2>
                                <span class="text-xs text-slate-600">Live</span>
                            </div>
                            <div class="card-body overflow-y-auto flex-1 min-h-0 pt-4">
                                <article class="prose prose-invert prose-sm sm:prose-base max-w-none prose-headings:text-slate-100 prose-p:text-slate-300 prose-a:text-blue-400 prose-code:text-emerald-300">
                                    <div v-html="renderPreview(newDoc.content)"></div>
                                </article>
                            </div>
                        </div>
                    </div>
                </div>
            </main>
        </div>
    `
};

// Documents Page
const DocumentsPage = {
    data() {
        return {
            documents: [],
            loading: true,
            searchQuery: ''
        }
    },
    computed: {
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
        async deleteDocument(id) {
            if (!confirm('Are you sure you want to delete this document?')) return;
            
            try {
                await apiClient.deleteDocument(id);
                await this.loadDocuments();
            } catch (error) {
                console.error('Failed to delete document:', error);
                alert('Failed to delete document: ' + error.message);
            }
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
                    <router-link to="/docs/new" class="btn btn-primary">
                        <svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
                        </svg>
                        Add Document
                    </router-link>
                </div>

                <!-- Search -->
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

                <!-- Documents List -->
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
                        <router-link to="/docs/new" class="btn btn-primary">Add Document</router-link>
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

// Document detail: summary mode (hot, with chapter nav) vs source mode (full markdown)
const DocumentDetailPage = {
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
        isCold() {
            return this.doc?.status === 'cold';
        },
        docSummaryRecord() {
            return this.summaries.find(s => s.tier === 'document');
        },
        docSummaryText() {
            return (this.docSummaryRecord?.content || '').trim();
        },
        showSummaryTab() {
            if (!this.doc || this.isCold) return false;
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
            return (ch?.summary || '').trim() || '_No summary for this section._';
        }
    },
    watch: {
        id: {
            immediate: true,
            handler() {
                this.load();
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
                if (this.showSummaryTab) {
                    if (this.docSummaryText) {
                        this.selectedNav = 'overview';
                    } else if (this.chapters.length) {
                        this.selectedNav = this.chapters[0].path;
                    } else {
                        this.selectedNav = 'overview';
                    }
                } else {
                    this.selectedNav = 'overview';
                }
            } catch (e) {
                this.loadError = e.message || String(e);
            } finally {
                this.loading = false;
            }
        },
        applyDefaultView() {
            if (this.isCold) {
                this.viewMode = 'source';
                return;
            }
            if (!this.showSummaryTab) {
                this.viewMode = 'source';
                return;
            }
            this.viewMode = 'summary';
        },
        renderMd(text) {
            if (!text) return '';
            try {
                return marked.parse(text);
            } catch {
                return '<p class="text-red-400">Failed to render markdown.</p>';
            }
        },
        selectNav(key) {
            this.selectedNav = key;
        },
        setViewMode(mode) {
            if (mode === 'summary' && (this.isCold || !this.showSummaryTab)) return;
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
                        <div v-if="isCold" class="shrink-0">
                            <span class="badge badge-lg badge-info">Cold document — full text</span>
                        </div>
                        <div v-else-if="showSummaryTab" class="shrink-0 join">
                            <button type="button"
                                class="btn btn-sm join-item"
                                :class="viewMode === 'summary' ? 'btn-primary' : 'btn-ghost border border-slate-700'"
                                @click="setViewMode('summary')">
                                Summary
                            </button>
                            <button type="button"
                                class="btn btn-sm join-item"
                                :class="viewMode === 'source' ? 'btn-primary' : 'btn-ghost border border-slate-700'"
                                @click="setViewMode('source')">
                                Original
                            </button>
                        </div>
                    </div>

                    <!-- Cold: always full document -->
                    <div v-if="isCold" class="card bg-slate-900/50 border border-slate-800">
                        <div class="card-body">
                            <h2 class="text-lg font-semibold text-slate-200 mb-4">Original</h2>
                            <div class="prose prose-invert max-w-none prose-headings:text-slate-100 prose-p:text-slate-300 border-t border-slate-800 pt-4">
                                <div v-html="renderMd(doc.content || '')"></div>
                            </div>
                        </div>
                    </div>

                    <!-- Hot: summary layout -->
                    <div v-else-if="viewMode === 'summary' && showSummaryTab" class="grid grid-cols-1 lg:grid-cols-12 gap-6">
                        <aside class="lg:col-span-3">
                            <div class="card bg-slate-900/50 border border-slate-800 lg:sticky lg:top-24">
                                <div class="card-body p-4">
                                    <h2 class="text-sm font-semibold text-slate-400 uppercase tracking-wide mb-3">Chapters</h2>
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
                                    <p v-if="!docSummaryText && chapters.length === 0" class="text-sm text-slate-500 mt-2">No structured summary yet.</p>
                                </div>
                            </div>
                        </aside>
                        <div class="lg:col-span-9">
                            <div class="card bg-slate-900/50 border border-slate-800 min-h-[320px]">
                                <div class="card-body">
                                    <h2 class="text-lg font-semibold text-slate-200 mb-2">
                                        {{ selectedNav === 'overview' ? 'Document summary' : (activeChapter?.title || 'Chapter') }}
                                    </h2>
                                    <div class="prose prose-invert max-w-none prose-headings:text-slate-100 prose-p:text-slate-300 border-t border-slate-800 pt-4">
                                        <div v-html="renderMd(summaryBodyMarkdown)"></div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Hot: original full text -->
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

// Tags Page
const TagsPage = {
    data() {
        return {
            tagGroups: [],
            tags: [],
            loading: true,
            selectedGroup: null
        }
    },
    computed: {
        filteredTags() {
            if (!this.selectedGroup) return [];
            return this.tags.filter(tag => tag.group_id === this.selectedGroup.id);
        }
    },
    async mounted() {
        await this.loadData();
    },
    methods: {
        async loadData() {
            try {
                this.loading = true;
                const [groupsData, tagsData] = await Promise.all([
                    apiClient.getTagGroups(),
                    apiClient.getTags()
                ]);
                this.tagGroups = groupsData;
                this.tags = tagsData;
                if (groupsData.length > 0 && !this.selectedGroup) {
                    this.selectedGroup = groupsData[0];
                }
            } catch (error) {
                console.error('Failed to load tags:', error);
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
                    <!-- L1 Tag Groups -->
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
                                            @click="selectedGroup = group"
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

                    <!-- L2 Tags -->
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
                                    <div v-else-if="!selectedGroup" class="text-center py-12 text-slate-500">
                                        <svg class="w-12 h-12 mx-auto mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
                                        </svg>
                                        <p>Select a group to view tags</p>
                                    </div>
                                    <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                        <div
                                            v-for="tag in filteredTags"
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
                                        <div v-if="filteredTags.length === 0" class="col-span-2 text-center py-12 text-slate-500">
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

// ==================== Router & App ====================
const { createRouter, createWebHashHistory } = VueRouter;

const routes = [
    { path: '/', component: SearchPage },
    { path: '/docs', component: DocumentsPage },
    { path: '/docs/new', component: DocumentCreatePage },
    { path: '/docs/:id', component: DocumentDetailPage, props: true },
    { path: '/tags', component: TagsPage }
];

const router = createRouter({
    history: createWebHashHistory(),
    routes
});

const { createApp } = Vue;

const App = {
    components: { AppHeader },
    template: `
        <div class="min-h-screen bg-slate-950">
            <AppHeader />
            <router-view v-slot="{ Component }">
                <transition name="fade" mode="out-in">
                    <component :is="Component" />
                </transition>
            </router-view>
        </div>
    `
};

createApp(App)
    .use(router)
    .mount('#app');
