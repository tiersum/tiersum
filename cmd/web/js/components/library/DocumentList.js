import { Icon } from '../Icon.js';
import { DocumentCard } from './DocumentCard.js';
import { BROWSE_ALL, BROWSE_COLD, BROWSE_UNTAGGED, BROWSE_TOPIC } from '../../utils/libraryFilters.js';

export const DocumentList = {
    components: { Icon, DocumentCard },
    props: {
        loading: Boolean,
        docs: { type: Array, default: () => [] },
        browseMode: String,
        selectedTopic: { type: Object, default: null },
        selectedCatalogTagName: { type: String, default: null },
        isViewer: Boolean,
        tags: { type: Array, default: () => [] },
        tagsLoading: Boolean
    },
    emits: ['update:search-query', 'open-doc', 'clear-catalog-tag', 'select-catalog-tag'],
    data() {
        return {
            localSearch: ''
        };
    },
    watch: {
        localSearch(val) {
            this.$emit('update:search-query', val);
        }
    },
    methods: {
        onOpenDoc(id) {
            this.$emit('open-doc', id);
        },
        onClearTag() {
            this.$emit('clear-catalog-tag');
        },
        onSelectTag(tag) {
            this.$emit('select-catalog-tag', tag);
        },
        filterLabel() {
            if (this.browseMode === BROWSE_COLD) return this.$t('libraryFilterCold');
            if (this.browseMode === BROWSE_UNTAGGED) return this.$t('libraryFilterUntagged');
            if (this.browseMode === BROWSE_TOPIC && this.selectedTopic) {
                if (this.selectedCatalogTagName) {
                    return this.$t('libraryFilterTopicTag', { topic: this.selectedTopic.name, tag: this.selectedCatalogTagName });
                }
                return this.$t('libraryFilterTopic', { topic: this.selectedTopic.name });
            }
            return this.$t('libraryFilterAll');
        },
        emptyTitle() {
            if (this.browseMode === BROWSE_COLD) return this.$t('libraryEmptyCold');
            if (this.browseMode === BROWSE_UNTAGGED) return this.$t('libraryEmptyUntagged');
            if (this.browseMode === BROWSE_TOPIC) return this.$t('libraryEmptyTopic');
            return this.$t('libraryEmptyAll');
        },
        emptyHint() {
            if (this.browseMode === BROWSE_COLD) {
                return this.$t('libraryHintCold');
            }
            if (this.browseMode === BROWSE_UNTAGGED) {
                return this.$t('libraryHintUntagged');
            }
            if (this.browseMode === BROWSE_TOPIC) {
                return this.$t('libraryHintTopic');
            }
            return this.$t('libraryHintAll');
        },
        searchPlaceholder() {
            if (this.browseMode === BROWSE_ALL) return this.$t('librarySearchAll');
            if (this.browseMode === BROWSE_COLD) return this.$t('librarySearchCold');
            if (this.browseMode === BROWSE_UNTAGGED) return this.$t('librarySearchUntagged');
            return this.$t('librarySearchTopic');
        }
    },
    template: `
        <div class="card bg-slate-900/50 border-slate-800 min-h-[320px] h-full flex flex-col">
            <div class="p-4 border-b border-slate-800 shrink-0 space-y-3">
                <!-- Filter status + result count -->
                <div class="flex flex-wrap items-center justify-between gap-2">
                    <div class="flex items-center gap-2 text-sm text-slate-300 min-w-0">
                        <icon name="filter" class-name="w-4 h-4 text-slate-500 shrink-0" />
                        <span class="truncate">{{ filterLabel() }}</span>
                        <span class="text-slate-500">· {{ docs.length }}</span>
                    </div>
                </div>

                <!-- Tag chips (topic mode only) -->
                <div v-if="browseMode === '${BROWSE_TOPIC}' && selectedTopic" class="space-y-2">
                    <div v-if="tagsLoading" class="flex gap-2">
                        <div v-for="i in 4" :key="'tc'+i" class="h-7 w-20 bg-slate-800 rounded-full animate-pulse"></div>
                    </div>
                    <div v-else-if="tags.length" class="flex flex-wrap gap-2">
                        <button
                            type="button"
                            :class="[
                                'px-3 py-1 rounded-full text-xs border transition-colors',
                                !selectedCatalogTagName
                                    ? 'bg-blue-500/15 border-blue-500/40 text-blue-300'
                                    : 'bg-slate-800/40 border-slate-700 text-slate-300 hover:border-slate-600'
                            ]"
                            @click="onClearTag"
                        >
                            {{ $t('libraryAllTags') }}
                        </button>
                        <button
                            v-for="tag in tags"
                            :key="tag.id"
                            type="button"
                            :class="[
                                'px-3 py-1 rounded-full text-xs border transition-colors inline-flex items-center gap-1.5',
                                selectedCatalogTagName === tag.name
                                    ? 'bg-emerald-500/15 border-emerald-500/40 text-emerald-300'
                                    : 'bg-slate-800/40 border-slate-700 text-slate-300 hover:border-slate-600'
                            ]"
                            @click="onSelectTag(tag)"
                        >
                            {{ tag.name }}
                            <span class="text-[10px] opacity-70">{{ tag.document_count }}</span>
                        </button>
                    </div>
                </div>

                <!-- Search -->
                <div class="relative max-w-md">
                    <icon name="search" class-name="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
                    <input
                        v-model="localSearch"
                        :placeholder="searchPlaceholder()"
                        class="w-full pl-10 pr-4 py-2 bg-slate-900/50 border border-slate-800 text-slate-100 placeholder:text-slate-500 focus:border-blue-500/50 focus:ring-2 focus:ring-blue-500/20 rounded-lg outline-none text-sm"
                    />
                </div>
            </div>

            <div class="p-4 overflow-y-auto flex-1">
                <div v-if="loading" class="space-y-4">
                    <div v-for="i in 3" :key="'d'+i" class="rounded-lg border border-slate-800 bg-slate-900/40 p-4 animate-pulse">
                        <div class="h-5 bg-slate-800 rounded w-1/3 mb-2"></div>
                        <div class="h-4 bg-slate-800 rounded w-2/3"></div>
                    </div>
                </div>
                <div v-else-if="docs.length === 0" class="text-center py-12">
                    <icon name="empty" class-name="w-16 h-16 mx-auto mb-4 text-slate-600" />
                    <h3 class="text-xl font-medium text-slate-300 mb-2">{{ emptyTitle() }}</h3>
                    <p class="text-slate-500 mb-4 text-sm max-w-sm mx-auto">{{ emptyHint() }}</p>
                    <router-link v-if="!isViewer" to="/docs/new" class="btn btn-primary btn-sm">{{ $t('libraryAddDoc') }}</router-link>
                </div>
                <div v-else class="grid gap-3">
                    <document-card
                        v-for="doc in docs"
                        :key="doc.id"
                        :doc="doc"
                        @open="onOpenDoc"
                    />
                </div>
            </div>
        </div>
    `
};
