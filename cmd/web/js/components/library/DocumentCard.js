import { Icon } from '../Icon.js';

export const DocumentCard = {
    components: { Icon },
    props: {
        doc: { type: Object, required: true }
    },
    emits: ['open'],
    methods: {
        formatDate(dateStr) {
            return new Date(dateStr).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric'
            });
        },
        onOpen() {
            this.$emit('open', this.doc.id);
        },
        statusMeta(status) {
            if (status === 'hot') return { dot: 'bg-amber-400', label: 'hot' };
            if (status === 'cold') return { dot: 'bg-blue-400', label: 'cold' };
            return { dot: 'bg-slate-400', label: 'warming' };
        }
    },
    template: `
        <div
            class="group rounded-xl border border-slate-800 bg-slate-900/40 hover:border-slate-600 hover:bg-slate-800/40 transition-all cursor-pointer p-4 sm:p-5"
            @click="onOpen"
        >
            <div class="flex items-start justify-between gap-3">
                <div class="flex-1 min-w-0">
                    <div class="flex flex-wrap items-center gap-2 mb-2">
                        <icon name="document" class-name="w-5 h-5 text-blue-500 shrink-0" />
                        <h3 class="text-base font-semibold text-slate-200 truncate">{{ doc.title }}</h3>
                        <span class="text-[10px] uppercase tracking-wide text-slate-500 border border-slate-700 rounded px-1.5 py-0.5">{{ doc.format }}</span>
                        <span class="inline-flex items-center gap-1 text-[10px] text-slate-400">
                            <span :class="['w-1.5 h-1.5 rounded-full', statusMeta(doc.status).dot]"></span>
                            {{ statusMeta(doc.status).label }}
                        </span>
                    </div>
                    <p class="text-slate-500 text-sm mb-3 line-clamp-2">{{ doc.content?.substring(0, 120) }}...</p>
                    <div class="flex flex-wrap items-center gap-3 text-xs text-slate-500">
                        <span class="inline-flex items-center gap-1">
                            <icon name="calendar" class-name="w-3.5 h-3.5" />
                            {{ formatDate(doc.created_at) }}
                        </span>
                        <span v-if="doc.tags?.length" class="inline-flex items-center gap-1 min-w-0">
                            <icon name="tag" class-name="w-3.5 h-3.5 shrink-0" />
                            <span class="truncate">{{ doc.tags.join(', ') }}</span>
                        </span>
                        <span v-else class="text-slate-600">No tags</span>
                    </div>
                </div>
                <div class="text-right shrink-0 flex flex-col items-end gap-1" @click.stop>
                    <div class="text-sm font-semibold text-slate-400">{{ (doc.hot_score || 0).toFixed(2) }}</div>
                    <div class="text-[10px] text-slate-600">hot score</div>
                </div>
            </div>
        </div>
    `
};
