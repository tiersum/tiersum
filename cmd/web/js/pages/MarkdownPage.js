/** Markdown page component: loads and renders .md files from /site/ directory. */

import { parseMarkdown } from '../markdown.js';
import { getLocale } from '../i18n.js';

export const MarkdownPage = {
    data() {
        return {
            content: '',
            loading: true,
            error: null,
            title: ''
        };
    },
    computed: {
        path() {
            return this.$route.params.path || 'index';
        },
        locale() {
            return getLocale();
        }
    },
    watch: {
        path: {
            immediate: true,
            handler() {
                this.loadMarkdown();
            }
        },
        locale() {
            this.loadMarkdown();
        },
        '$route.hash'() {
            this.scrollToHash();
        }
    },
    methods: {
        async loadMarkdown() {
            this.loading = true;
            this.error = null;
            this.content = '';

            try {
                const path = this.path;
                const safePath = path.replace(/[^a-zA-Z0-9-_/]/g, '');
                const locale = getLocale();
                const url = locale === 'zh'
                    ? `/site/${safePath}.zh.md`
                    : `/site/${safePath}.md`;

                const response = await fetch(url);
                if (!response.ok) {
                    if (response.status === 404) {
                        throw new Error(this.$t('mdPageNotFound'));
                    }
                    throw new Error(this.$t('mdLoadFailed', { status: response.status }));
                }
                
                const text = await response.text();
                this.content = parseMarkdown(text);
                
                // Extract title from first h1
                const titleMatch = text.match(/^#\s+(.+)$/m);
                this.title = titleMatch ? titleMatch[1] : safePath;
                
                // Update page title
                if (typeof document !== 'undefined') {
                    document.title = this.title + ' — TierSum';
                }
            } catch (e) {
                this.error = e.message || this.$t('error');
                console.error('Failed to load markdown:', e);
            } finally {
                this.loading = false;
                // Scroll to hash after DOM update
                this.$nextTick(() => {
                    this.scrollToHash();
                });
            }
        },
        scrollToHash() {
            const hash = this.$route.hash;
            if (!hash) return;
            const id = hash.slice(1);
            const el = document.getElementById(id);
            if (el) {
                el.scrollIntoView({ behavior: 'smooth', block: 'start' });
            }
        }
    },
    template: `
        <div class="min-h-screen bg-slate-950">
            <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 pb-16">
                <div class="flex flex-col lg:flex-row gap-8">
                    <!-- Sidebar Navigation -->
                    <aside class="lg:w-64 shrink-0">
                        <nav class="sticky top-24 space-y-6">
                            <div>
                                <h3 class="text-xs font-semibold text-slate-500 uppercase tracking-wider mb-3">{{ $t('mdSiteNav') }}</h3>
                                <ul class="space-y-1">
                                    <li>
                                        <router-link 
                                            to="/site/index" 
                                            :class="['block px-3 py-2 rounded-lg text-sm transition-colors', path === 'index' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            {{ $t('navHome') }}
                                        </router-link>
                                    </li>
                                    <li>
                                        <router-link 
                                            to="/site/features" 
                                            :class="['block px-3 py-2 rounded-lg text-sm transition-colors', path === 'features' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            {{ $t('navFeatures') }}
                                        </router-link>
                                    </li>
                                    <li>
                                        <router-link 
                                            to="/site/about" 
                                            :class="['block px-3 py-2 rounded-lg text-sm transition-colors', path === 'about' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            {{ $t('navAbout') }}
                                        </router-link>
                                    </li>
                                    <li>
                                        <router-link 
                                            to="/site/documentation" 
                                            :class="['block px-3 py-2 rounded-lg text-sm transition-colors', path === 'documentation' ? 'bg-blue-500/10 text-blue-300' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50']"
                                        >
                                            {{ $t('navDocs') }}
                                        </router-link>
                                    </li>
                                </ul>
                            </div>
                        </nav>
                    </aside>

                    <!-- Main Content -->
                    <main class="flex-1 min-w-0">
                        <div v-if="loading" class="space-y-4">
                            <div class="h-8 bg-slate-800 rounded animate-pulse w-1/3"></div>
                            <div class="h-4 bg-slate-800 rounded animate-pulse w-full"></div>
                            <div class="h-4 bg-slate-800 rounded animate-pulse w-5/6"></div>
                            <div class="h-4 bg-slate-800 rounded animate-pulse w-4/6"></div>
                        </div>
                        
                        <div v-else-if="error" class="card bg-slate-900/50 border border-slate-800">
                            <div class="card-body">
                                <div class="alert alert-error bg-red-950/40 border-red-900 text-red-200">
                                    <svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                    </svg>
                                    <span>{{ error }}</span>
                                </div>
                                <router-link to="/site/index" class="btn btn-primary btn-sm mt-4">
                                    {{ $t('mdGoHome') }}
                                </router-link>
                            </div>
                        </div>
                        
                         <div v-else class="markdown-body max-w-none text-slate-300" ref="contentEl">
                            <div v-html="content"></div>
                        </div>
                    </main>
                </div>
            </div>
        </div>
    `
};
