import { Icon } from '../Icon.js';
import { BROWSE_ALL, BROWSE_COLD, BROWSE_UNTAGGED, BROWSE_TOPIC } from '../../utils/libraryFilters.js';

const BROWSE_ITEMS = [
    { key: BROWSE_ALL, label: 'All documents' },
    { key: BROWSE_COLD, label: 'Cold' },
    { key: BROWSE_UNTAGGED, label: 'Untagged' }
];

function tabClass(active) {
    return [
        'px-3 py-2 rounded-md text-sm font-medium transition-colors',
        active
            ? 'bg-blue-500/15 text-blue-300'
            : 'text-slate-300 hover:bg-slate-800 hover:text-slate-100'
    ].join(' ');
}

function topicClass(active) {
    return [
        'w-full text-left px-3 py-2.5 rounded-lg transition-all border-l-4 text-sm',
        active
            ? 'bg-blue-500/10 border-blue-500 text-blue-300'
            : 'bg-transparent border-transparent text-slate-300 hover:bg-slate-800/60 hover:text-slate-100'
    ].join(' ');
}

export const LibraryNav = {
    components: { Icon },
    props: {
        loading: Boolean,
        browseMode: String,
        topics: { type: Array, default: () => [] },
        selectedTopic: { type: Object, default: null },
        isViewer: Boolean,
        regrouping: Boolean
    },
    emits: ['set-browse-mode', 'select-topic', 'regroup'],
    data() {
        return {
            browseItems: BROWSE_ITEMS
        };
    },
    methods: {
        tabClass,
        topicClass,
        onSetBrowseMode(mode) {
            this.$emit('set-browse-mode', mode);
        },
        onSelectTopic(topic) {
            this.$emit('select-topic', topic);
        },
        onRegroup() {
            this.$emit('regroup');
        }
    },
    template: `
        <div class="card bg-slate-900/50 border-slate-800 h-full flex flex-col">
            <div class="p-4 border-b border-slate-800 shrink-0">
                <h2 class="text-lg font-semibold text-slate-100 flex items-center gap-2">
                    <icon name="folder" class-name="w-5 h-5 text-blue-500" />
                    Browse
                </h2>
            </div>
            <div class="p-3 overflow-y-auto flex-1">
                <div v-if="loading" class="space-y-3">
                    <div v-for="i in 6" :key="'sk'+i" class="h-10 bg-slate-800 rounded animate-pulse"></div>
                </div>
                <template v-else>
                    <!-- Browse tabs -->
                    <div class="flex flex-wrap gap-1 mb-4">
                        <button
                            v-for="item in browseItems"
                            :key="item.key"
                            type="button"
                            :class="tabClass(browseMode === item.key)"
                            @click="onSetBrowseMode(item.key)"
                        >
                            {{ item.label }}
                        </button>
                    </div>

                    <!-- Topics -->
                    <div class="border-t border-slate-800 pt-3">
                        <div class="flex items-center justify-between px-1 mb-2">
                            <p class="text-[10px] uppercase tracking-wider text-slate-500">Topics</p>
                            <button
                                v-if="!isViewer"
                                type="button"
                                class="text-[10px] text-blue-400 hover:text-blue-300 disabled:opacity-50"
                                :disabled="regrouping"
                                @click="onRegroup"
                            >
                                {{ regrouping ? 'Regrouping...' : 'Regroup' }}
                            </button>
                        </div>
                        <template v-if="topics.length === 0">
                            <div class="text-center py-6 text-slate-500 text-sm px-2">
                                No topics yet. Ingest tagged documents or use Regroup.
                            </div>
                        </template>
                        <template v-else>
                            <div class="space-y-1">
                                <button
                                    v-for="topic in topics"
                                    :key="topic.id"
                                    type="button"
                                    :class="topicClass(browseMode === '${BROWSE_TOPIC}' && selectedTopic?.id === topic.id)"
                                    @click="onSelectTopic(topic)"
                                >
                                    <div class="flex items-center justify-between gap-2">
                                        <span class="font-medium line-clamp-2">{{ topic.name }}</span>
                                        <span class="text-[10px] text-slate-500 shrink-0">{{ topic.tag_names?.length || 0 }}</span>
                                    </div>
                                </button>
                            </div>
                        </template>
                    </div>
                </template>
            </div>
        </div>
    `
};
