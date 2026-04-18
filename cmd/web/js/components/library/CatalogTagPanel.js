import { Icon } from '../Icon.js';

function btnClass(active) {
    return [
        'w-full text-left px-4 py-3 rounded-lg transition-all border text-sm',
        active
            ? 'bg-blue-500/10 border-blue-500/50 text-blue-300'
            : 'bg-slate-800/30 border-transparent text-slate-200 hover:bg-slate-800/60 hover:border-slate-700'
    ].join(' ');
}

export const CatalogTagPanel = {
    components: { Icon },
    props: {
        visible: Boolean,
        loading: Boolean,
        selectedTopic: { type: Object, default: null },
        tags: { type: Array, default: () => [] },
        selectedCatalogTagName: { type: String, default: null }
    },
    emits: ['clear-catalog-tag', 'select-catalog-tag'],
    methods: {
        btnClass,
        onClear() {
            this.$emit('clear-catalog-tag');
        },
        onSelect(tag) {
            this.$emit('select-catalog-tag', tag);
        }
    },
    template: `
        <div v-if="visible" class="card bg-slate-900/50 border-slate-800 min-h-[200px] h-full">
            <div class="card-body p-0 flex flex-col h-full">
                <div class="p-4 border-b border-slate-800 flex items-center justify-between gap-2 shrink-0">
                    <div class="flex items-center gap-2 min-w-0">
                        <icon name="tag" class-name="w-5 h-5 text-emerald-500" />
                        <h2 class="text-lg font-semibold text-slate-100 truncate">Tags</h2>
                    </div>
                    <span class="badge badge-outline badge-primary truncate max-w-[8rem]">{{ selectedTopic?.name }}</span>
                </div>
                <div class="p-3 overflow-y-auto flex-1 space-y-2">
                    <button
                        type="button"
                        :class="btnClass(!selectedCatalogTagName)"
                        @click="onClear"
                    >
                        All tags in topic
                    </button>
                    <div v-if="loading" class="space-y-2">
                        <div v-for="i in 5" :key="'tg'+i" class="h-14 bg-slate-800 rounded animate-pulse"></div>
                    </div>
                    <template v-else>
                        <button
                            v-for="tag in tags"
                            :key="tag.id"
                            type="button"
                            :class="btnClass(selectedCatalogTagName === tag.name)"
                            @click="onSelect(tag)"
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
    `
};
